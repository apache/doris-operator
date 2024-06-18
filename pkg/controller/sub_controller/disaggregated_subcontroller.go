package sub_controller

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DisaggregatedSubController interface {
	//Sync reconcile for sub controller. bool represent the component have updated.
	Sync(ctx context.Context, obj client.Object) error
	//clear all resource about sub-component.
	ClearResources(ctx context.Context, obj client.Object) (bool, error)

	//return the controller name, beController, feController,cnController for log.
	GetControllerName() string

	//UpdateStatus update the component status on src.
	UpdateComponentStatus(obj client.Object) error
}
