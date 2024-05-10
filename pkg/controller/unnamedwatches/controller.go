package unnamedwatches

import (
	"context"
	pc "github.com/selectdb/doris-operator/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type watch interface {
	handler.EventHandler
	//get the watch identification.
	GetName() string

	//return the watch type
	GetType() client.Object
	// update the specific resource when it is not valid.
	Reconcile(ctx context.Context) error
}

// define the need resource for unnamedwatches
type WResource struct {
	watches []watch
}

var (
	WebhookWatchSecretName                    = "doris-operator-webhook-secret-watch"
	DefaultMutatingWebhookConfigurationName   = "doris-operator-mutate-webhook"
	DefaultValidatingWebhookConfigurationName = "doris-operator-validate-webhook"
)
var (
	OperatorPodSelector       = "control-plane=doris-operator"
	OperatorPodTimeAnnotation = "doris-operator/update-time"
)

var (
	_ reconcile.Reconciler = &WResource{}
	_ pc.Controller        = &WResource{}
)

// webhook need watch resources for running.
func (wr *WResource) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	if err := wr.ReconcileResource(ctx); err != nil {
		klog.Errorf("wresource reconcile failed to reconcile some resource, error=%s", err.Error())
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// reconciles the resources about certificates.
func (wr *WResource) ReconcileResource(ctx context.Context) error {
	for _, w := range wr.watches {
		if err := w.Reconcile(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (wr *WResource) Init(mgr ctrl.Manager, options *pc.Options) {
	if !options.EnableWebHook {
		klog.Infof("wresource init not enable.")
		return
	}

	cfg := mgr.GetConfig()
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Errorf("wresource init build clientset from mgr failed, err=%s", err.Error())
		os.Exit(1)
	}

	ws := &WatchSecret{
		client: clientset,
		Name:   WebhookWatchSecretName,
		NamespaceName: types.NamespacedName{
			Name:      options.SecretName,
			Namespace: options.Namespace,
		},
		WebhookService:                     options.WebhookService,
		Type:                               &corev1.Secret{},
		MutatingWebhookConfigurationName:   DefaultMutatingWebhookConfigurationName,
		ValidatingWebhookConfigurationName: DefaultValidatingWebhookConfigurationName,
	}

	wr.watches = append(wr.watches, ws)

	//force a first reconciliation to create the resources
	if err := wr.ReconcileResource(context.Background()); err != nil {
		klog.Errorf("first reconciliation failed to reconcile resource, err=%s", err.Error())
	}

	c, err := controller.New(options.Name, mgr, controller.Options{Reconciler: wr})
	if err != nil {
		klog.Errorf("wresource init new controller failed, err=%s", err.Error())
	}
	for _, w := range wr.watches {
		if err := c.Watch(&source.Kind{Type: w.GetType()}, w); err != nil {
			klog.Errorf("wresource init build clientset from mgr failed, err=%s", err.Error())
			os.Exit(1)
		}
	}

}
