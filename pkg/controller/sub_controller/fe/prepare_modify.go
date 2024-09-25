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
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	if *(cluster.Spec.FeSpec.Replicas) < *(cluster.Spec.FeSpec.ElectionNumber) {
		fc.K8srecorder.Event(cluster, string(sc.EventWarning), string(sc.FESpecSetError), "The number of fe ElectionNumber is large than Replicas, Replicas has been corrected to the correct minimum value")
		klog.Errorf("prepareStatefulsetApply namespace=%s,name=%s ,The number of fe ElectionNumber(%d) is large than Replicas(%d)", cluster.Namespace, cluster.Name, *(cluster.Spec.FeSpec.ElectionNumber), *(cluster.Spec.FeSpec.Replicas))
		ele := *(cluster.Spec.FeSpec.ElectionNumber)
		cluster.Spec.FeSpec.Replicas = &ele
	}

	scaleNumber := *(cluster.Spec.FeSpec.Replicas) - *(oldSt.Spec.Replicas)
	// fe scale
	if scaleNumber < 0 {
		if err := fc.dropObserverBySqlClient(ctx, fc.K8sclient, cluster); err != nil {
			klog.Errorf("ScaleDownObserver failed, err:%s ", err.Error())
			return err
		}
		return nil
	}

	//TODO check upgrade ,restart

	return nil
}

// dropObserverBySqlClient handles doris'SQL(drop frontend) through the MySQL client when dealing with scale in observer
// targetDCR is new dcr
// scaleNumber is the number of Observer needing scale down
func (fc *Controller) dropObserverBySqlClient(ctx context.Context, k8sclient client.Client, targetDCR *v1.DorisCluster) error {
	// get adminuserName and pwd
	secret, _ := k8s.GetSecret(ctx, k8sclient, targetDCR.Namespace, targetDCR.Spec.AuthSecret)
	adminUserName, password := v1.GetClusterSecret(targetDCR, secret)
	// get host and port
	serviceName := v1.GenerateExternalServiceName(targetDCR, v1.Component_FE)
	// When the operator and dcr are deployed in different namespace, it will be inaccessible, so need to add the dcr svc namespace
	host := serviceName + "." + targetDCR.Namespace
	maps, _ := k8s.GetConfig(ctx, k8sclient, &targetDCR.Spec.FeSpec.ConfigMapInfo, targetDCR.Namespace, v1.Component_FE)
	queryPort := resource.GetPort(maps, resource.QUERY_PORT)

	// connect to doris sql to get master node
	// It may not be the master, or even the node that needs to be deleted, causing the deletion SQL to fail.
	dbConf := mysql.DBConfig{
		User:     adminUserName,
		Password: password,
		Host:     host,
		Port:     strconv.FormatInt(int64(queryPort), 10),
		Database: "mysql",
	}
	masterDBClient, err := mysql.NewDorisMasterSqlDB(dbConf)
	if err != nil {
		klog.Errorf("NewDorisMasterSqlDB failed, get fe node connection err:%s", err.Error())
		return err
	}
	defer masterDBClient.Close()

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
		frontendMap, err = mysql.BuildSeqNumberToFrontendMap(allObserves, nil, podTemplateName)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	} else { // use ip
		podMap := make(map[string]string) // key is pod ip, value is pod name
		pods, err := k8s.GetPods(ctx, k8sclient, targetDCR.Namespace, v1.GetPodLabels(targetDCR, v1.Component_FE))
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, GetPods err:%s", err)
			return nil
		}
		for _, item := range pods.Items {
			if strings.HasPrefix(item.GetName(), podTemplateName) {
				podMap[item.Status.PodIP] = item.GetName()
			}
		}
		frontendMap, err = mysql.BuildSeqNumberToFrontendMap(allObserves, podMap, podTemplateName)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	}
	observes := mysql.FindNeedDeletedFrontends(frontendMap, realScaleNumber)
	// drop node and return
	return masterDBClient.DropObserver(observes)

}
