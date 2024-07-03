package disaggregated_fe

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ sub_controller.DisaggregatedSubController = &DisaggregatedFEController{}

var (
	disaggregatedFEController = "disaggregatedFEController"
)

type DisaggregatedFEController struct {
	k8sClient      client.Client
	k8sRecorder    record.EventRecorder
	controllerName string
}

func New(mgr ctrl.Manager) *DisaggregatedFEController {
	return &DisaggregatedFEController{
		k8sClient:      mgr.GetClient(),
		k8sRecorder:    mgr.GetEventRecorderFor(disaggregatedFEController),
		controllerName: disaggregatedFEController,
	}
}

func (dfc *DisaggregatedFEController) Sync(ctx context.Context, obj client.Object) error {
	ddc := obj.(*v1.DorisDisaggregatedCluster)
	//TODO: reconcile fe
	if ddc == nil {
		//TODO implement me
		panic("implement me")
	}

	return nil
}

func (dfc *DisaggregatedFEController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	//TODO: implement me
	return true, nil
}

func (dfc *DisaggregatedFEController) GetControllerName() string {
	return disaggregatedFEController
}

func (dfc *DisaggregatedFEController) UpdateComponentStatus(obj client.Object) error {
	//TODO： implement me
	return nil
}
