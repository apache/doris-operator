package controller

import (
	v1 "github.com/selectdb/doris-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	//Controllers through the init for add Controller.
	Controllers []Controller
	Scheme      = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(v1.AddToScheme(Scheme))
	//+kubebuilder:scaffold:scheme
}

type Controller interface {
	Init(mgr ctrl.Manager)
}
