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

package v1

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
func (ddc *DorisDisaggregatedCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(ddc).
		Complete()
}

// +kubebuilder:unnamedwatches:path=/mutate-disaggregated-doris-com-v1-dorisdisaggregatedcluster,mutating=true,failurePolicy=ignore,sideEffects=None,groups=disaggregated.cluster.doris.com,resources=dorisdisaggregatedclusters,verbs=create;update;delete,versions=v1,name=mdorisdisaggregatedcluster.kb.io,admissionReviewVersions=v1
var _ webhook.CustomDefaulter = &DorisDisaggregatedCluster{}

// Default implements webhook.Defaulter so a unnamedwatches will be registered for the type
func (ddc *DorisDisaggregatedCluster) Default(ctx context.Context, obj runtime.Object) error {
	klog.Infof("disaggregatedwebhook mutate disaggregated doris cluster name=%s.", ddc.Name)
	// TODO(user): fill in your defaulting logic.
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:unnamedwatches:path=/validate-disaggregated-doris-com-v1-dorisdisaggregatedcluster,mutating=false,failurePolicy=ignore,sideEffects=None,groups=disaggregated.cluster.doris.com,resources=dorisdisaggregatedclusters,verbs=create;update,versions=v1,name=vdorisdisaggregatedcluster.kb.io,admissionReviewVersions=v1
var _ webhook.CustomValidator = &DorisDisaggregatedCluster{}

// ValidateCreate implements webhook.Validator so a unnamedwatches will be registered for the type
func (ddc *DorisDisaggregatedCluster) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	klog.Info("validate create", "name", ddc.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a unnamedwatches will be registered for the type
func (ddc *DorisDisaggregatedCluster) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	klog.Info("validate update", "name", ddc.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a unnamedwatches will be registered for the type
func (ddc *DorisDisaggregatedCluster) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	klog.Info("validate delete", "name", ddc.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
