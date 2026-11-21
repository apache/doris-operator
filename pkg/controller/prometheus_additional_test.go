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
	"testing"

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_ReconcilePrometheusAdditionalSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 to scheme failed: %s", err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()

	dcr := &dorisv1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "doris", UID: "demo-uid"},
		Spec: dorisv1.DorisClusterSpec{
			FeSpec: &dorisv1.FeSpec{},
			BeSpec: &dorisv1.BeSpec{},
		},
	}
	build := func() (*corev1.Secret, error) {
		return resource.BuildPrometheusAdditionalSecret(dcr)
	}

	for i := 0; i < 2; i++ {
		if err := reconcilePrometheusAdditionalSecret(ctx, c, dcr, true, build); err != nil {
			t.Fatalf("reconcile enabled failed: %s", err)
		}
		secret := &corev1.Secret{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: "doris", Name: "demo-prometheus-additional"}, secret); err != nil {
			t.Fatalf("get generated secret failed: %s", err)
		}
		if _, ok := secret.Data[resource.PrometheusAdditionalConfigKey]; !ok {
			t.Fatalf("generated secret missing config key")
		}
	}

	if err := reconcilePrometheusAdditionalSecret(ctx, c, dcr, false, build); err != nil {
		t.Fatalf("reconcile disabled failed: %s", err)
	}
	secret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{Namespace: "doris", Name: "demo-prometheus-additional"}, secret)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected secret deleted, got err: %v", err)
	}

	if err := reconcilePrometheusAdditionalSecret(ctx, c, dcr, false, build); err != nil {
		t.Fatalf("reconcile disabled twice failed: %s", err)
	}
}
