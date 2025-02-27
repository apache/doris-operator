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

/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.

func (r *DorisCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:unnamedwatches:path=/mutate-doris-selectdb-com-v1-doriscluster,mutating=true,failurePolicy=ignore,sideEffects=None,groups=doris.selectdb.com,resources=dorisclusters,verbs=create;update;delete,versions=v1,name=mdoriscluster.kb.io,admissionReviewVersions=v1
var _ webhook.CustomDefaulter = &DorisCluster{}

// Default implements webhook.Defaulter so a unnamedwatches will be registered for the type
func (r *DorisCluster) Default(ctx context.Context, obj runtime.Object) error {
	klog.Infof("mutatingwebhook mutate doriscluster name=%s.", r.Name)
	// TODO(user): fill in your defaulting logic.
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:unnamedwatches:path=/validate-doris-selectdb-com-v1-doriscluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=doris.selectdb.com,resources=dorisclusters,verbs=create;update,versions=v1,name=vdoriscluster.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &DorisCluster{}

// ValidateCreate implements webhook.Validator so a unnamedwatches will be registered for the type
func (r *DorisCluster) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	klog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a unnamedwatches will be registered for the type
func (r *DorisCluster) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	klog.Info("validate update", "name", r.Name)
	var errors []error
	// fe FeSpec.Replicas must greater than or equal to FeSpec.ElectionNumber
	if *r.Spec.FeSpec.Replicas < r.GetElectionNumber() {
		errors = append(errors, fmt.Errorf("'FeSpec.Replicas' error: the number of FeSpec.Replicas should greater than or equal to FeSpec.ElectionNumber"))
	}

	if len(errors) != 0 {
		return nil, kerrors.NewAggregate(errors)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a unnamedwatches will be registered for the type
func (r *DorisCluster) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	klog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
