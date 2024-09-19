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
	"encoding/json"
	"errors"
	"fmt"
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils"
	"github.com/apache/doris-operator/pkg/common/utils/disaggregated_ms/ms_http"
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

	cgs := ddc.Spec.ComputeGroups
	for i, _ := range cgs {
		//// if be unique identifier updated, operator should revert it.
		//dcgs.revertNotAllowedUpdateFields(ddc, &cgs[i])
		if event, err := dcgs.computeGroupSync(ctx, ddc, &cgs[i]); err != nil {
			if event != nil {
				dcgs.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
			}
			klog.Errorf("disaggregatedComputeGroupsController computeGroups sync failed, compute group Uniqueid %s  sync failed, err=%s", cgs[i].UniqueId, sc.EventString(event))
		}
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

	var cgStatus *dv1.ComputeGroupStatus

	clusterId := cluster.GetCGId(cg)
	for i := range cluster.Status.ComputeGroupStatuses {
		if cluster.Status.ComputeGroupStatuses[i].ClusterId == clusterId {
			cgStatus = &cluster.Status.ComputeGroupStatuses[i]
			break
		}
	}
	scaleType := getScaleType(st, &est, cgStatus.Phase)

	if scaleType == "resume" {
		if cgStatus.SuspendReplicas != *(st.Spec.Replicas) {
			errMessage := fmt.Sprintf("ResumeComputeGroup configuration is abnormal. The replicas of resumes(%d) is not equal to the replicas of suspends(%d).", *st.Spec.Replicas, cgStatus.SuspendReplicas)
			return &sc.Event{
				Type:    sc.EventNormal,
				Reason:  sc.CGResumeReplicasInconsistency,
				Message: errMessage,
			}, errors.New(errMessage)
		}
	}

	if err := k8s.ApplyStatefulSet(ctx, dcgs.K8sclient, st, func(st, est *appv1.StatefulSet) bool {
		return resource.StatefulsetDeepEqualWithOmitKey(st, est, dv1.DisaggregatedSpecHashValueAnnotation, true, false)
	}); err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcileStatefulset apply statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGApplyResourceFailed, Message: err.Error()}, err
	}

	switch scaleType {
	case "resume":
		err := ms_http.ResumeComputeGroup(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, cluster.Status.InstanceId, clusterId)
		cgStatus.SuspendReplicas = 0
		if err != nil {
			cgStatus.Phase = dv1.ResumeFailed
			klog.Errorf("computeGroupSync ResumeComputeGroup response failed , err: %s", err.Error())
			return &sc.Event{
				Type:    sc.EventNormal,
				Reason:  sc.CGResumeStatusRequestFailed,
				Message: "ResumeComputeGroup request of disaggregated BE failed: " + err.Error(),
			}, err
		}
		cgStatus.Phase = dv1.Scaling
	case "scaleDown":
		if err := dcgs.dropCGFromHttpClient(cluster, cg); err != nil {
			cgStatus.Phase = dv1.ScaleDownFailed
			klog.Errorf("ScaleDownBE failed, err:%s ", err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.CGHTTPFailed, Message: err.Error()},
				err
		}
		cgStatus.Phase = dv1.Scaling
	case "suspend":
		err := ms_http.SuspendComputeGroup(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, cluster.Status.InstanceId, clusterId)
		if err != nil {
			cgStatus.Phase = dv1.SuspendFailed
			klog.Errorf("computeGroupSync SuspendComputeGroup response failed , err: %s", err.Error())
			return &sc.Event{
				Type:    sc.EventNormal,
				Reason:  sc.CGSuspendStatusRequestFailed,
				Message: "SuspendComputeGroup request of disaggregated BE failed: " + err.Error(),
			}, err
		}
		cgStatus.SuspendReplicas = *est.Spec.Replicas
		cgStatus.Phase = dv1.Suspended
	}

	return nil, nil
}

func getScaleType(st, est *appv1.StatefulSet, phase dv1.Phase) string {
	if (*(st.Spec.Replicas) > *(est.Spec.Replicas) && *(est.Spec.Replicas) == 0) || phase == dv1.ResumeFailed {
		return "resume"
	}

	if (*(st.Spec.Replicas) < *(est.Spec.Replicas) && *(st.Spec.Replicas) > 0) || phase == dv1.ScaleDownFailed {
		return "scaleDown"
	}

	if (*(st.Spec.Replicas) < *(est.Spec.Replicas) && *(st.Spec.Replicas) == 0) || phase == dv1.SuspendFailed {
		return "suspend"
	}
	return ""
}

// initial compute group status before sync resources. status changing with sync steps, and generate the last status by classify pods.
func (dcgs *DisaggregatedComputeGroupsController) initialCGStatus(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) {
	cgss := ddc.Status.ComputeGroupStatuses
	clusterId := ddc.GetCGId(cg)
	defaultStatus := dv1.ComputeGroupStatus{
		Phase:           dv1.Reconciling,
		ClusterId:       clusterId,
		UniqueId:        cg.UniqueId,
		StatefulsetName: ddc.GetCGStatefulsetName(cg),
		ServiceName:     ddc.GetCGServiceName(cg),
		//set for status updated.
		Replicas: *cg.Replicas,
	}

	for i := range cgss {
		if cgss[i].ClusterId == clusterId {
			if cgss[i].Phase == dv1.ScaleDownFailed || cgss[i].Phase == dv1.Suspended ||
				cgss[i].Phase == dv1.SuspendFailed || cgss[i].Phase == dv1.ResumeFailed ||
				cgss[i].Phase == dv1.Scaling {
				defaultStatus.Phase = cgss[i].Phase
			}
			defaultStatus.SuspendReplicas = cgss[i].SuspendReplicas
			cgss[i] = defaultStatus
			return
		}
	}

	ddc.Status.ComputeGroupStatuses = append(ddc.Status.ComputeGroupStatuses, defaultStatus)
}

// clusterId and cloudUniqueId is not allowed update, when be mistakenly modified on these fields, operator should revert it by status fields.
/*func (dccs *DisaggregatedComputeGroupsController) revertNotAllowedUpdateFields(ddc *dv1.DorisDisaggregatedCluster, cc *dv1.ComputeGroup) {
	for _, ccs := range ddc.Status.ComputeGroupStatuses {
		if (ccs.ComputeClusterName != "" && ccs.ComputeClusterName == cc.Name) || (ccs.ClusterId != "" && ccs.ClusterId == cc.ClusterId) {
			if ccs.ClusterId != "" && ccs.ClusterId != cc.ClusterId {
				cc.ClusterId = ccs.ClusterId
			}
		}
	}
}*/

// check compute groups unique identifier duplicated or not. return duplicated key.
func (dcgs *DisaggregatedComputeGroupsController) validateDuplicated(cgs []dv1.ComputeGroup) string {
	/*	n_d, _ := validateCCNameDuplicated(cgs)
		cid_d, _ := validateCCIdDuplicated(cgs)
		ds := n_d
		if cid_d != "" {
			ds = ds + ";" + cid_d
		}

		if ds == "" {
			return ds, false
		}
		return ds, true*/

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

/*// validate the name of compute group is duplicated or not in compute group.
// the cc name must be configured.
func validateCCNameDuplicated(ccs []dv1.ComputeGroup) (string, bool) {
	ss := set.NewSetString()
	for _, cc := range ccs {
		if ss.Find(cc.Name) {
			return cc.Name, true
		}
		ss.Add(cc.Name)
	}

	return "", false
}

// if cluster id have already configured, checking repeating or not. if not configured ignoring check.
func validateCCIdDuplicated(ccs []dv1.ComputeGroup) (string, bool) {
	scids := set.NewSetString()
	for _, cc := range ccs {
		if cc.ClusterId != "" && scids.Find(cc.ClusterId) {
			return cc.ClusterId, true
		}
		scids.Add(cc.ClusterId)
	}
	return "", false
}
*/

// clear not configed cg resources, delete not configed cg status from ddc.status .
func (dcgs *DisaggregatedComputeGroupsController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	var clearCGs []dv1.ComputeGroupStatus
	var eCGs []dv1.ComputeGroupStatus

	for i, cgs := range ddc.Status.ComputeGroupStatuses {
		for _, cg := range ddc.Spec.ComputeGroups {
			if cgs.UniqueId == cg.UniqueId {
				eCGs = append(eCGs, ddc.Status.ComputeGroupStatuses[i])
				goto NoNeedAppend
			}
		}

		clearCGs = append(clearCGs, ddc.Status.ComputeGroupStatuses[i])
		// no need clear should not append.
	NoNeedAppend:
	}

	for i := range clearCGs {
		cgs := clearCGs[i]
		cleared := true
		if err := k8s.DeleteStatefulset(ctx, dcgs.K8sclient, ddc.Namespace, cgs.StatefulsetName); err != nil {
			cleared = false
			klog.Errorf("disaggregatedComputeGroupsController delete statefulset namespace %s name %s failed, err=%s", ddc.Namespace, cgs.StatefulsetName, err.Error())
			dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGStatefulsetDeleteFailed), err.Error())
		}

		if err := k8s.DeleteService(ctx, dcgs.K8sclient, ddc.Namespace, cgs.ServiceName); err != nil {
			cleared = false
			klog.Errorf("disaggregatedComputeGroupsController delete service namespace %s name %s failed, err=%s", ddc.Namespace, cgs.ServiceName, err.Error())
			dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGServiceDeleteFailed), err.Error())
		}
		if !cleared {
			eCGs = append(eCGs, clearCGs[i])
		} else {
			// drop compute group from meta
			response, err := ms_http.DropComputeGroup(ddc.Status.MetaServiceStatus.MetaServiceEndpoint, ddc.Status.MetaServiceStatus.MsToken, ddc.Status.InstanceId, &cgs)
			if err != nil {
				klog.Errorf("computeGroupSync ClearResources DropComputeGroup response failed , response: %s", err.Error())
				dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGHTTPFailed), "DropComputeGroup request failed: "+err.Error())
			}
			if response.Code != ms_http.SuccessCode {
				jsonData, _ := json.Marshal(response)
				klog.Errorf("computeGroupSync ClearResources DropComputeGroup response failed , response: %s", jsonData)
				dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CGHTTPFailed), "DropComputeGroup request failed: "+response.Msg)
			}

		}

	}

	for i := range eCGs {
		err := dcgs.ClearStatefulsetUnusedPVCs(ctx, ddc, eCGs[i])
		if err != nil {
			klog.Errorf("disaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs clear whole ComputeGroup PVC failed, err=%s", err.Error())
		}
	}
	for i := range clearCGs {
		err := dcgs.ClearStatefulsetUnusedPVCs(ctx, ddc, clearCGs[i])
		if err != nil {
			klog.Errorf("disaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs clear part ComputeGroup PVC failed, err=%s", err.Error())
		}
	}

	ddc.Status.ComputeGroupStatuses = eCGs

	return true, nil
}

// ClearStatefulsetUnusedPVCs
// 1.delete unused pvc skip cluster is Suspend
// 2.delete unused pvc for statefulset
// 3.delete pvc if not used by any statefulset
func (dcgs *DisaggregatedComputeGroupsController) ClearStatefulsetUnusedPVCs(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster, cgs dv1.ComputeGroupStatus) error {
	var cg *dv1.ComputeGroup
	for i := range ddc.Spec.ComputeGroups {
		clusterId := ddc.GetCGId(&ddc.Spec.ComputeGroups[i])
		if clusterId == cgs.ClusterId {
			cg = &ddc.Spec.ComputeGroups[i]
		}
	}

	currentPVCs := corev1.PersistentVolumeClaimList{}
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)

	pvcLabels := dcgs.newCGPodsSelector(ddc.Name, cgs.ClusterId)

	if err := dcgs.K8sclient.List(ctx, &currentPVCs, client.InNamespace(ddc.Namespace), client.MatchingLabels(pvcLabels)); err != nil {
		dcgs.K8srecorder.Event(ddc, string(sc.EventWarning), sc.PVCListFailed, fmt.Sprintf("DisaggregatedComputeGroupsController ClearStatefulsetUnusedPVCs list pvc failed:%s!", err.Error()))
		return err
	}

	for i := range currentPVCs.Items {
		pvcMap[currentPVCs.Items[i].Name] = &currentPVCs.Items[i]
	}

	if cg != nil {
		replicas := int(*cg.Replicas)
		stsName := ddc.GetCGStatefulsetName(cg)
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
	selector := dcgs.newCGPodsSelector(ddc.Name, cgs.ClusterId)
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

func (dfc *DisaggregatedComputeGroupsController) dropCGFromHttpClient(cluster *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) error {
	cgReplica := cg.Replicas

	// drop be can also use the unique id of fe
	unionId := "1:" + cluster.GetInstanceId() + ":" + cluster.GetFEStatefulsetName() + "-0"
	clusterId := cluster.GetCGId(cg)
	cgNodes, err := ms_http.GetBECluster(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, unionId, clusterId)
	if err != nil {
		klog.Errorf("dropCGFromHttpClient GetBECluster failed, err:%s ", err.Error())
		return err
	}

	var dropNodes []*ms_http.NodeInfo
	for _, node := range cgNodes {
		splitCloudUniqueIDArr := strings.Split(node.CloudUniqueID, "-")
		podNum, err := strconv.Atoi(splitCloudUniqueIDArr[len(splitCloudUniqueIDArr)-1])
		if err != nil {
			klog.Errorf("splitCloudUniqueIDArr can not split CloudUniqueID : %s,err:%s", node.CloudUniqueID, err.Error())
			return err
		}
		if podNum >= int(*cgReplica) {
			dropNodes = append(dropNodes, node)
		}
	}
	if len(dropNodes) == 0 {
		return nil
	}

	reqCluster := ms_http.Cluster{
		//	ClusterName: cg.Name,
		ClusterID: clusterId,
		Type:      ms_http.BeNodeType,
		Nodes:     dropNodes,
	}

	specifyCluster, err := ms_http.DropBENodes(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, cluster.GetInstanceId(), reqCluster)
	if err != nil {
		klog.Errorf("dropCGFromHttpClient DropBENodes failed, err:%s ", err.Error())
		return err
	}

	if specifyCluster.Code != ms_http.SuccessCode {
		jsonData, _ := json.Marshal(specifyCluster)
		klog.Errorf("dropCGFromHttpClient DropBENodes response failed , response: %s", jsonData)
		return err
	}

	return nil
}
