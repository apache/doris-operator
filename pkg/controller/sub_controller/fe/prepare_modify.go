// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package fe

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/mysql"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strconv"
	"strings"
)

// prepareStatefulsetApply means Pre-operation and status control on the client side
func (fc *Controller) prepareStatefulsetApply(ctx context.Context, cluster *v1.DorisCluster) error {
	var oldSt appv1.StatefulSet
	err := fc.K8sclient.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: v1.GenerateComponentStatefulSetName(cluster, v1.Component_FE)}, &oldSt)
	if err != nil {
		klog.Infof("fe controller controlClusterPhaseAndPreOperation get fe StatefulSet failed, err: %s", err.Error())
		return nil
	}
	if cluster.Spec.FeSpec.Replicas == nil {
		cluster.Spec.FeSpec.Replicas = resource.GetInt32Pointer(0)
	}
	scaleNumber := *(cluster.Spec.FeSpec.Replicas) - *(oldSt.Spec.Replicas)
	// fe scale
	/*	if scaleNumber != 0 { // set fe Phase as SCALING
		cluster.Status.FEStatus.ComponentCondition.Phase = v1.Scaling
		// In Reconcile, it is possible that the status cannot be updated in time,
		// resulting in an error in the status judgment based on the last status,
		// so the status will be forced to modify here
			if err := k8s.SetDorisClusterPhase(ctx, fc.K8sclient, cluster.Name, cluster.Namespace, v1.Scaling, v1.Component_FE); err != nil {
			klog.Errorf("SetDorisClusterPhase 'SCALING' failed err:%s ", err.Error())
			return err
		}
	}*/

	if scaleNumber < 0 {
		if err := fc.dropObserverFromSqlClient(ctx, fc.K8sclient, cluster); err != nil {
			klog.Errorf("ScaleDownObserver failed, err:%s ", err.Error())
			return err
		}
		return nil
	}

	//TODO check upgrade ,restart

	return nil
}

// dropObserverFromSqlClient handles doris'SQL(drop frontend) through the MySQL client when dealing with scale in observer
// targetDCR is new dcr
// scaleNumber is the number of Observer needing scale down
func (fc *Controller) dropObserverFromSqlClient(ctx context.Context, k8sclient client.Client, targetDCR *v1.DorisCluster) error {
	// get adminuserName and pwd
	secret, _ := k8s.GetSecret(ctx, k8sclient, targetDCR.Namespace, targetDCR.Spec.AuthSecret)
	adminUserName, password := v1.GetClusterSecret(targetDCR, secret)
	// get host and port
	serviceName := v1.GenerateExternalServiceName(targetDCR, v1.Component_FE)
	maps, _ := k8s.GetConfig(ctx, k8sclient, &targetDCR.Spec.FeSpec.ConfigMapInfo, targetDCR.Namespace, v1.Component_FE)
	queryPort := resource.GetPort(maps, resource.QUERY_PORT)

	// connect to doris sql to get master node
	// It may not be the master, or even the node that needs to be deleted, causing the deletion SQL to fail.
	dbConf := mysql.DBConfig{
		User:     adminUserName,
		Password: password,
		Host:     serviceName,
		Port:     strconv.FormatInt(int64(queryPort), 10),
		Database: "mysql",
	}
	loadBalanceDBClient, err := mysql.NewDorisSqlDB(dbConf)
	if err != nil {
		klog.Errorf("DropObserverFromSqlClient failed, get fe node connection err:%s", err.Error())
		return err
	}
	defer loadBalanceDBClient.Close()
	master, _, err := loadBalanceDBClient.GetFollowers()
	if err != nil {
		klog.Errorf("DropObserverFromSqlClient GetFollowers master failed, err:%s", err.Error())
		return err
	}
	var masterDBClient *mysql.DB
	if master.CurrentConnected == "Yes" {
		masterDBClient = loadBalanceDBClient
	} else {
		// Get the connection to the master
		masterDBClient, err = mysql.NewDorisSqlDB(mysql.DBConfig{
			User:     adminUserName,
			Password: password,
			Host:     master.Host,
			Port:     strconv.FormatInt(int64(queryPort), 10),
			Database: "mysql",
		})
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, get fe master connection  err:%s", err.Error())
			return err
		}
		defer masterDBClient.Close()
	}

	// get all Observes
	allObserves, err := masterDBClient.GetObservers()
	if err != nil {
		klog.Errorf("DropObserverFromSqlClient failed, GetObservers err:%s", err.Error())
		return err
	}

	// make sure real scaleNumber, this may involve retrying tasks and scaling down followers.
	electionNumber := Default_Election_Number
	if targetDCR.Spec.FeSpec.ElectionNumber != nil {
		electionNumber = *(targetDCR.Spec.FeSpec.ElectionNumber)
	}
	// means: realScaleNumber = allobservers - (replicas - election)
	realScaleNumber := int32(len(allObserves)) - *(targetDCR.Spec.FeSpec.Replicas) + electionNumber
	if realScaleNumber <= 0 {
		klog.Errorf("DropObserverFromSqlClient failed, Observers number(%d) is not larger than scale number(%d) ", len(allObserves), *(targetDCR.Spec.FeSpec.Replicas)-electionNumber)
		return nil
	}

	// get scale Observes
	var frontendMap map[int]*mysql.Frontend // frontendMap key is fe pod index ,value is frontend
	podTemplateName := resource.GeneratePodTemplateName(targetDCR, v1.Component_FE)

	if resource.GetStartMode(maps) == resource.START_MODEL_FQDN { // use host
		frontendMap, err = buildSeqNumberToFrontend(allObserves, nil, podTemplateName)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	} else { // use ip
		podMap := make(map[string]string) // key is pod ip, value is pod name
		pods, err := k8s.GetPods(ctx, k8sclient, *targetDCR, v1.Component_FE)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, GetPods err:%s", err)
			return nil
		}
		for _, item := range pods.Items {
			if strings.HasPrefix(item.GetName(), podTemplateName) {
				podMap[item.Status.PodIP] = item.GetName()
			}
		}
		frontendMap, err = buildSeqNumberToFrontend(allObserves, podMap, podTemplateName)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	}
	observes := getFirstFewFrontendsAfterDescendOrder(frontendMap, realScaleNumber)
	// drop node and return
	return masterDBClient.DropObserver(observes)

}

// buildSeqNumberToFrontend
// input ipMap key is podIP,value is fe.podName(from 'kubectl get pods -owide')
// return frontendMap key is fe pod index ,value is frontend
func buildSeqNumberToFrontend(frontends []*mysql.Frontend, ipMap map[string]string, podTemplateName string) (map[int]*mysql.Frontend, error) {
	frontendMap := make(map[int]*mysql.Frontend)
	for _, fe := range frontends {
		var podSignName string
		if strings.HasPrefix(fe.Host, podTemplateName) {
			// use fqdn, not need ipMap
			// podSignName like: doriscluster-sample-fe-0.doriscluster-sample-fe-internal.doris.svc.cluster.local
			podSignName = fe.Host
		} else {
			// use ip
			// podSignName like: doriscluster-sample-fe-0
			podSignName = ipMap[fe.Host]
		}
		split := strings.Split(strings.Split(strings.Split(podSignName, podTemplateName)[1], ".")[0], "-")
		num, err := strconv.Atoi(split[len(split)-1])
		if err != nil {
			klog.Errorf("buildSeqNumberToFrontend can not split pod name,pod name: %s,err:%s", podSignName, err.Error())
			return nil, err
		}
		frontendMap[num] = fe
	}
	return frontendMap, nil
}

// GetFirstFewFrontendsAfterDescendOrder means descending sort fe by index and return top scaleNumber
func getFirstFewFrontendsAfterDescendOrder(frontendMap map[int]*mysql.Frontend, scaleNumber int32) []*mysql.Frontend {
	var topFrontends []*mysql.Frontend
	if int(scaleNumber) <= len(frontendMap) {
		keys := make([]int, 0, len(frontendMap))
		for k := range frontendMap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] > keys[j]
		})

		for i := 0; i < int(scaleNumber); i++ {
			topFrontends = append(topFrontends, frontendMap[keys[i]])
		}
	} else {
		klog.Errorf("getFirstFewFrontendsAfterDescendOrder frontendMap size(%d) not larger than scaleNumber(%d)", len(frontendMap), scaleNumber)
	}
	return topFrontends
}
