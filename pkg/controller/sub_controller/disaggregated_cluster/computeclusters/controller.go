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
	"errors"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
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
	//1. generate resources.
	//2. initial compute cluster status.
	//3. sync resources.
	//TODO: 3. judge suspend
	if cc.Replicas != nil && *cc.Replicas == 0 {
		ms_http.SuspendComputeCluster()
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
	event, err = dccs.reconcileStatefulset(ctx, st)
	if err != nil {
		klog.Errorf("disaggregatedComputeClustersController reconcile statefulset namespace %s name %s failed, err=%s", st.Namespace, st.Name, err.Error())
	}

	return event, err
}

func (dccs *DisaggregatedComputeClustersController) reconcileStatefulset(ctx context.Context, st *appv1.StatefulSet) (*sc.Event, error) {
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

	return nil, nil
}

// initial compute cluster status before sync resources. status changing with sync steps, and generate the last status by classify pods.
func (dccs *DisaggregatedComputeClustersController) initialCCStatus(ddc *dv1.DorisDisaggregatedCluster, cc *dv1.ComputeCluster) {
	ccss := ddc.Status.ComputeClusterStatuses
	for i, _ := range ccss {
		if ccss[i].ComputeClusterName == cc.Name || ccss[i].ClusterId == cc.ClusterId {
			ccss[i].Phase = dv1.Reconciling
			return
		}
	}

	ccs := dv1.ComputeClusterStatus{
		Phase:              dv1.Reconciling,
		ComputeClusterName: cc.Name,
		ClusterId:          cc.ClusterId,
		//set for status updated.
		Replicas: *cc.Replicas,
	}
	if ddc.Status.ComputeClusterStatuses == nil {
		ddc.Status.ComputeClusterStatuses = []dv1.ComputeClusterStatus{}
	}
	ddc.Status.ComputeClusterStatuses = append(ddc.Status.ComputeClusterStatuses, ccs)
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
	for i, ccs := range ddc.Status.ComputeClusterStatuses {
		for _, cc := range ddc.Spec.ComputeClusters {
			if ccs.ComputeClusterName == cc.Name || ccs.ClusterId == cc.ClusterId {
				eCCs = append(eCCs, ddc.Status.ComputeClusterStatuses[i])
				goto NoNeedAppend
			}
		}

		clearCCs = append(clearCCs, ddc.Status.ComputeClusterStatuses[i])
		// no need clear should not append.
	NoNeedAppend:
	}

	for i, ccs := range clearCCs {
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
			//TODO: 12. drop compute cluster from meta
			ms_http.DropComputeCluster()
		}
	}

	//TODO:13. drop pvcs
	for _, cc := range eCCs {
		dccs.ClearStatefulsetUnusedPVCs(cc)
	}
	//TODO:13. drop pvcs
	for _, cc := range clearCCs {
		dccs.ClearStatefulsetUnusedPVCs(cc)
	}

	ddc.Status.ComputeClusterStatuses = eCCs

	return true, nil
}

func (dccs *DisaggregatedComputeClustersController) ClearStatefulsetUnusedPVCs(cc dv1.ComputeClusterStatus) {

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
