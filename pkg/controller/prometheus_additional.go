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

package controller

import (
	"context"

	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func reconcilePrometheusAdditionalSecret(ctx context.Context, c client.Client, owner client.Object, enabled bool,
	build func() (*corev1.Secret, error)) error {
	if !enabled {
		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: owner.GetNamespace(), Name: resource.PrometheusAdditionalSecretName(owner.GetName())}
		err := c.Get(ctx, key, secret)
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !metav1.IsControlledBy(secret, owner) {
			return nil
		}
		return client.IgnoreNotFound(c.Delete(ctx, secret))
	}

	secret, err := build()
	if err != nil {
		return err
	}

	existing := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), existing); err == nil {
		if equality.Semantic.DeepEqual(existing.Data, secret.Data) &&
			equality.Semantic.DeepEqual(existing.Labels, secret.Labels) &&
			equality.Semantic.DeepEqual(existing.OwnerReferences, secret.OwnerReferences) {
			return nil
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	return k8s.CreateOrUpdateClientObject(ctx, c, secret)
}
