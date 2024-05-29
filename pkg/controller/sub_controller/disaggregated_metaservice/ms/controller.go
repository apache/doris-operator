package ms

import (
	"context"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ sub_controller.DisaggregatedSubController = &DisaggregatedMSController{}

var (
	disaggregatedMSController = "disaggregatedMSController"
)

type DisaggregatedMSController struct {
	k8sClient      client.Client
	k8sRecorder    record.EventRecorder
	controllerName string
}

func New(mgr ctrl.Manager) *DisaggregatedMSController {
	return &DisaggregatedMSController{
		k8sClient:   mgr.GetClient(),
		k8sRecorder: mgr.GetEventRecorderFor(disaggregatedMSController),
	}
}

func (msc *DisaggregatedMSController) Sync(ctx context.Context, obj client.Object) error {
	//TODO implement me
	panic("implement me")
}

func (msc *DisaggregatedMSController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (msc *DisaggregatedMSController) GetControllerName() string {
	return msc.controllerName
}

func (msc *DisaggregatedMSController) UpdateComponentStatus(obj client.Object) error {
	//TODO implement me
	panic("implement me")
}
