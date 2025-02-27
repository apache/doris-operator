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
	pc "github.com/apache/doris-operator/pkg/controller"
	v1 "k8s.io/api/admissionregistration/v1"
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
	WatchSecretWebhookName                    = "doris-operator-webhook-secret-watch"
	WatchValidatingWebhookConfigurationName   = "doris-operator-validate-webhook-watch"
	WatchMutatingWebhookConfigurationName     = "doris-operator-mutate-webhook-watch"
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
		klog.Infof("wresource init not enable and WebHook not enable.")
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
		Name:   WatchSecretWebhookName,
		NamespaceName: types.NamespacedName{
			Name:      options.SecretName,
			Namespace: options.Namespace,
		},
		WebhookService:                     options.WebhookService,
		Type:                               &corev1.Secret{},
		MutatingWebhookConfigurationName:   DefaultMutatingWebhookConfigurationName,
		ValidatingWebhookConfigurationName: DefaultValidatingWebhookConfigurationName,
	}

	wv := &WatchValidatingWebhookConfiguration{
		client: clientset,
		Name:   WatchValidatingWebhookConfigurationName,
		SecretNamespaceName: types.NamespacedName{
			Name:      options.SecretName,
			Namespace: options.Namespace,
		},
		ValidatingWebhookConfigurationName: DefaultValidatingWebhookConfigurationName,
		Type:                               &v1.ValidatingWebhookConfiguration{},
	}

	wm := &WatchMutationWebhookConfiguration{
		client: clientset,
		Name:   WatchMutatingWebhookConfigurationName,
		SecretNamespaceName: types.NamespacedName{
			Name:      options.SecretName,
			Namespace: options.Namespace,
		},
		MutationWebhookConfigurationName: DefaultMutatingWebhookConfigurationName,
		Type:                             &v1.MutatingWebhookConfiguration{},
	}

	wr.watches = append(wr.watches, ws, wv, wm)

	//force a first reconciliation to create the resources
	if err := wr.ReconcileResource(context.Background()); err != nil {
		klog.Errorf("first reconciliation failed to reconcile resource, err=%s", err.Error())
	}

	c, err := controller.New(options.Name, mgr, controller.Options{Reconciler: wr})
	if err != nil {
		klog.Errorf("wresource init new controller failed, err=%s", err.Error())
	}
	for _, w := range wr.watches {
		if err := c.Watch(source.Kind(mgr.GetCache(), w.GetType(), w)); err != nil {
			klog.Errorf("wresource init build clientset from mgr failed, err=%s", err.Error())
			os.Exit(1)
		}
	}

}
