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
	"crypto/x509/pkix"
	"fmt"
	"github.com/apache/doris-operator/pkg/common/utils/certificate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

var (
	certificateType = "HTTP"
	testDNSName     = "doris.example.com"
)

var _ watch = &WatchSecret{}

// define secret should be watched by operator.
type WatchSecret struct {
	client kubernetes.Interface
	//the watch controller name.
	Name                               string
	NamespaceName                      types.NamespacedName
	Type                               client.Object
	WebhookService                     string
	MutatingWebhookConfigurationName   string
	ValidatingWebhookConfigurationName string
}

// return the watch resource name.
func (w *WatchSecret) GetName() string {
	return w.Name
}
func (w *WatchSecret) GetType() client.Object {
	return w.Type
}

func (w *WatchSecret) Create(ctx context.Context, event event.TypedCreateEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchSecret) Update(ctx context.Context, event event.TypedUpdateEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(event.ObjectNew) {
		limitingInterface.Add(req)
	}
}

func (w *WatchSecret) Delete(ctx context.Context, event event.TypedDeleteEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchSecret) Generic(ctx context.Context, event event.TypedGenericEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	for _, req := range w.toReconcileRequest(event.Object) {
		limitingInterface.Add(req)
	}
}

func (w *WatchSecret) toReconcileRequest(object metav1.Object) []reconcile.Request {
	if object.GetName() == w.NamespaceName.Name {
		return []reconcile.Request{{
			NamespacedName: w.NamespaceName,
		}}
	}

	return nil
}

func (w *WatchSecret) Reconcile(ctx context.Context) error {
	secret, err := w.client.CoreV1().Secrets(w.NamespaceName.Namespace).Get(ctx, w.NamespaceName.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("watchSecret reconcile failed to get secret, error=%s", err.Error())
		return err
	}

	ca := certificate.BuildCAFromSecret(secret)
	if ca != nil && certificate.ValidCA(ca) {
		return nil
	}

	dnsNames := []string{
		fmt.Sprintf("%s.%s", w.WebhookService, w.NamespaceName.Namespace),
		fmt.Sprintf("%s.%s.svc", w.WebhookService, w.NamespaceName.Namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", w.WebhookService, w.NamespaceName.Namespace),
		testDNSName,
	}

	// build new ca.
	cp := certificate.CAOptions{
		Subject: pkix.Name{
			CommonName:   w.GetName() + "-" + certificateType,
			Organization: []string{w.GetName()},
		},
		DnsNames: dnsNames,
	}
	ca, err = certificate.NewCAConfigSecret(cp)
	if err != nil {
		klog.Errorf("watchSecret reconcile failed to newCa, error=%s.", err)
		return err
	}

	ns := w.generateCASecret(ca)
	//update secret data
	secret.Data = ns.Data

	if _, err := w.client.CoreV1().Secrets(w.NamespaceName.Namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("watchSecret reconcile update secret name=%s，in namespace=%s failed, err=%s", secret.Name, secret.Namespace, err.Error())
		return err
	}

	mutatingWebhook, err := w.client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, w.MutatingWebhookConfigurationName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("watchSecret reconcile get mutatingwebhookconfiguration name=%s failed, err=%s.", w.MutatingWebhookConfigurationName, err.Error())
		return err
	}
	for i, _ := range mutatingWebhook.Webhooks {
		mutatingWebhook.Webhooks[i].ClientConfig.CABundle = ca.GetEncodeCert()
	}

	if _, err := w.client.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(ctx, mutatingWebhook, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("watchSecret reconcile update mutatingwebhookconfiguration name=%s failed, err=%s.", w.MutatingWebhookConfigurationName, err.Error())
		return err
	}

	validatingWebhook, err := w.client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, w.ValidatingWebhookConfigurationName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("watchSecret reconcile get validatingwebhookconfiguration name=%s failed, err=%s.", w.ValidatingWebhookConfigurationName, err.Error())
		return err
	}

	for i, _ := range validatingWebhook.Webhooks {
		validatingWebhook.Webhooks[i].ClientConfig.CABundle = ca.GetEncodeCert()
	}

	if _, err := w.client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(ctx, validatingWebhook, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("watchSecret reconcile update validatingwebhookconfiguration name=%s failed, err=%s.", w.ValidatingWebhookConfigurationName, err.Error())
		return err
	}
	w.updateOperatorPods(ctx, w.client, w.NamespaceName.Namespace)
	return nil
}

func (w *WatchSecret) updateOperatorPods(ctx context.Context, client kubernetes.Interface, operatorNamespace string) {
	labels := metav1.ListOptions{
		LabelSelector: OperatorPodSelector,
	}

	pods, err := client.CoreV1().Pods(operatorNamespace).List(ctx, labels)
	if err != nil {
		klog.Errorf("wresource updateOperatorPods list pod failed, err=%s", err.Error())
		return
	}

	for _, pod := range pods.Items {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Fetch the last the version of the Pod
			pod, err := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			// Update the annotation
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[OperatorPodTimeAnnotation] = time.Now().Format(time.RFC3339Nano)
			_, err = client.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
			return err
		})
		if err != nil {
			klog.Errorf("wresource updateOperatorPods update pod name=%s failed, err=%s", pod.Name, err.Error())
		}
	}
}

// construct ca secret for operator and apiserver.
func (w *WatchSecret) generateCASecret(ca *certificate.CA) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      w.NamespaceName.Name,
			Namespace: w.NamespaceName.Namespace,
		},
	}

	s.Data = make(map[string][]byte, 2)
	s.Data[certificate.TlsKeyName] = ca.GetEncodePrivateKey()
	s.Data[certificate.TLsCertName] = ca.GetEncodeCert()
	return s
}

// check the certificate for resource is valid.
func (w *WatchSecret) shouldRenewCertificate(secret *corev1.Secret) bool {
	if secret == nil {
		return true
	}

	ca := certificate.BuildCAFromSecret(secret)
	if ca == nil {
		return true
	}

	return certificate.ValidCA(ca)
}

var _ watch = &WatchSecret{}
