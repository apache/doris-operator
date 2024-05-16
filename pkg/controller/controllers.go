package controller

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	//Controllers through the init for add Controller.
	Controllers []Controller
	Scheme      = runtime.NewScheme()
)

type Controller interface {
	Init(mgr ctrl.Manager, options *Options)
}
