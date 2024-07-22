package unnamedwatches

import (
	"context"
	"github.com/selectdb/doris-operator/pkg/common/utils/certificate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ watch = &WatchMutationWebhookConfiguration{}

type WatchMutationWebhookConfiguration struct {
	client                           kubernetes.Interface
	Name                             string
	MutationWebhookConfigurationName string
	SecretNamespaceName              types.NamespacedName
	Type                             client.Object
}

// return the watch resource name.
func (w *WatchMutationWebhookConfiguration) GetName() string {
	return w.Name
}
func (w *WatchMutationWebhookConfiguration) GetType() client.Object {
	return w.Type
}

func (w *WatchMutationWebhookConfiguration) Create(event event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchMutationWebhookConfiguration) Update(event event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
	for _, req := range w.toReconcileRequest(event.ObjectNew) {
		limitingInterface.Add(req)
	}
}

func (w *WatchMutationWebhookConfiguration) Delete(event event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchMutationWebhookConfiguration) Generic(event event.GenericEvent, limitingInterface workqueue.RateLimitingInterface) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchMutationWebhookConfiguration) toReconcileRequest(object metav1.Object) []reconcile.Request {
	if object.GetName() == w.Name {
		return []reconcile.Request{reconcile.Request{}}
	}

	return nil
}

func (w *WatchMutationWebhookConfiguration) Reconcile(ctx context.Context) error {
	secret, err := w.client.CoreV1().Secrets(w.SecretNamespaceName.Namespace).Get(ctx, w.SecretNamespaceName.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("watchMutationWebhookConfiguration reconcile failed to get secret, error=%s", err.Error())
		return err
	}

	mutationWebhook, err := w.client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, w.MutationWebhookConfigurationName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("watchMutationWebhookConfiguration get MutationWebhookConfiguration name=%s failed, err=%s.", w.MutationWebhookConfigurationName, err.Error())
		return err
	}

	cert := secret.Data[certificate.TLsCertName]
	for i, _ := range mutationWebhook.Webhooks {
		mutationWebhook.Webhooks[i].ClientConfig.CABundle = cert
	}
	if _, err := w.client.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(ctx, mutationWebhook, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("watchMutationWebhookConfiguration reconcile update MutationWebhookConfiguration name=%s, failed, err=%s", w.MutationWebhookConfigurationName, err.Error())
		return err
	}

	return nil
}
