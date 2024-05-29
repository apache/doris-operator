package controller

import (
	"context"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	dcgs "github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/computegroups"
	dfe "github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/disaggregated_fe"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	_ reconcile.Reconciler = &DisaggregatedClusterReconciler{}
	_ Controller           = &DisaggregatedClusterReconciler{}
)

var (
	disaggregatedClusterController = "disaggregatedClusterController"
)

type DisaggregatedClusterReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Scs      map[string]sub_controller.DisaggregatedSubController
}

func (dc *DisaggregatedClusterReconciler) Init(mgr ctrl.Manager, options *Options) {
	scs := make(map[string]sub_controller.DisaggregatedSubController)
	dfec := dfe.New(mgr)
	scs[dfec.GetControllerName()] = dfec
	dcgsc := dcgs.New(mgr)
	scs[dcgsc.GetControllerName()] = dcgsc

	if err := (&DisaggregatedClusterReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor(disaggregatedClusterController),
		Scs:      scs,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller ", "disaggregatedClusterReconciler")
		os.Exit(1)
	}
}

func (dc *DisaggregatedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return dc.resourceBuilder(ctrl.NewControllerManagedBy(mgr)).Complete(dc)
}

func (dc *DisaggregatedClusterReconciler) resourceBuilder(builder *ctrl.Builder) *ctrl.Builder {
	return builder.For(&dv1.DorisDisaggregatedCluster{}).
		Owns(&appv1.StatefulSet{}).
		Owns(&corev1.Service{})
}

func (dc *DisaggregatedClusterReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var dcr dv1.DorisDisaggregatedCluster
	err := dc.Get(ctx, req.NamespacedName, &dcr)
	if apierrors.IsNotFound(err) {
		klog.Warningf("disaggreatedClusterReconciler not find resource DorisDisaggregatedCluster namespaceName %s", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	//TODO: test, verify ListWatch work
	klog.Infof("the reconcile disaggregatedCluster namespace %s, name %s", req.Namespace, req.Name)
	//TODO: implement the logic to reconcile DisaggregatedCluster

	return ctrl.Result{}, nil
}
