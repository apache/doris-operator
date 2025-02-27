// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package unnamedwatches

import (
	"context"
	"github.com/apache/doris-operator/pkg/common/utils/certificate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ watch = &WatchValidatingWebhookConfiguration{}

type WatchValidatingWebhookConfiguration struct {
	client kubernetes.Interface
	//the watch controller name.
	Name                               string
	ValidatingWebhookConfigurationName string
	SecretNamespaceName                types.NamespacedName
	Type                               client.Object
}

// return the watch resource name.
func (w *WatchValidatingWebhookConfiguration) GetName() string {
	return w.Name
}
func (w *WatchValidatingWebhookConfiguration) GetType() client.Object {
	return w.Type
}

func (w *WatchValidatingWebhookConfiguration) Create(ctx context.Context, e event.TypedCreateEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(e.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchValidatingWebhookConfiguration) Update(ctx context.Context, e event.TypedUpdateEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(e.ObjectNew) {
		limitingInterface.Add(req)
	}
}

func (w *WatchValidatingWebhookConfiguration) Delete(ctx context.Context, e event.TypedDeleteEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(e.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchValidatingWebhookConfiguration) Generic(ctx context.Context, e event.TypedGenericEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(e.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchValidatingWebhookConfiguration) toReconcileRequest(e metav1.Object) []reconcile.Request {
	if e.GetName() == w.Name {
		return []reconcile.Request{reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      e.GetName(),
				Namespace: e.GetNamespace(),
			},
		}}
	}

	return nil
}

func (w *WatchValidatingWebhookConfiguration) Reconcile(ctx context.Context) error {
	secret, err := w.client.CoreV1().Secrets(w.SecretNamespaceName.Namespace).Get(ctx, w.SecretNamespaceName.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("watchValidatingWebhookConfiguration reconcile failed to get secret, error=%s", err.Error())
		return err
	}

	validatingWebhook, err := w.client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, w.ValidatingWebhookConfigurationName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("watchValidatingWebhookConfiguration get ValidatingWebhookConfiguration name=%s failed, err=%s.", w.ValidatingWebhookConfigurationName, err.Error())
		return err
	}

	cert := secret.Data[certificate.TLsCertName]
	for i, _ := range validatingWebhook.Webhooks {
		validatingWebhook.Webhooks[i].ClientConfig.CABundle = cert
	}
	if _, err := w.client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(ctx, validatingWebhook, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("watchValidatingWebhookConfiguration reconcile update validatingWebhookConfiguration name=%s， failed, err=%s.", w.ValidatingWebhookConfigurationName, err.Error())
		return err
	}

	return nil
}
