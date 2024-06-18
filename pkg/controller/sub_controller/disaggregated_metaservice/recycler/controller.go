package recycler

import (
	"context"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller struct {
	sub_controller.DisaggregatedSubDefaultController
}

var (
	disaggregatedRecyclerController = "disaggregatedRecyclerController"
)

func New(mgr ctrl.Manager) *Controller {
	return &Controller{
		sub_controller.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(disaggregatedRecyclerController),
			ControllerName: disaggregatedRecyclerController,
		}}
}

func (rc *Controller) Sync(ctx context.Context, obj client.Object) error {
	//TODO implement me
	return nil
}

func (rc *Controller) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	//TODO implement me
	return true, nil
}

func (rc *Controller) GetControllerName() string {
	return disaggregatedRecyclerController
}

func (rc *Controller) UpdateComponentStatus(obj client.Object) error {
	//TODO implement me
	return nil
}
