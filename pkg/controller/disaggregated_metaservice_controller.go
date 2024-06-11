package controller

import (
	"context"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_metaservice/fdb"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_metaservice/ms"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_metaservice/recycle"
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
	_ reconcile.Reconciler = &DisaggregatedMetaServiceReconciler{}

	_ Controller = &DisaggregatedMetaServiceReconciler{}
)

var (
	disaggregatedMetaServiceControllerName = "disaggregatedMetaServiceControllerName"
)

type DisaggregatedMetaServiceReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Scs      map[string]sub_controller.DisaggregatedSubController
}

func (dms *DisaggregatedMetaServiceReconciler) Init(mgr ctrl.Manager, options *Options) {
	scs := make(map[string]sub_controller.DisaggregatedSubController)
	dfdbc := fdb.New(mgr)
	scs[dfdbc.GetControllerName()] = dfdbc
	dmsc := ms.New(mgr)
	scs[dmsc.GetControllerName()] = dmsc
	dryc := recycle.New(mgr)
	scs[dryc.GetControllerName()] = dryc

	if err := (&DisaggregatedMetaServiceReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor("disaggregatedMetaServiceControllerName"),
		Scs:      scs,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller ", "disaggregatedMetaServiceReconciler")
		os.Exit(1)
	}

	if options.EnableWebHook {
		if err := (&mv1.DorisDisaggregatedMetaService{}).SetupWebhookWithManager(mgr); err != nil {
			klog.Error(err, "  unable to create unamedwatches ", " controller ", " DorisDisaggregatedMetaService ")
			os.Exit(1)
		}
	}
}

func (dms *DisaggregatedMetaServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return dms.resourceBuilder(ctrl.NewControllerManagedBy(mgr)).Complete(dms)
}

func (dms *DisaggregatedMetaServiceReconciler) resourceBuilder(builder *ctrl.Builder) *ctrl.Builder {
	return builder.For(&mv1.DorisDisaggregatedMetaService{}).Owns(&appv1.StatefulSet{}).Owns(&corev1.Service{})
}

func (dms *DisaggregatedMetaServiceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var dmsr mv1.DorisDisaggregatedMetaService
	err := dms.Get(ctx, req.NamespacedName, &dmsr)
	if apierrors.IsNotFound(err) {
		klog.Warningf("disaggregatedMetaServiceReconciler not find resource DisaggregatedMetaService namespace %s", req.Namespace)
		return ctrl.Result{}, nil
	}

	//TODO: test, verify ListWatch work
	klog.Infof("the reconcile disaggregatedMetaService namespace %s, name %s", req.Namespace, req.Name)
	//TODO: implement the logic to reconcile DisaggregatedCluster

	return ctrl.Result{}, nil
}
