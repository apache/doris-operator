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

package computegroups

import (
	"context"
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

func (dcgs *DisaggregatedComputeGroupsController) preApplyStatefulSet(ctx context.Context, st, est *appv1.StatefulSet, cluster *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) error {
	var cgStatus *dv1.ComputeGroupStatus
	uniqueId := cg.UniqueId
	for i := range cluster.Status.ComputeGroupStatuses {
		if cluster.Status.ComputeGroupStatuses[i].UniqueId == uniqueId {
			cgStatus = &cluster.Status.ComputeGroupStatuses[i]
			break
		}
	}
	optType := getOperationType(st, est, cgStatus.Phase)

	switch optType {
	case "scaleDown":
		err := dcgs.scaleOut(ctx, cgStatus, cluster, cg)
		if err != nil {
			return err
		}
	default:
		// default do nothing, not need pre ApplyStatefulSet
	}

	return nil

}

func (dcgs *DisaggregatedComputeGroupsController) scaleOut(ctx context.Context, cgStatus *dv1.ComputeGroupStatus, cluster *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) error {
	sqlClient, err := dcgs.getMasterSqlClient(ctx, dcgs.K8sclient, cluster)
	if err != nil {
		klog.Errorf("ScaleOut getMasterSqlClient failed, get fe master node connection err:%s", err.Error())
		return err
	}
	defer sqlClient.Close()

	cgKeepAmount := *cg.Replicas
	cgName := cluster.GetCGName(cg)

	if cluster.Spec.EnableDecommission {
		if err := dcgs.scaledOutBENodesByDecommission(cluster, cgStatus, sqlClient, cgName, cgKeepAmount); err != nil {
			return err
		}
	} else { // not decommission , drop node
		if err := dcgs.scaledOutBENodesByDrop(sqlClient, cgName, cgKeepAmount); err != nil {
			cgStatus.Phase = dv1.ScaleDownFailed
			klog.Errorf("ScaleOut scaledOutBENodesByDrop ddcName:%s, namespace:%s, computeGroupName:%s, drop nodes failed:%s ", cluster.Name, cluster.Namespace, cgName, err.Error())
			return err
		}
		cgStatus.Phase = dv1.Scaling
	}
	// return nil will apply sts
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) scaledOutBENodesByDecommission(cluster *dv1.DorisDisaggregatedCluster, cgStatus *dv1.ComputeGroupStatus, sqlClient *mysql.DB, cgName string, cgKeepAmount int32) error {
	decommissionPhase, err := dcgs.decommissionProgressCheck(sqlClient, cgName, cgKeepAmount)
	if err != nil {
		return err
	}
	switch decommissionPhase {
	case resource.DecommissionAcceptable:
		err = dcgs.decommissionBENodes(sqlClient, cgName, cgKeepAmount)
		if err != nil {
			cgStatus.Phase = dv1.ScaleDownFailed
			klog.Errorf("scaledOutBENodesByDecommission ddcName:%s, namespace:%s, computeGroupName:%s , Decommission failed, err:%s ", cluster.Name, cluster.Namespace, cgName, err.Error())
			return err
		}
		cgStatus.Phase = dv1.Decommissioning
		return nil
	case resource.Decommissioning, resource.DecommissionPhaseUnknown:
		cgStatus.Phase = dv1.Decommissioning
		klog.Infof("scaledOutBENodesByDecommission ddcName:%s, namespace:%s, computeGroupName:%s, Decommission in progress", cluster.Name, cluster.Namespace, cgName)
		return nil
	case resource.Decommissioned:
		dcgs.scaledOutBENodesByDrop(sqlClient, cgName, cgKeepAmount)
	}
	cgStatus.Phase = dv1.Scaling
	return nil
}

func getOperationType(st, est *appv1.StatefulSet, phase dv1.Phase) string {
	//Should not check 'phase == dv1.Ready', because the default value of the state initialization is Reconciling in the new Reconcile
	// *st.Spec.Replicas < *est.Spec.Replicas represents need initial scaleDown, it belongs to the start phase.
	if *(st.Spec.Replicas) < *(est.Spec.Replicas) || phase == dv1.Decommissioning || phase == dv1.ScaleDownFailed {
		return "scaleDown"
	}
	return ""
}

func (dcgs *DisaggregatedComputeGroupsController) scaledOutBENodesByDrop(
	masterDBClient *mysql.DB,
	cgName string,
	cgKeepAmount int32) error {

	dropNodes, err := getScaledOutBENode(masterDBClient, cgName, cgKeepAmount)
	if err != nil {
		klog.Errorf("scaledOutBENodesByDrop getScaledOutBENode cgName %s failed, err:%s ", cgName, err.Error())
		return err
	}

	if len(dropNodes) == 0 {
		return nil
	}
	err = masterDBClient.DropBE(dropNodes)
	if err != nil {
		klog.Errorf("scaledOutBENodesByDrop cgName %s DropBENodes failed, err:%s ", cgName, err.Error())
		return err
	}
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) decommissionBENodes(
	masterDBClient *mysql.DB,
	cgName string,
	cgKeepAmount int32) error {

	dropNodes, err := getScaledOutBENode(masterDBClient, cgName, cgKeepAmount)
	if err != nil {
		klog.Errorf("decommissionBENodes getScaledOutBENode cgName %s failed, err:%s ", cgName, err.Error())
		return err
	}

	if len(dropNodes) == 0 {
		return nil
	}
	err = masterDBClient.DecommissionBE(dropNodes)
	if err != nil {
		klog.Errorf("decommissionBENodes cgName %s DropBENodes failed, err:%s ", cgName, err.Error())
		return err
	}
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) getMasterSqlClient(ctx context.Context, k8sclient client.Client, cluster *dv1.DorisDisaggregatedCluster) (*mysql.DB, error) {
	// get user and password
	secret, _ := k8s.GetSecret(ctx, k8sclient, cluster.Namespace, cluster.Spec.AuthSecret)
	adminUserName, password := resource.GetDorisLoginInformation(secret)

	// get host and port
	// When the operator and dcr are deployed in different namespace, it will be inaccessible, so need to add the dcr svc namespace
	host := cluster.GetFEVIPAddresss()
	confMap := dcgs.GetConfigValuesFromConfigMaps(cluster.Namespace, resource.FE_RESOLVEKEY, cluster.Spec.FeSpec.ConfigMaps)
	queryPort := resource.GetPort(confMap, resource.QUERY_PORT)

	// connect to doris sql to get master node
	// It may not be the master, or even the node that needs to be deleted, causing the deletion SQL to fail.
	dbConf := mysql.DBConfig{
		User:     adminUserName,
		Password: password,
		Host:     host,
		Port:     strconv.FormatInt(int64(queryPort), 10),
		Database: "mysql",
	}
	// Connect to the master and run the SQL statement of system admin, because it is not excluded that the user can shrink be and fe at the same time
	masterDBClient, err := mysql.NewDorisMasterSqlDB(dbConf)
	if err != nil {
		klog.Errorf("getMasterSqlClient NewDorisMasterSqlDB failed for ddc %s namespace %s, get fe node connection err:%s", cluster.Namespace, cluster.Name, err.Error())
		return nil, err
	}
	return masterDBClient, nil
}

// isDecommissionProgressFinished check decommission status
func (dcgs *DisaggregatedComputeGroupsController) decommissionProgressCheck(masterDBClient *mysql.DB, cgName string, cgKeepAmount int32) (resource.DecommissionPhase, error) {
	allBackends, err := masterDBClient.GetBackendsByCGName(cgName)
	if err != nil {
		klog.Errorf("decommissionProgressCheck failed, cgName %s ShowBackends err:%s", cgName, err.Error())
		return resource.DecommissionPhaseUnknown, err
	}
	dts := resource.ConstructDecommissionTaskStatus(allBackends, cgKeepAmount)
	return dts.GetDecommissionPhase(), nil
}

func getScaledOutBENode(
	masterDBClient *mysql.DB,
	cgName string,
	cgKeepAmount int32) ([]*mysql.Backend, error) {

	allBackends, err := masterDBClient.GetBackendsByCGName(cgName)
	if err != nil {
		klog.Errorf("scaledOutBEPreprocessing failed,  cgName %s ShowBackends err:%s", cgName, err.Error())
		return nil, err
	}

	var dropNodes []*mysql.Backend
	for i := range allBackends {
		node := allBackends[i]
		split := strings.Split(node.Host, ".")
		splitCGIDArr := strings.Split(split[0], "-")
		podNum, err := strconv.Atoi(splitCGIDArr[len(splitCGIDArr)-1])
		if err != nil {
			klog.Errorf("scaledOutBEPreprocessing  cgName %s splitCGIDArr can not split host : %s,err:%s", cgName, node.Host, err.Error())
			return nil, err
		}
		if podNum >= int(cgKeepAmount) {
			dropNodes = append(dropNodes, node)
		}
	}
	return dropNodes, nil
}

func skipApplyStatefulset(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) bool {
	var cgStatus *dv1.ComputeGroupStatus
	uniqueId := cg.UniqueId
	for i := range ddc.Status.ComputeGroupStatuses {
		if ddc.Status.ComputeGroupStatuses[i].UniqueId == uniqueId {
			cgStatus = &ddc.Status.ComputeGroupStatuses[i]
			break
		}
	}

	if cgStatus.Phase == dv1.Decommissioning {
		return true
	}
	return false
}
