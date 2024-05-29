package fdb

import (
	"context"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ sub_controller.DisaggregatedSubController = &DisaggregatedFDBController{}

var (
	disaggregatedFDBController = "disaggregatedFDBController"
)

type DisaggregatedFDBController struct {
	k8sClient      client.Client
	k8sRecorder    record.EventRecorder
	controllerName string
}

func New(mgr ctrl.Manager) *DisaggregatedFDBController {
	return &DisaggregatedFDBController{
		k8sClient:      mgr.GetClient(),
		k8sRecorder:    mgr.GetEventRecorderFor(disaggregatedFDBController),
		controllerName: disaggregatedFDBController,
	}
}

func (fdbc *DisaggregatedFDBController) Sync(ctx context.Context, obj client.Object) error {
	//TODO implement me
	panic("implement me")
}

func (fdbc *DisaggregatedFDBController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (fdbc *DisaggregatedFDBController) GetControllerName() string {
	return fdbc.controllerName
}

func (fdbc *DisaggregatedFDBController) UpdateComponentStatus(obj client.Object) error {
	//TODO implement me
	panic("implement me")
}
