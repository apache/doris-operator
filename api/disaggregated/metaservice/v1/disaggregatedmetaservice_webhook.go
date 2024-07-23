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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
func (ddm *DorisDisaggregatedMetaService) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(ddm).
		Complete()
}

// +kubebuilder:unnamedwatches:path=/mutate-disaggregated-metaservice-doris-com-v1-dorisdisaggregatedmetaservice,mutating=true,failurePolicy=ignore,sideEffects=None,groups=disaggregated.metaservice.doris.com,resources=dorisdisaggregatedmetaservices,verbs=create;update;delete,versions=v1,name=mdorisdisaggregatedmetaservice.kb.io,admissionReviewVersions=v1
var _ webhook.Defaulter = &DorisDisaggregatedMetaService{}

// Default implements webhook.Defaulter so a unnamedwatches will be registered for the type
func (ddm *DorisDisaggregatedMetaService) Default() {
	klog.Infof("mutatingwebhook mutate metaservice name=%s.", ddm.Name)
	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:unnamedwatches:path=/validate-disaggregated-metaservice-doris-com-v1-dorisdisaggregatedmetaservice,mutating=false,failurePolicy=ignore,sideEffects=None,groups=disaggregated.metaservice.doris.com,resources=dorisdisaggregatedmetaservices,verbs=create;update,versions=v1,name=vdorisdisaggregatedmetaservice.kb.io,admissionReviewVersions=v1
var _ webhook.Validator = &DorisDisaggregatedMetaService{}

// ValidateCreate implements webhook.Validator so a unnamedwatches will be registered for the type
func (ddm *DorisDisaggregatedMetaService) ValidateCreate() error {
	klog.Info("validate create", "name", ddm.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a unnamedwatches will be registered for the type
func (ddm *DorisDisaggregatedMetaService) ValidateUpdate(old runtime.Object) error {
	klog.Info("validate update", "name", ddm.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a unnamedwatches will be registered for the type
func (ddm *DorisDisaggregatedMetaService) ValidateDelete() error {
	klog.Info("validate delete", "name", ddm.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
