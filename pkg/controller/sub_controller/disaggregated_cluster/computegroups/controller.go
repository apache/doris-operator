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
	"errors"
	"fmt"
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	"github.com/apache/doris-operator/pkg/common/utils/set"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"sync"
)

var _ sc.DisaggregatedSubController = &DisaggregatedComputeGroupsController{}

var (
	disaggregatedComputeGroupsController = "disaggregatedComputeGroupsController"
)

type DisaggregatedComputeGroupsController struct {
	sc.DisaggregatedSubDefaultController
}

func New(mgr ctrl.Manager) *DisaggregatedComputeGroupsController {
	return &DisaggregatedComputeGroupsController{
		sc.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(disaggregatedComputeGroupsController),
			ControllerName: disaggregatedComputeGroupsController,
		},
	}
}

func (dcgs *DisaggregatedComputeGroupsController) Sync(ctx context.Context, obj client.Object) error {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	if len(ddc.Spec.ComputeGroups) == 0 {
		klog.Errorf("disaggregatedComputeGroupsController sync disaggregatedDorisCluster namespace=%s,name=%s have not compute group spec.", ddc.Namespace, ddc.Name)
		dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.ComputeGroupsEmpty), "compute group empty, the cluster will not work normal.")
		return nil
	}

	if !dcgs.feAvailable(ddc) {
		dcgs.K8srecorder.Event(ddc, string(sc.EventNormal), string(sc.WaitFEAvailable), "fe have not ready.")
		return nil
	}

	// validating compute group information.
	if event, res := dcgs.validateComputeGroup(ddc.Spec.ComputeGroups); !res {
		klog.Errorf("disaggregatedComputeGroupsController namespace=%s name=%s validateComputeGroup have not match specifications %s.", ddc.Namespace, ddc.Name, sc.EventString(event))
		dcgs.K8srecorder.Eventf(ddc, string(event.Type), string(event.Reason), event.Message)
		return errors.New("validating compute group failed")
	}

	var errs []error
	cgs := ddc.Spec.ComputeGroups
	for i, _ := range cgs {

		if event, err := dcgs.computeGroupSync(ctx, ddc, &cgs[i]); err != nil {
			if event != nil {
				dcgs.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
			}
			errs = append(errs, err)
			klog.Errorf("disaggregatedComputeGroupsController computeGroups sync failed, compute group Uniqueid %s  sync failed, err=%s", cgs[i].UniqueId, sc.EventString(event))
		}
	}

	if len(errs) != 0 {
		msg := fmt.Sprintf("disaggregatedComputeGroupsController sync namespace: %s ,ddc name: %s, compute group has the following error: ", ddc.Namespace, ddc.Name)
		for _, err := range errs {
			msg += err.Error()
		}
		return errors.New(msg)
	}

	return nil
}

// validate compute group config information.
func (dcgs *DisaggregatedComputeGroupsController) validateComputeGroup(cgs []dv1.ComputeGroup) (*sc.Event, bool) {
	dupl := dcgs.validateDuplicated(cgs)
	if dupl != "" {
		klog.Errorf("disaggregatedComputeGroupsController validateComputeGroup validate Duplicated have duplicate unique identifier %s.", dupl)
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGUniqueIdentifierDuplicate, Message: "unique identifier " + dupl + " duplicate in compute groups."}, false
	}

	if reg, res := dcgs.validateRegex(cgs); !res {
		klog.Errorf("disaggregatedComputeGroupsController validateComputeGroup validateRegex %s have not match regular expression", reg)
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGUniqueIdentifierNotMatchRegex, Message: reg}, false
	}

	return nil, true
}

func (dcgs *DisaggregatedComputeGroupsController) feAvailable(ddc *dv1.DorisDisaggregatedCluster) bool {
	//if fe deploy in k8s, should wait fe available
	//1. wait for fe ok.
	endpoints := corev1.Endpoints{}
	if err := dcgs.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.GetFEServiceName()}, &endpoints); err != nil {
		klog.Infof("disaggregatedComputeGroupsController Sync wait fe service name %s available occur failed %s\n", ddc.GetFEServiceName(), err.Error())
		return false
	}

	for _, sub := range endpoints.Subsets {
		if len(sub.Addresses) > 0 {
			return true
		}
	}
	return false
}

func (dcgs *DisaggregatedComputeGroupsController) computeGroupSync(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) (*sc.Event, error) {
	if cg.Replicas == nil {
		cg.Replicas = resource.GetInt32Pointer(1)
	}
	cvs := dcgs.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.BE_RESOLVEKEY, cg.CommonSpec.ConfigMaps)
	st := dcgs.NewStatefulset(ddc, cg, cvs)
	svc := dcgs.newService(ddc, cg, cvs)
	dcgs.initialCGStatus(ddc, cg)

	dcgs.CheckSecretMountPath(ddc, cg.Secrets)
	dcgs.CheckSecretExist(ctx, ddc, cg.Secrets)

	event, err := dcgs.DefaultReconcileService(ctx, svc)
	if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcile service namespace %s name %s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return event, err
	}
	event, err = dcgs.reconcileStatefulset(ctx, st, ddc, cg)
	if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcile statefulset namespace %s name %s failed, err=%s", st.Namespace, st.Name, err.Error())
	}

	return event, err
}

// reconcileStatefulset return bool means reconcile print error message.
func (dcgs *DisaggregatedComputeGroupsController) reconcileStatefulset(ctx context.Context, st *appv1.StatefulSet, cluster *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) (*sc.Event, error) {
	var est appv1.StatefulSet
	if err := dcgs.K8sclient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, dcgs.K8sclient, st); err != nil {
			klog.Errorf("disaggregatedComputeGroupsController reconcileStatefulset create statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.CGCreateResourceFailed, Message: err.Error()}, err
		}

		return nil, nil
	} else if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcileStatefulset get statefulset failed, namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return nil, err
	}

	err := dcgs.preApplyStatefulSet(ctx, st, &est, cluster, cg)
	if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcileStatefulset preApplyStatefulSet namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGSqlExecFailed, Message: err.Error()}, err
	}
	if skipApplyStatefulset(cluster, cg) {
		return nil, nil
	}

	if err := k8s.ApplyStatefulSet(ctx, dcgs.K8sclient, st, func(st, est *appv1.StatefulSet) bool {
		return resource.StatefulsetDeepEqualWithKey(st, est, dv1.DisaggregatedSpecHashValueAnnotation, false)
	}); err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcileStatefulset apply statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGApplyResourceFailed, Message: err.Error()}, err
	}

	return nil, nil
}

// initial compute group status before sync resources. status changing with sync steps, and generate the last status by classify pods.
func (dcgs *DisaggregatedComputeGroupsController) initialCGStatus(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) {
	cgss := ddc.Status.ComputeGroupStatuses
	//clusterId := ddc.GetCGId(cg)
	uniqueId := cg.UniqueId
	defaultStatus := dv1.ComputeGroupStatus{
		Phase:           dv1.Reconciling,
		UniqueId:        cg.UniqueId,
		StatefulsetName: ddc.GetCGStatefulsetName(cg),
		ServiceName:     ddc.GetCGServiceName(cg),
		//set for status updated.
		Replicas: *cg.Replicas,
	}

	for i := range cgss {
		if cgss[i].UniqueId == uniqueId {
			if cgss[i].Phase != dv1.Ready {
				defaultStatus.Phase = cgss[i].Phase
			}
			defaultStatus.SuspendReplicas = cgss[i].SuspendReplicas
			cgss[i] = defaultStatus
			return
		}
	}

	ddc.Status.ComputeGroupStatuses = append(ddc.Status.ComputeGroupStatuses, defaultStatus)
}

// check compute groups unique identifier duplicated or not. return duplicated key.
func (dcgs *DisaggregatedComputeGroupsController) validateDuplicated(cgs []dv1.ComputeGroup) string {
	dupl := ""
	uniqueIds := set.NewSetString()
	for _, cg := range cgs {
		if uniqueIds.Find(cg.UniqueId) {
			dupl = dupl + cg.UniqueId + ";"
		}
		uniqueIds.Add(cg.UniqueId)
	}

	return dupl
}

// checking the cg name compliant with regular expression or not.
func (dcgs *DisaggregatedComputeGroupsController) validateRegex(cgs []dv1.ComputeGroup) (string, bool) {
	var regStr = ""
	for _, cg := range cgs {
		res, err := regexp.Match(compute_group_name_regex, []byte(cg.UniqueId))
		if !res {
			regStr = regStr + cg.UniqueId + " not match " + compute_group_name_regex
		}
		//for debugging, output the error in log
		if err != nil {
			klog.Errorf("disaggregatedComputeGroupsController validateRegex compute group name %s failed, err=%s", cg.UniqueId, err.Error())
		}
	}
	if regStr != "" {
		return regStr, false
	}

	return "", true
}

// clear not configed cg resources, delete not configed cg status from ddc.status .
func (dcgs *DisaggregatedComputeGroupsController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)

	var eCGs []dv1.ComputeGroupStatus
	for i, cgs := range ddc.Status.ComputeGroupStatuses {
		for _, cg := range ddc.Spec.ComputeGroups {
			if cgs.UniqueId == cg.UniqueId {
				eCGs = append(eCGs, ddc.Status.ComputeGroupStatuses[i])
				break
			}
		}
	}

	//list the svcs and stss owner reference to dorisDisaggregatedCluster.
	cls := dcgs.GetCG2LayerCommonSchedulerLabels(ddc.Name)
	svcs, err := k8s.ListServicesInNamespace(ctx, dcgs.K8sclient, ddc.Namespace, cls)
	if err != nil {
		klog.Errorf("DisaggregatedComputeGroupsController ListServicesInNamespace failed, dorisdisaggregatedcluster name=%s", ddc.Name)
		return false, err
	}
	stss, err := k8s.ListStatefulsetInNamespace(ctx, dcgs.K8sclient, ddc.Namespace, cls)
	if err != nil {
		klog.Errorf("DisaggregatedComputeGroupsController ListStatefulsetInNamespace failed, dorisdisaggregatedcluster name=%s", ddc.Name)
		return false, err
	}

	//clear unused service and statefulset.
	delSvcNames := dcgs.findUnusedSvcs(svcs, ddc)
	delStsNames, delUniqueIds := dcgs.findUnusedStssAndUniqueIds(stss, ddc)

	if err = dcgs.clearCGInDorisMeta(ctx, delUniqueIds, ddc); err != nil {
		return false, err
	}
	if err = dcgs.clearSvcs(ctx, delSvcNames, ddc); err != nil {
		return false, err
	}
	if err = dcgs.clearStatefulsets(ctx, delStsNames, ddc); err != nil {
		return false, err
	}

	//clear unused pvc
	for i := range eCGs {
		err = dcgs.ClearStatefulsetUnusedPVCs(ctx, ddc, eCGs[i])
		if err != nil {
			klog.Errorf("disaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs clear ComputeGroup reduced replicas PVC failed, namespace=%s, ddc name=%s, uniqueId=%s err=%s", ddc.Namespace, ddc.Name, eCGs[i].UniqueId, err.Error())
		}
	}

	for _, uniqueId := range delUniqueIds {
		//new fake computeGroup status for clear all pvcs owner reference to deleted compute group.
		fakeCgs := dv1.ComputeGroupStatus{
			UniqueId: uniqueId,
		}
		err = dcgs.ClearStatefulsetUnusedPVCs(ctx, ddc, fakeCgs)
		if err != nil {
			klog.Errorf("disaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs clear deleted compute group failed, namespace=%s, ddc name=%s, uniqueId=%s err=%s", ddc.Namespace, ddc.Name, uniqueId, err.Error())
		}
	}

	ddc.Status.ComputeGroupStatuses = eCGs
	return true, nil

	//TODO: next pr remove the code
	//sqlClient, err := dcgs.getMasterSqlClient(ctx, dcgs.K8sclient, ddc)
	//if err != nil {
	//	klog.Errorf("computeGroupSync ClearResources dropCGBySQLClient getMasterSqlClient failed: %s", err.Error())
	//	dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGSqlExecFailed), "computeGroupSync dropCGBySQLClient failed: "+err.Error())
	//	return false, err
	//}
	//defer sqlClient.Close()
	//
	//for i := range clearCGs {
	//	cgs := clearCGs[i]
	//	cleared := true
	//	if err := k8s.DeleteStatefulset(ctx, dcgs.K8sclient, ddc.Namespace, cgs.StatefulsetName); err != nil {
	//		cleared = false
	//		klog.Errorf("disaggregatedComputeGroupsController delete statefulset namespace %s name %s failed, err=%s", ddc.Namespace, cgs.StatefulsetName, err.Error())
	//		dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGStatefulsetDeleteFailed), err.Error())
	//	}
	//
	//	if err := k8s.DeleteService(ctx, dcgs.K8sclient, ddc.Namespace, cgs.ServiceName); err != nil {
	//		cleared = false
	//		klog.Errorf("disaggregatedComputeGroupsController delete service namespace %s name %s failed, err=%s", ddc.Namespace, cgs.ServiceName, err.Error())
	//		dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGServiceDeleteFailed), err.Error())
	//	}
	//	if !cleared {
	//		eCGs = append(eCGs, clearCGs[i])
	//		continue
	//	}
	//	// drop compute group
	//	cgName := strings.ReplaceAll(cgs.UniqueId, "_", "-")
	//	cgKeepAmount := int32(0)
	//	err = dcgs.scaledOutBENodesByDrop(sqlClient, cgName, cgKeepAmount)
	//	if err != nil {
	//		klog.Errorf("computeGroupSync ClearResources dropCGBySQLClient failed: %s", err.Error())
	//		dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGSqlExecFailed), "computeGroupSync dropCGBySQLClient failed: "+err.Error())
	//	}
	//
	//}
	//
	//for i := range eCGs {
	//	err := dcgs.ClearStatefulsetUnusedPVCs(ctx, ddc, eCGs[i])
	//	if err != nil {
	//		klog.Errorf("disaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs clear whole ComputeGroup PVC failed, err=%s", err.Error())
	//	}
	//}
	//for i := range clearCGs {
	//	err := dcgs.ClearStatefulsetUnusedPVCs(ctx, ddc, clearCGs[i])
	//	if err != nil {
	//		klog.Errorf("disaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs clear part ComputeGroup PVC failed, err=%s", err.Error())
	//	}
	//}
	//
	//ddc.Status.ComputeGroupStatuses = eCGs
	//
	//return true, nil
}

func (dcgs *DisaggregatedComputeGroupsController) clearStatefulsets(ctx context.Context, stsNames []string, ddc *dv1.DorisDisaggregatedCluster) error {
	for _, name := range stsNames {
		if err := k8s.DeleteStatefulset(ctx, dcgs.K8sclient, ddc.Namespace, name); err != nil {
			klog.Errorf("DisaggregatedComputeGroupsController clear statefulset failed, namespace=%s, name =%s, err=%s", ddc.Namespace, name, err.Error())
			return err
		}
	}
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) clearSvcs(ctx context.Context, svcNames []string, ddc *dv1.DorisDisaggregatedCluster) error {
	for _, name := range svcNames {
		if err := k8s.DeleteService(ctx, dcgs.K8sclient, ddc.Namespace, name); err != nil {
			klog.Errorf("DisaggregatedComputeGroupsController clear service failed, namespace=%s, name =%s, err=%s", ddc.Namespace, name, err.Error())
			return err
		}
	}
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) clearCGInDorisMeta(ctx context.Context, cgNames []string, ddc *dv1.DorisDisaggregatedCluster) error {
    if len(cgNames) == 0 {
        return nil
    }

	sqlClient, err := dcgs.getMasterSqlClient(ctx, dcgs.K8sclient, ddc)
	if err != nil {
		klog.Errorf("DisaggregatedComputeGroupsController clearCGInDorisMeta dropCGBySQLClient getMasterSqlClient failed: %s", err.Error())
		dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGSqlExecFailed), "computeGroupSync dropCGBySQLClient failed: "+err.Error())
		return err
	}
	defer sqlClient.Close()

	for _, name := range cgNames {
		//clear cg, the keepAmount = 0
		//confirm used the right cgName, as the cgName get from the uniqueid that '-' replaced by '_'.
		cgName := strings.ReplaceAll(name, "-", "_")
		err = dcgs.scaledOutBENodesByDrop(sqlClient, cgName, 0)
		if err != nil {
			klog.Errorf("DisaggregatedComputeGroupsController clearCGInDorisMeta dropCGBySQLClient failed: %s", err.Error())
			dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGSqlExecFailed), "computeGroupSync dropCGBySQLClient failed: "+err.Error())
			return err
		}
	}

	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) findUnusedSvcs(svcs []corev1.Service, ddc *dv1.DorisDisaggregatedCluster) []string {
	var unusedSvcNames []string
	for i, _ := range svcs {
		own := ownerReference2ddc(&svcs[i], ddc)
		if !own {
			//not owner reference to ddc, should skip the service.
			continue
		}

		svcUniqueId := getUniqueIdFromClientObject(&svcs[i])
		exist := false
		for j := 0; j < len(ddc.Spec.ComputeGroups); j++ {
			if ddc.Spec.ComputeGroups[j].UniqueId == svcUniqueId {
				exist = true
				break
			}
		}

		if !exist {
			unusedSvcNames = append(unusedSvcNames, svcs[i].Name)
		}
	}

	return unusedSvcNames
}

func (dcgs *DisaggregatedComputeGroupsController) findUnusedStssAndUniqueIds(stss []appv1.StatefulSet, ddc *dv1.DorisDisaggregatedCluster) ([]string /*sts*/, []string /*cgNames*/) {
	var unusedStsNames []string
	var unusedUniqueIds []string
	for i, _ := range stss {
		own := ownerReference2ddc(&stss[i], ddc)
		if !own {
			//not owner reference tto ddc should skip the statefulset.
			continue
		}

		stsUniqueId := getUniqueIdFromClientObject(&stss[i])
		exist := false
		for j := 0; j < len(ddc.Spec.ComputeGroups); j++ {
			if ddc.Spec.ComputeGroups[j].UniqueId == stsUniqueId {
				exist = true
				break
			}
		}
		if !exist {
			unusedStsNames = append(unusedStsNames, stss[i].Name)
			unusedUniqueIds = append(unusedUniqueIds, stsUniqueId)
		}
	}

	return unusedStsNames, unusedUniqueIds
}

// ClearStatefulsetUnusedPVCs
// 1.delete unused pvc skip cluster is Suspend
// 2.delete unused pvc for statefulset
// 3.delete pvc if not used by any statefulset
func (dcgs *DisaggregatedComputeGroupsController) ClearStatefulsetUnusedPVCs(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster, cgs dv1.ComputeGroupStatus) error {
	var cg *dv1.ComputeGroup
	for i := range ddc.Spec.ComputeGroups {
		/*	uniqueId := ddc.GetCGId(&ddc.Spec.ComputeGroups[i])
			if clusterId == cgs.ClusterId {
				cg = &ddc.Spec.ComputeGroups[i]
			}*/
		if ddc.Spec.ComputeGroups[i].UniqueId == cgs.UniqueId {
			cg = &ddc.Spec.ComputeGroups[i]
		}
	}

	currentPVCs := corev1.PersistentVolumeClaimList{}
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)

	pvcLabels := dcgs.newCGPodsSelector(ddc.Name, cgs.UniqueId)

	if err := dcgs.K8sclient.List(ctx, &currentPVCs, client.InNamespace(ddc.Namespace), client.MatchingLabels(pvcLabels)); err != nil {
		dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), sc.PVCListFailed, fmt.Sprintf("DisaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs list pvc failed:%s!", err.Error()))
		return err
	}

	for i := range currentPVCs.Items {
		pvcMap[currentPVCs.Items[i].Name] = &currentPVCs.Items[i]
	}

	if cg != nil {
        //we should use statefulset replicas for avoiding the phase=scaleDown, when phase `scaleDown` cg' replicas is less than statefuslet.
		replicas := 0
		stsName := ddc.GetCGStatefulsetName(cg)
        sts, err := k8s.GetStatefulSet(ctx, dcgs.K8sclient, ddc.Namespace, stsName)
		if err != nil {
			klog.Errorf("DisaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs get statefulset namespace=%s, name=%s, failed, err=%s", ddc.Namespace, stsName, err.Error())
			//waiting next reconciling.
			return nil
		}
		replicas = int(*sts.Spec.Replicas)

		cvs := dcgs.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.BE_RESOLVEKEY, cg.CommonSpec.ConfigMaps)
		paths, _ := dcgs.getCacheMaxSizeAndPaths(cvs)

		if cgs.Phase == dv1.Suspended || cgs.Phase == dv1.SuspendFailed || replicas == 0 {
			return nil
		}

		var reservePVCNameList []string

		for i := 0; i < replicas; i++ {
			iStr := strconv.Itoa(i)
			reservePVCNameList = append(reservePVCNameList, resource.BuildPVCName(stsName, iStr, LogStoreName))
			for j := 0; j < len(paths); j++ {
				jStr := strconv.Itoa(j)
				reservePVCNameList = append(reservePVCNameList, resource.BuildPVCName(stsName, iStr, StorageStorePreName+jStr))
			}
		}

		for _, pvcName := range reservePVCNameList {
			if _, ok := pvcMap[pvcName]; ok {
				delete(pvcMap, pvcName)
			}
		}
	}

	var mergeError error
	for _, claim := range pvcMap {
		if err := k8s.DeletePVC(ctx, dcgs.K8sclient, claim.Namespace, claim.Name, pvcLabels); err != nil {
			dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), sc.PVCDeleteFailed, err.Error())
			klog.Errorf("ClearStatefulsetUnusedPVCs deletePVCs failed: namespace %s, name %s delete pvc %s, err: %s .", claim.Namespace, claim.Name, claim.Name, err.Error())
			mergeError = utils.MergeError(mergeError, err)
		}
	}
	return mergeError
}

func (dcgs *DisaggregatedComputeGroupsController) GetControllerName() string {
	return dcgs.ControllerName
}

func (dcgs *DisaggregatedComputeGroupsController) UpdateComponentStatus(obj client.Object) error {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	cgss := ddc.Status.ComputeGroupStatuses
	if len(cgss) == 0 {
		klog.Errorf("disaggregatedComputeGroupsController updateComponentStatus compute group status is empty!")
		return nil
	}

	errChan := make(chan error, len(cgss))
	wg := sync.WaitGroup{}
	wg.Add(len(cgss))
	for i, _ := range cgss {
		go func(idx int) {
			defer wg.Done()
			errChan <- dcgs.updateCGStatus(ddc, &cgss[idx])
		}(i)
	}

	wg.Wait()
	close(errChan)
	errMs := ""
	for err := range errChan {
		if err != nil {
			errMs += err.Error()
		}
	}

	var fullAvailableCount int32
	var availableCount int32
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cgs.Phase == dv1.Ready {
			fullAvailableCount++
		}
		if cgs.AvailableReplicas > 0 {
			availableCount++
		}
	}
	ddc.Status.ClusterHealth.CGCount = int32(len(ddc.Status.ComputeGroupStatuses))
	ddc.Status.ClusterHealth.CGFullAvailableCount = fullAvailableCount
	ddc.Status.ClusterHealth.CGAvailableCount = availableCount
	if errMs == "" {
		return nil
	}
	return errors.New(errMs)
}

func (dcgs *DisaggregatedComputeGroupsController) updateCGStatus(ddc *dv1.DorisDisaggregatedCluster, cgs *dv1.ComputeGroupStatus) error {
	selector := dcgs.newCGPodsSelector(ddc.Name, cgs.UniqueId)
	var podList corev1.PodList
	if err := dcgs.K8sclient.List(context.Background(), &podList, client.InNamespace(ddc.Namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}

	var availableReplicas int32
	var creatingReplicas int32
	var failedReplicas int32
	//get all pod status that controlled by st.
	for _, pod := range podList.Items {
		if ready := k8s.PodIsReady(&pod.Status); ready {
			availableReplicas++
		} else if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			creatingReplicas++
		} else {
			failedReplicas++
		}
	}

	cgs.AvailableReplicas = availableReplicas
	if availableReplicas == cgs.Replicas {
		cgs.Phase = dv1.Ready
	}
	return nil
}
