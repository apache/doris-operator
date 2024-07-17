package computegroups

import (
	"bytes"
	"context"
	"errors"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/common/utils/set"
	sc "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

var _ sc.DisaggregatedSubController = &DisaggregatedComputeGroupsController{}

var (
	disaggregatedComputeGroupsController = "disaggregatedComputeGroupsController"
)

type DisaggregatedComputeGroupsController struct {
	k8sClient      client.Client
	k8sRecorder    record.EventRecorder
	controllerName string
}

func New(mgr ctrl.Manager) *DisaggregatedComputeGroupsController {
	return &DisaggregatedComputeGroupsController{
		k8sClient:      mgr.GetClient(),
		k8sRecorder:    mgr.GetEventRecorderFor(disaggregatedComputeGroupsController),
		controllerName: disaggregatedComputeGroupsController,
	}
}

func (dccs *DisaggregatedComputeGroupsController) Sync(ctx context.Context, obj client.Object) error {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	if len(ddc.Spec.ComputeGroups) == 0 {
		klog.Errorf("disaggregatedComputeGroupsController sync disaggregatedDorisCluster namespace=%s,name=%s have not compute group spec.", ddc.Namespace, ddc.Name)
		dccs.k8sRecorder.Event(ddc, string(sc.EventWarning), string(sc.ComputeGroupsEmpty), "computegroups empty, the cluster will not work normal.")
		return nil
	}

	// validating compute group information.
	if event, res := dccs.validateComputeGroup(ddc.Spec.ComputeGroups); !res {
		klog.Errorf("disaggregatedComputeGroupsController namespace=%s name=%s validateComputeGroup have not match specifications %s.", ddc.Namespace, ddc.Name, sc.EventString(event))
		dccs.k8sRecorder.Eventf(ddc, string(event.Type), string(event.Reason), event.Message)
		return errors.New("validating cg failed")
	}

	cgs := ddc.Spec.ComputeGroups
	for i, _ := range cgs {
		// if be unique identifier updated, operator should revert it.
		dccs.revertNotAllowedUpdateFields(ddc, &cgs[i])
		if event, err := dccs.computeGroupSync(ctx, ddc, &cgs[i]); err != nil {
			if event != nil {
				dccs.k8sRecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
			}
			klog.Errorf("disaggregatedComputeGroupsController computeGroups sync failed, computegroup name %s clusterId %s sync failed, err=%s", cgs[i].Name, cgs[i].ClusterId, sc.EventString(event))
		}
	}

	return nil
}

// validate compute group config information.
func (dccs *DisaggregatedComputeGroupsController) validateComputeGroup(cgs []dv1.ComputeGroup) (*sc.Event, bool) {
	if dupl, duplicate := dccs.validateDuplicated(cgs); duplicate {
		klog.Errorf("disaggregatedComputeGroupsController validateComputeGroup validate Duplicated have duplicate unique identifier %s.", dupl)
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGUniqueIdentifierDuplicate, Message: "unique identifier " + dupl + " duplicate in compute groups."}, false
	}

	if reg, res := dccs.validateRegex(cgs); !res {
		klog.Errorf("disaggregatedComputeGroupsController validateComputeGroup validateRegex %s have not match regular expression", reg)
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGUniqueIdentifierNotMatchRegex, Message: reg}, false
	}

	return nil, true
}

func (dccs *DisaggregatedComputeGroupsController) feAvailable(ddc *dv1.DorisDisaggregatedCluster) bool {
	//if fe deploy in k8s, should wait fe available
	//1. wait for fe ok.
	endpoints := corev1.Endpoints{}
	if err := dccs.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.GetFEServiceName()}, &endpoints); err != nil {
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

func (dccs *DisaggregatedComputeGroupsController) computeGroupSync(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) (*sc.Event, error) {
	//1. generate resources.
	//2. initial computegroup status.
	//3. sync resources.
	cvs := dccs.getConfigValuesFromConfigMaps(ddc.Namespace, cg.CommonSpec.ConfigMaps)
	st := dccs.NewStatefulset(ddc, cg, cvs)
	svc := dccs.newService(ddc, cg, cvs)
	dccs.initialCGStatus(ddc, cg)

	event, err := dccs.reconcileService(ctx, svc)
	if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcile service namespace %s name %s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return event, err
	}
	event, err = dccs.reconcileStatefulset(ctx, st)
	if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcile statefulset namespace %s name %s failed, err=%s", st.Namespace, st.Name, err.Error())
	}

	return event, err
}

func (dccs *DisaggregatedComputeGroupsController) reconcileService(ctx context.Context, svc *corev1.Service) (*sc.Event, error) {
	var esvc corev1.Service
	if err := dccs.k8sClient.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, &esvc); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, dccs.k8sClient, svc); err != nil {
			klog.Errorf("disaggregatedComputeGroupsController reconcileService create service namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.CGCreateResourceFailed, Message: err.Error()}, err
		}
	} else if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcileService get service failed, namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return nil, err
	}

	if err := k8s.ApplyService(ctx, dccs.k8sClient, svc, func(nsvc, osvc *corev1.Service) bool {
		return resource.ServiceDeepEqualWithAnnoKey(nsvc, osvc, dv1.DisaggregatedSpecHashValueAnnotation)
	}); err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcileService apply service namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGApplyResourceFailed, Message: err.Error()}, err
	}

	return nil, nil
}

func (dccs *DisaggregatedComputeGroupsController) reconcileStatefulset(ctx context.Context, st *appv1.StatefulSet) (*sc.Event, error) {
	var est appv1.StatefulSet
	if err := dccs.k8sClient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, dccs.k8sClient, st); err != nil {
			klog.Errorf("disaggregatedComputeGroupsController reconcileStatefulset create statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.CGCreateResourceFailed, Message: err.Error()}, err
		}

		return nil, nil
	} else if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcileStatefulset get statefulset failed, namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return nil, err
	}

	if err := k8s.ApplyStatefulSet(ctx, dccs.k8sClient, st, func(st, est *appv1.StatefulSet) bool {
		return resource.StatefulsetDeepEqualWithAnnoKey(st, est, dv1.DisaggregatedSpecHashValueAnnotation, false)
	}); err != nil {
		klog.Errorf("disaggregatedComputeGroupsController reconcileStatefulset apply statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CGApplyResourceFailed, Message: err.Error()}, err
	}

	return nil, nil
}

// initial compute group status before sync resources. status changing with sync steps, and generate the last status by classify pods.
func (dccs *DisaggregatedComputeGroupsController) initialCGStatus(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) {
	cgss := ddc.Status.ComputeGroupStatuses
	for i, _ := range cgss {
		if cgss[i].ComputeGroupName == cg.Name || cgss[i].ClusterId == cg.ClusterId {
			cgss[i].Phase = dv1.Reconciling
			return
		}
	}

	cgs := dv1.ComputeGroupStatus{
		Phase:            dv1.Reconciling,
		ComputeGroupName: cg.Name,
		ClusterId:        cg.ClusterId,
		//set for status updated.
		Replicas: *cg.Replicas,
	}
	if ddc.Status.ComputeGroupStatuses == nil {
		ddc.Status.ComputeGroupStatuses = []dv1.ComputeGroupStatus{}
	}
	ddc.Status.ComputeGroupStatuses = append(ddc.Status.ComputeGroupStatuses, cgs)
}

// get compute start config from all configmaps that config in CR, resolve config for config ports in pod or service, etc.
func (dccs *DisaggregatedComputeGroupsController) getConfigValuesFromConfigMaps(namespace string, cms []dv1.ConfigMap) map[string]interface{} {
	if len(cms) == 0 {
		return nil
	}

	for _, cm := range cms {
		kcm, err := k8s.GetConfigMap(context.Background(), dccs.k8sClient, namespace, cm.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("disaggregatedComputeGroupsController getConfigValuesFromConfigMaps namespace=%s, name=%s, failed, err=%s", namespace, cm.Name, err.Error())
			continue
		}

		if _, ok := kcm.Data[resource.BE_RESOLVEKEY]; !ok {
			continue
		}

		v := kcm.Data[resource.BE_RESOLVEKEY]
		viper.ReadConfig(bytes.NewBuffer([]byte(v)))
		return viper.AllSettings()
	}

	return nil
}

// clusterId and cloudUniqueId is not allowed update, when be mistakenly modified on these fields, operator should revert it by status fields.
func (dccs *DisaggregatedComputeGroupsController) revertNotAllowedUpdateFields(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) {
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if (cgs.ComputeGroupName != "" && cgs.ComputeGroupName == cg.Name) || (cgs.ClusterId != "" && cgs.ClusterId == cg.ClusterId) {
			if cgs.ClusterId != "" && cgs.ClusterId != cg.ClusterId {
				cg.ClusterId = cgs.ClusterId
			}
		}
	}
}

// check compute groups unique identifier duplicated or not. return duplicated key.
func (dccs *DisaggregatedComputeGroupsController) validateDuplicated(cgs []dv1.ComputeGroup) (string, bool) {
	n_d, _ := validateCGNameDuplicated(cgs)
	cid_d, _ := validateCGClusterIdDuplicated(cgs)
	ds := n_d
	if cid_d != "" {
		ds = ds + ";" + cid_d
	}

	if ds == "" {
		return ds, false
	}
	return ds, true
}

// checking the cg name compliant with regular expression or not.
func (dccs *DisaggregatedComputeGroupsController) validateRegex(cgs []dv1.ComputeGroup) (string, bool) {
	var regStr = ""
	for _, cg := range cgs {
		res, err := regexp.Match(compute_group_name_regex, []byte(cg.Name))
		if !res {
			regStr = regStr + cg.Name + " not match " + compute_group_name_regex
		}
		//for debugging, output the error in log
		if err != nil {
			klog.Errorf("disaggregatedComputeGroupsController validateRegex cg name %s failed, err=%s", cg.Name, err.Error())
		}
	}
	if regStr != "" {
		return regStr, false
	}

	return "", true
}

// validate the name of compute group is duplicated or not in computegroups.
// the cg name must be configured.
func validateCGNameDuplicated(cgs []dv1.ComputeGroup) (string, bool) {
	ss := set.NewSetString()
	for _, cg := range cgs {
		if ss.Find(cg.Name) {
			return cg.Name, true
		}
		ss.Add(cg.Name)
	}

	return "", false
}

// if cluster id have already configured, checking repeating or not. if not configured ignoring check.
func validateCGClusterIdDuplicated(cgs []dv1.ComputeGroup) (string, bool) {
	scids := set.NewSetString()
	for _, cg := range cgs {
		if cg.ClusterId != "" && scids.Find(cg.ClusterId) {
			return cg.ClusterId, true
		}
		scids.Add(cg.ClusterId)
	}
	return "", false
}

// clear not configed cg resources, delete not configed cg status from ddc.status .
func (dccs *DisaggregatedComputeGroupsController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	var clearCGs []dv1.ComputeGroupStatus
	var eCGs []dv1.ComputeGroupStatus
	for i, cgs := range ddc.Status.ComputeGroupStatuses {
		for _, cg := range ddc.Spec.ComputeGroups {
			if cgs.ComputeGroupName == cg.Name || cgs.ClusterId == cg.ClusterId {
				eCGs = append(eCGs, ddc.Status.ComputeGroupStatuses[i])
				goto NoNeedAppend
			}
		}

		clearCGs = append(clearCGs, ddc.Status.ComputeGroupStatuses[i])
		// no need clear should not append.
	NoNeedAppend:
	}

	for i, cgs := range clearCGs {
		cleared := true
		if err := k8s.DeleteStatefulset(ctx, dccs.k8sClient, ddc.Namespace, cgs.StatefulsetName); err != nil {
			cleared = false
			klog.Errorf("disaggregatedComputeGroupsController delete statefulset namespace %s name %s failed, err=%s", ddc.Namespace, cgs.StatefulsetName, err.Error())
			dccs.k8sRecorder.Event(ddc, string(sc.EventWarning), string(sc.CGStatefulsetDeleteFailed), err.Error())
		}

		if err := k8s.DeleteService(ctx, dccs.k8sClient, ddc.Namespace, cgs.ServiceName); err != nil {
			cleared = false
			klog.Errorf("disaggregatedComputeGroupsController delete service namespace %s name %s failed, err=%s", ddc.Namespace, cgs.ServiceName, err.Error())
			dccs.k8sRecorder.Event(ddc, string(sc.EventWarning), string(sc.CGStatefulsetDeleteFailed), err.Error())
		}
		if !cleared {
			eCGs = append(eCGs, clearCGs[i])
		}
	}

	ddc.Status.ComputeGroupStatuses = eCGs

	return true, nil
}

func (dccs *DisaggregatedComputeGroupsController) GetControllerName() string {
	return dccs.controllerName
}

func (dccs *DisaggregatedComputeGroupsController) UpdateComponentStatus(obj client.Object) error {
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
			errChan <- dccs.updateCGStatus(ddc, &cgss[idx])
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

func (dccs *DisaggregatedComputeGroupsController) updateCGStatus(ddc *dv1.DorisDisaggregatedCluster, cgs *dv1.ComputeGroupStatus) error {
	selector := dccs.newCGPodsSelector(ddc.Name, cgs.ClusterId)
	var podList corev1.PodList
	if err := dccs.k8sClient.List(context.Background(), &podList, client.InNamespace(ddc.Namespace), client.MatchingLabels(selector)); err != nil {
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
