package recycle

import (
	"context"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ sub_controller.DisaggregatedSubController = &DisaggregatedRecycleController{}

var (
	disaggregatedRecycleController = "disaggregatedRecycleController"
)

type DisaggregatedRecycleController struct {
	k8sClient   client.Client
	k8sRecorder record.EventRecorder
}

func New(mgr ctrl.Manager) *DisaggregatedRecycleController {
	return &DisaggregatedRecycleController{
		k8sClient:   mgr.GetClient(),
		k8sRecorder: mgr.GetEventRecorderFor(disaggregatedRecycleController),
	}
}

func (rc *DisaggregatedRecycleController) Sync(ctx context.Context, obj client.Object) error {
	//TODO implement me
	panic("implement me")
}

func (rc *DisaggregatedRecycleController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (rc *DisaggregatedRecycleController) GetControllerName() string {
	return disaggregatedRecycleController
}

func (rc *DisaggregatedRecycleController) UpdateComponentStatus(obj client.Object) error {
	//TODO implement me
	panic("implement me")
}
