package computegroups

import (
	"context"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ sub_controller.DisaggregatedSubController = &DisaggregatedComputeGroupsController{}

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
	//TODO implement me
	panic("implement me")
}

func (dccs *DisaggregatedComputeGroupsController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (dccs *DisaggregatedComputeGroupsController) GetControllerName() string {
	return dccs.controllerName
}

func (dccs *DisaggregatedComputeGroupsController) UpdateComponentStatus(obj client.Object) error {
	//TODO implement me
	panic("implement me")
}
