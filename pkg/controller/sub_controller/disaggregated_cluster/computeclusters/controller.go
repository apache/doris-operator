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

package computeclusters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_http"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/common/utils/set"
	sc "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
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

var _ sc.DisaggregatedSubController = &DisaggregatedComputeClustersController{}

var (
	disaggregatedComputeClustersController = "disaggregatedComputeClustersController"
)

type DisaggregatedComputeClustersController struct {
	sc.DisaggregatedSubDefaultController
}

func New(mgr ctrl.Manager) *DisaggregatedComputeClustersController {
	return &DisaggregatedComputeClustersController{
		sc.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(disaggregatedComputeClustersController),
			ControllerName: disaggregatedComputeClustersController,
		},
	}
}

func (dccs *DisaggregatedComputeClustersController) Sync(ctx context.Context, obj client.Object) error {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	if len(ddc.Spec.ComputeClusters) == 0 {
		klog.Errorf("disaggregatedComputeClustersController sync disaggregatedDorisCluster namespace=%s,name=%s have not compute cluster spec.", ddc.Namespace, ddc.Name)
		dccs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.ComputeClustersEmpty), "compute cluster empty, the cluster will not work normal.")
		return nil
	}

	if !dccs.feAvailable(ddc) {
		dccs.K8srecorder.Event(ddc, string(sc.EventNormal), string(sc.WaitFEAvailable), "fe have not ready.")
		return nil
	}

	// validating compute cluster information.
	if event, res := dccs.validateComputeCluster(ddc.Spec.ComputeClusters); !res {
		klog.Errorf("disaggregatedComputeClustersController namespace=%s name=%s validateComputeCluster have not match specifications %s.", ddc.Namespace, ddc.Name, sc.EventString(event))
		dccs.K8srecorder.Eventf(ddc, string(event.Type), string(event.Reason), event.Message)
		return errors.New("validating compute cluster failed")
	}

	ccs := ddc.Spec.ComputeClusters
	for i, _ := range ccs {
		// if be unique identifier updated, operator should revert it.
		dccs.revertNotAllowedUpdateFields(ddc, &ccs[i])
		if event, err := dccs.computeClusterSync(ctx, ddc, &ccs[i]); err != nil {
			if event != nil {
				dccs.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
			}
			klog.Errorf("disaggregatedComputeClustersController computeClusters sync failed, compute cluster name %s clusterId %s sync failed, err=%s", ccs[i].Name, ccs[i].ClusterId, sc.EventString(event))
		}
	}

	return nil
}

// validate compute cluster config information.
func (dccs *DisaggregatedComputeClustersController) validateComputeCluster(ccs []dv1.ComputeCluster) (*sc.Event, bool) {
	if dupl, duplicate := dccs.validateDuplicated(ccs); duplicate {
		klog.Errorf("disaggregatedComputeClustersController validateComputeCluster validate Duplicated have duplicate unique identifier %s.", dupl)
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CCUniqueIdentifierDuplicate, Message: "unique identifier " + dupl + " duplicate in compute clusters."}, false
	}

	if reg, res := dccs.validateRegex(ccs); !res {
		klog.Errorf("disaggregatedComputeClustersController validateComputeCluster validateRegex %s have not match regular expression", reg)
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CCUniqueIdentifierNotMatchRegex, Message: reg}, false
	}

	return nil, true
}

func (dccs *DisaggregatedComputeClustersController) feAvailable(ddc *dv1.DorisDisaggregatedCluster) bool {
	//if fe deploy in k8s, should wait fe available
	//1. wait for fe ok.
	endpoints := corev1.Endpoints{}
	if err := dccs.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.GetFEServiceName()}, &endpoints); err != nil {
		klog.Infof("disaggregatedComputeClustersController Sync wait fe service name %s available occur failed %s\n", ddc.GetFEServiceName(), err.Error())
		return false
	}

	for _, sub := range endpoints.Subsets {
		if len(sub.Addresses) > 0 {
			return true
		}
	}
	return false
}

func (dccs *DisaggregatedComputeClustersController) computeClusterSync(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster, cc *dv1.ComputeCluster) (*sc.Event, error) {
	if cc.Replicas == nil {
		cc.Replicas = resource.GetInt32Pointer(1)
	}
	cvs := dccs.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.BE_RESOLVEKEY, cc.CommonSpec.ConfigMaps)
	st := dccs.NewStatefulset(ddc, cc, cvs)
	svc := dccs.newService(ddc, cc, cvs)
	dccs.initialCCStatus(ddc, cc)

	event, err := dccs.DefaultReconcileService(ctx, svc)
	if err != nil {
		klog.Errorf("disaggregatedComputeClustersController reconcile service namespace %s name %s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return event, err
	}
	event, err = dccs.reconcileStatefulset(ctx, st, ddc, cc)
	if err != nil {
		klog.Errorf("disaggregatedComputeClustersController reconcile statefulset namespace %s name %s failed, err=%s", st.Namespace, st.Name, err.Error())
	}

	return event, err
}

func (dccs *DisaggregatedComputeClustersController) reconcileStatefulset(ctx context.Context, st *appv1.StatefulSet, cluster *dv1.DorisDisaggregatedCluster, cc *dv1.ComputeCluster) (*sc.Event, error) {
	var est appv1.StatefulSet
	if err := dccs.K8sclient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, dccs.K8sclient, st); err != nil {
			klog.Errorf("disaggregatedComputeClustersController reconcileStatefulset create statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.CCCreateResourceFailed, Message: err.Error()}, err
		}

		return nil, nil
	} else if err != nil {
		klog.Errorf("disaggregatedComputeClustersController reconcileStatefulset get statefulset failed, namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return nil, err
	}

	if err := k8s.ApplyStatefulSet(ctx, dccs.K8sclient, st, func(st, est *appv1.StatefulSet) bool {
		return resource.StatefulsetDeepEqualWithOmitKey(st, est, dv1.DisaggregatedSpecHashValueAnnotation, true, false)
	}); err != nil {
		klog.Errorf("disaggregatedComputeClustersController reconcileStatefulset apply statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CCApplyResourceFailed, Message: err.Error()}, err
	}

	var ccStatus *dv1.ComputeClusterStatus

	for i := range cluster.Status.ComputeClusterStatuses {
		if cluster.Status.ComputeClusterStatuses[i].ClusterId == cc.ClusterId {
			ccStatus = &cluster.Status.ComputeClusterStatuses[i]
			break
		}
	}

	//scaleType = "resume"
	if (*(st.Spec.Replicas) > *(est.Spec.Replicas) && *(est.Spec.Replicas) == 0) || ccStatus.Phase == dv1.ResumeFailed {
		klog.Errorf("------------  resume : %d", *(st.Spec.Replicas)-*(est.Spec.Replicas))
		response, err := ms_http.ResumeComputeCluster(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, cluster.Status.InstanceId, cc.ClusterId)
		if response.Code != ms_http.SuccessCode {
			ccStatus.Phase = dv1.ResumeFailed
			jsonData, _ := json.Marshal(response)
			klog.Errorf("computeClusterSync ResumeComputeCluster response failed , response: %s", jsonData)
			return &sc.Event{
				Type:    sc.EventNormal,
				Reason:  sc.CCResumeStatusRequestFailed,
				Message: "ResumeComputeCluster request of disaggregated BE failed: " + response.Msg,
			}, err
		}
		ccStatus.Phase = dv1.Scaling
	}

	//scaleType = "scaleDown"
	if (*(st.Spec.Replicas) < *(est.Spec.Replicas) && *(st.Spec.Replicas) > 0) || ccStatus.Phase == dv1.ScaleDownFailed {
		klog.Errorf("------------  scaleDown : %d", *(st.Spec.Replicas)-*(est.Spec.Replicas))
		if err := dccs.dropCCFromHttpClient(cluster, cc); err != nil {
			ccStatus.Phase = dv1.ScaleDownFailed
			klog.Errorf("ScaleDownBE failed, err:%s ", err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.CCHTTPFailed, Message: err.Error()},
				err
		}
		ccStatus.Phase = dv1.Scaling
	}

	//scaleType = "suspend"
	if (*(st.Spec.Replicas) < *(est.Spec.Replicas) && *(st.Spec.Replicas) == 0) || ccStatus.Phase == dv1.SuspendFailed {
		klog.Errorf("------------  suspend : %d", *(st.Spec.Replicas)-*(est.Spec.Replicas))
		response, err := ms_http.SuspendComputeCluster(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, cluster.Status.InstanceId, cc.ClusterId)
		if response.Code != ms_http.SuccessCode {
			ccStatus.Phase = dv1.SuspendFailed
			jsonData, _ := json.Marshal(response)
			klog.Errorf("computeClusterSync SuspendComputeCluster response failed , response: %s", jsonData)
			return &sc.Event{
				Type:    sc.EventNormal,
				Reason:  sc.CCSuspendStatusRequestFailed,
				Message: "SuspendComputeCluster request of disaggregated BE failed: " + response.Msg,
			}, err
		}
		ccStatus.Phase = dv1.Suspended
	}

	return nil, nil
}

// initial compute cluster status before sync resources. status changing with sync steps, and generate the last status by classify pods.
func (dccs *DisaggregatedComputeClustersController) initialCCStatus(ddc *dv1.DorisDisaggregatedCluster, cc *dv1.ComputeCluster) {
	ccss := ddc.Status.ComputeClusterStatuses
	klog.Errorf("------------------------------ init status for cc: %s ", cc.Name)
	defCcs := dv1.ComputeClusterStatus{
		Phase:              dv1.Reconciling,
		ComputeClusterName: cc.Name,
		ClusterId:          cc.ClusterId,
		StatefulsetName:    ddc.GetCCStatefulsetName(cc),
		ServiceName:        ddc.GetCCServiceName(cc),
		//set for status updated.
		Replicas: *cc.Replicas,
	}

	for i := range ccss {
		if ccss[i].ClusterId == cc.ClusterId {
			klog.Errorf("------------------------------ init status for status: %s ", ccss[i].Phase)
			if ccss[i].Phase == dv1.ScaleDownFailed || ccss[i].Phase == dv1.Suspended ||
				ccss[i].Phase == dv1.SuspendFailed || ccss[i].Phase == dv1.ResumeFailed ||
				ccss[i].Phase == dv1.Scaling {
				defCcs.Phase = ccss[i].Phase
			}
			ccss[i] = defCcs
			return
		}
	}

	ddc.Status.ComputeClusterStatuses = append(ddc.Status.ComputeClusterStatuses, defCcs)
}

// clusterId and cloudUniqueId is not allowed update, when be mistakenly modified on these fields, operator should revert it by status fields.
func (dccs *DisaggregatedComputeClustersController) revertNotAllowedUpdateFields(ddc *dv1.DorisDisaggregatedCluster, cc *dv1.ComputeCluster) {
	for _, ccs := range ddc.Status.ComputeClusterStatuses {
		if (ccs.ComputeClusterName != "" && ccs.ComputeClusterName == cc.Name) || (ccs.ClusterId != "" && ccs.ClusterId == cc.ClusterId) {
			if ccs.ClusterId != "" && ccs.ClusterId != cc.ClusterId {
				cc.ClusterId = ccs.ClusterId
			}
		}
	}
}

// check compute clusters unique identifier duplicated or not. return duplicated key.
func (dccs *DisaggregatedComputeClustersController) validateDuplicated(ccs []dv1.ComputeCluster) (string, bool) {
	n_d, _ := validateCCNameDuplicated(ccs)
	cid_d, _ := validateCCIdDuplicated(ccs)
	ds := n_d
	if cid_d != "" {
		ds = ds + ";" + cid_d
	}

	if ds == "" {
		return ds, false
	}
	return ds, true
}

// checking the cc name compliant with regular expression or not.
func (dccs *DisaggregatedComputeClustersController) validateRegex(ccs []dv1.ComputeCluster) (string, bool) {
	var regStr = ""
	for _, cc := range ccs {
		res, err := regexp.Match(compute_cluster_name_regex, []byte(cc.Name))
		if !res {
			regStr = regStr + cc.Name + " not match " + compute_cluster_name_regex
		}
		//for debugging, output the error in log
		if err != nil {
			klog.Errorf("disaggregatedComputeClustersController validateRegex compute cluster name %s failed, err=%s", cc.Name, err.Error())
		}
	}
	if regStr != "" {
		return regStr, false
	}

	return "", true
}

// validate the name of compute cluster is duplicated or not in compute cluster.
// the cc name must be configured.
func validateCCNameDuplicated(ccs []dv1.ComputeCluster) (string, bool) {
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
func validateCCIdDuplicated(ccs []dv1.ComputeCluster) (string, bool) {
	scids := set.NewSetString()
	for _, cc := range ccs {
		if cc.ClusterId != "" && scids.Find(cc.ClusterId) {
			return cc.ClusterId, true
		}
		scids.Add(cc.ClusterId)
	}
	return "", false
}

// clear not configed cc resources, delete not configed cc status from ddc.status .
func (dccs *DisaggregatedComputeClustersController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	var clearCCs []dv1.ComputeClusterStatus
	var eCCs []dv1.ComputeClusterStatus
	klog.Errorf("ComputeClusterStatuses len 0 -----------%d ", len(ddc.Status.ComputeClusterStatuses))

	for i, ccs := range ddc.Status.ComputeClusterStatuses {
		for _, cc := range ddc.Spec.ComputeClusters {
			if ccs.ClusterId == cc.ClusterId {
				eCCs = append(eCCs, ddc.Status.ComputeClusterStatuses[i])
				goto NoNeedAppend
			}
		}

		clearCCs = append(clearCCs, ddc.Status.ComputeClusterStatuses[i])
		// no need clear should not append.
	NoNeedAppend:
	}

	for i := range clearCCs {
		ccs := clearCCs[i]
		klog.Errorf("%d-------clearCCs:%s : %s ", i, ccs.ClusterId, ccs.StatefulsetName)
		cleared := true
		if err := k8s.DeleteStatefulset(ctx, dccs.K8sclient, ddc.Namespace, ccs.StatefulsetName); err != nil {
			cleared = false
			klog.Errorf("disaggregatedComputeClustersController delete statefulset namespace %s name %s failed, err=%s", ddc.Namespace, ccs.StatefulsetName, err.Error())
			dccs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CCStatefulsetDeleteFailed), err.Error())
		}

		if err := k8s.DeleteService(ctx, dccs.K8sclient, ddc.Namespace, ccs.ServiceName); err != nil {
			cleared = false
			klog.Errorf("disaggregatedComputeClustersController delete service namespace %s name %s failed, err=%s", ddc.Namespace, ccs.ServiceName, err.Error())
			dccs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CCServiceDeleteFailed), err.Error())
		}
		if !cleared {
			eCCs = append(eCCs, clearCCs[i])
		} else {
			// drop compute cluster from meta
			response, err := ms_http.DropComputeCluster(ddc.Status.MetaServiceStatus.MetaServiceEndpoint, ddc.Status.MetaServiceStatus.MsToken, ddc.Status.InstanceId, &ccs)
			if err != nil {
				klog.Errorf("computeClusterSync ClearResources DropComputeCluster response failed , response: %s", err.Error())
				dccs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CCHTTPFailed), "DropComputeCluster request failed: "+err.Error())
			}
			if response.Code != ms_http.SuccessCode {
				jsonData, _ := json.Marshal(response)
				klog.Errorf("computeClusterSync ClearResources DropComputeCluster response failed , response: %s", jsonData)
				dccs.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.CCHTTPFailed), "DropComputeCluster request failed: "+response.Msg)
			}

		}

	}

	// TODO:13. drop pvcs
	for i, _ := range eCCs {
		klog.Errorf(" drop eccs pvc  -----------%s " + eCCs[i].ClusterId)

		err := dccs.ClearStatefulsetUnusedPVCs(ctx, ddc, eCCs[i])
		if err != nil {
			klog.Errorf("disaggregatedComputeClustersController ClearStatefulsetUnusedPVCs eCCs failed, err=%s", err.Error())
		}
	}
	// TODO:13. drop pvcs
	for i, _ := range clearCCs {
		klog.Errorf("------------clearCCs pvc: ", clearCCs[i].ClusterId)
		err := dccs.ClearStatefulsetUnusedPVCs(ctx, ddc, clearCCs[i])
		if err != nil {
			klog.Errorf("disaggregatedComputeClustersController ClearStatefulsetUnusedPVCs clearCCs failed, err=%s", err.Error())
		}
	}

	klog.Errorf("ComputeClusterStatuses len 3 -----------%d ", len(ddc.Status.ComputeClusterStatuses))
	ddc.Status.ComputeClusterStatuses = eCCs
	klog.Errorf("ComputeClusterStatuses len 4 -----------%d ", len(ddc.Status.ComputeClusterStatuses))

	return true, nil
}

// ClearStatefulsetUnusedPVCs
// 1.delete unused pvc skip cluster is Suspend
// 2.delete unused pvc for statefulset
// 3.delete pvc if not used by any statefulset
func (dccs *DisaggregatedComputeClustersController) ClearStatefulsetUnusedPVCs(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster, ccs dv1.ComputeClusterStatus) error {
	var cc *dv1.ComputeCluster
	for i := range ddc.Spec.ComputeClusters {
		if ddc.Spec.ComputeClusters[i].ClusterId == ccs.ClusterId {
			cc = &ddc.Spec.ComputeClusters[i]
		}
	}

	currentPVCs := corev1.PersistentVolumeClaimList{}
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)

	pvcLabels := dccs.newCCPodsSelector(ddc.Name, ccs.ClusterId)

	if err := dccs.K8sclient.List(ctx, &currentPVCs, client.InNamespace(ddc.Namespace), client.MatchingLabels(pvcLabels)); err != nil {
		dccs.K8srecorder.Event(ddc, string(sc.EventWarning), sc.PVCListFailed, fmt.Sprintf("DisaggregatedComputeClustersController ClearStatefulsetUnusedPVCs list pvc failed:%s!", err.Error()))
		return err
	}

	for i := range currentPVCs.Items {
		pvcMap[currentPVCs.Items[i].Name] = &currentPVCs.Items[i]
	}

	if cc != nil {
		replicas := int(*cc.Replicas)
		stsName := ddc.GetCCStatefulsetName(cc)
		cvs := dccs.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.BE_RESOLVEKEY, cc.CommonSpec.ConfigMaps)
		paths, _ := dccs.getCacheMaxSizeAndPaths(cvs)

		if ccs.Phase == dv1.Suspended || ccs.Phase == dv1.SuspendFailed || replicas == 0 {
			klog.Infof("ClearStatefulsetUnusedPVCs compute cluster phase is %v, no need to delete pvc", ccs.Phase)
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
		if err := k8s.DeletePVC(ctx, dccs.K8sclient, claim.Namespace, claim.Name, pvcLabels); err != nil {
			dccs.K8srecorder.Event(ddc, string(sc.EventWarning), sc.PVCDeleteFailed, err.Error())
			klog.Errorf("ClearStatefulsetUnusedPVCs deletePVCs failed: namespace %s, name %s delete pvc %s, err: %s .", claim.Namespace, claim.Name, claim.Name, err.Error())
			mergeError = utils.MergeError(mergeError, err)
		}
	}
	return mergeError
}

func (dccs *DisaggregatedComputeClustersController) GetControllerName() string {
	return dccs.ControllerName
}

func (dccs *DisaggregatedComputeClustersController) UpdateComponentStatus(obj client.Object) error {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	ccss := ddc.Status.ComputeClusterStatuses
	if len(ccss) == 0 {
		klog.Errorf("disaggregatedComputeClustersController updateComponentStatus compute cluster status is empty!")
		return nil
	}

	errChan := make(chan error, len(ccss))
	wg := sync.WaitGroup{}
	wg.Add(len(ccss))
	for i, _ := range ccss {
		go func(idx int) {
			defer wg.Done()
			errChan <- dccs.updateCCStatus(ddc, &ccss[idx])
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
	for _, ccs := range ddc.Status.ComputeClusterStatuses {
		if ccs.Phase == dv1.Ready {
			fullAvailableCount++
		}
		if ccs.AvailableReplicas > 0 {
			availableCount++
		}
	}
	ddc.Status.ClusterHealth.CCCount = int32(len(ddc.Status.ComputeClusterStatuses))
	ddc.Status.ClusterHealth.CCFullAvailableCount = fullAvailableCount
	ddc.Status.ClusterHealth.CCAvailableCount = availableCount
	if errMs == "" {
		return nil
	}

	return errors.New(errMs)
}

func (dccs *DisaggregatedComputeClustersController) updateCCStatus(ddc *dv1.DorisDisaggregatedCluster, ccs *dv1.ComputeClusterStatus) error {
	selector := dccs.newCCPodsSelector(ddc.Name, ccs.ClusterId)
	var podList corev1.PodList
	if err := dccs.K8sclient.List(context.Background(), &podList, client.InNamespace(ddc.Namespace), client.MatchingLabels(selector)); err != nil {
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

	ccs.AvailableReplicas = availableReplicas
	if availableReplicas == ccs.Replicas {
		ccs.Phase = dv1.Ready
	}
	return nil
}

func (dfc *DisaggregatedComputeClustersController) dropCCFromHttpClient(cluster *dv1.DorisDisaggregatedCluster, cc *dv1.ComputeCluster) error {
	ccReplica := cc.Replicas

	// drop be can also use the unique id of fe
	unionId := "1:" + cluster.GetInstanceId() + ":" + cluster.GetFEStatefulsetName() + "-0"

	ccNodes, err := ms_http.GetBECluster(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, unionId, cc.ClusterId)
	if err != nil {
		klog.Errorf("dropCCFromHttpClient GetBECluster failed, err:%s ", err.Error())
		return err
	}

	var dropNodes []*ms_http.NodeInfo
	for _, node := range ccNodes {
		splitCloudUniqueIDArr := strings.Split(node.CloudUniqueID, "-")
		podNum, err := strconv.Atoi(splitCloudUniqueIDArr[len(splitCloudUniqueIDArr)-1])
		if err != nil {
			klog.Errorf("splitCloudUniqueIDArr can not split CloudUniqueID : %s,err:%s", node.CloudUniqueID, err.Error())
			return err
		}
		if podNum >= int(*ccReplica) {
			dropNodes = append(dropNodes, node)
		}
	}
	if len(dropNodes) == 0 {
		return nil
	}

	reqCluster := ms_http.Cluster{
		ClusterName: cc.Name,
		ClusterID:   cc.ClusterId,
		Type:        ms_http.BeNodeType,
		Nodes:       dropNodes,
	}

	specifyCluster, err := ms_http.DropBENodes(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, cluster.GetInstanceId(), reqCluster)
	if err != nil {
		klog.Errorf("dropCCFromHttpClient DropBENodes failed, err:%s ", err.Error())
		return err
	}

	if specifyCluster.Code != ms_http.SuccessCode {
		jsonData, _ := json.Marshal(specifyCluster)
		klog.Errorf("dropCCFromHttpClient DropBENodes response failed , response: %s", jsonData)
		return err
	}

	return nil
}
