package ms

import (
	"context"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller struct {
	sub_controller.DisaggregatedSubDefaultController
}

var (
	disaggregatedMSController = "disaggregatedMSController"
)

func New(mgr ctrl.Manager) *Controller {
	return &Controller{
		sub_controller.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(disaggregatedMSController),
			ControllerName: disaggregatedMSController,
		}}
}

func (msc *Controller) Sync(ctx context.Context, obj client.Object) error {
	dms := obj.(*mv1.DorisDisaggregatedMetaService)

	if dms.Status.FDBStatus.AvailableStatus != mv1.Available {
		klog.Info("MS Controller Sync: ", "the FDB is UnAvailable ", "namespace ", dms.Namespace, " disaggregated doris cluster name ", dms.Name)
		return nil
	}

	msc.initMSStatus(dms)
	msSpec := dms.Spec.MS

	config, err := msc.GetMSConfig(ctx, msSpec.ConfigMaps, dms.Namespace, mv1.Component_MS)
	if err != nil {
		klog.Error("MS Controller Sync ", "resolve ms configmap failed, namespace ", dms.Namespace, " error :", err)
		return err
	}

	msc.CheckMSConfigMountPath(dms, mv1.Component_MS)

	// MS only Build Internal Service
	internalService := resource.BuildDMSInternalService(dms, mv1.Component_MS, config)
	if err := k8s.ApplyService(ctx, msc.K8sclient, &internalService, resource.DMSServiceDeepEqual); err != nil {
		klog.Errorf("MS controller sync apply internalService name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			internalService.Name, internalService.Namespace, dms.Name, err.Error())
		return err
	}

	if !msc.PrepareMSReconcileResources(ctx, dms, mv1.Component_MS) {
		klog.Infof("MS controller sync preparing resource for reconciling namespace %s name %s!", dms.Namespace, dms.Name)
		return nil
	}

	// TODO prepareStatefulsetApply
	st := msc.buildMSStatefulSet(dms)

	if err = k8s.ApplyStatefulSet(ctx, msc.K8sclient, &st, func(new *appv1.StatefulSet, old *appv1.StatefulSet) bool {
		msc.RestrictConditionsEqual(new, old)
		return resource.DMSStatefulSetDeepEqual(new, old, false)
	}); err != nil {
		klog.Errorf("MS controller sync statefulset name=%s, namespace=%s, disaggregated-metaservice-name=%s failed. message=%s.",
			st.Name, st.Namespace, dms.Name, err.Error())
		return err
	}

	return nil
}

// ClearResources clear resources for MS
// 1. clear deleted pod Resources if necessary
// 2. clear deleted statefulset Resources When CR is marked as cleared
func (msc *Controller) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	dms := obj.(*mv1.DorisDisaggregatedMetaService)
	// clear deleted pod Resources
	if err := msc.RecycleResources(ctx, dms, mv1.Component_MS); err != nil {
		klog.Errorf("MS ClearResources recycle pvc resource for reconciling namespace %s name %s!", dms.Namespace, dms.Name)
		return false, err
	}
	// DeletionTimestamp is IsZero means dms not delete
	// clear deleted statefulset Resources
	if dms.DeletionTimestamp.IsZero() {
		return true, nil
	}

	if dms.Spec.MS == nil {
		return msc.ClearCommonResources(ctx, dms, mv1.Component_MS)
	}
	return true, nil
}

func (msc *Controller) GetControllerName() string {
	return msc.ControllerName
}

func (msc *Controller) UpdateComponentStatus(obj client.Object) error {
	dms := obj.(*mv1.DorisDisaggregatedMetaService)

	if dms.Spec.MS == nil {
		return nil
	}
	return msc.ClassifyPodsByStatus(dms.Namespace, &dms.Status.MSStatus, mv1.GenerateStatefulSetSelector(dms, mv1.Component_MS), *dms.Spec.MS.Replicas)
}

func (d *Controller) initMSStatus(dms *mv1.DorisDisaggregatedMetaService) {
	initPhase := mv1.Creating

	if mv1.IsReconcilingStatusPhase(dms.Status.MSStatus.Phase) {
		initPhase = dms.Status.MSStatus.Phase
	}
	status := mv1.BaseStatus{
		Phase:           initPhase,
		AvailableStatus: mv1.UnAvailable,
	}
	dms.Status.MSStatus = status
}
