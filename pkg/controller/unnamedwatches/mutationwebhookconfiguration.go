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

func (w *WatchMutationWebhookConfiguration) Create(ctx context.Context, event event.TypedCreateEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchMutationWebhookConfiguration) Update(ctx context.Context, event event.TypedUpdateEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(event.ObjectNew) {
		limitingInterface.Add(req)
	}
}

func (w *WatchMutationWebhookConfiguration) Delete(ctx context.Context, event event.TypedDeleteEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchMutationWebhookConfiguration) Generic(ctx context.Context, event event.TypedGenericEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchMutationWebhookConfiguration) toReconcileRequest(metaObject metav1.Object) []reconcile.Request {
	if metaObject.GetName() == w.Name {
		return []reconcile.Request{reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      metaObject.GetName(),
				Namespace: metaObject.GetNamespace(),
			},
		}}
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
