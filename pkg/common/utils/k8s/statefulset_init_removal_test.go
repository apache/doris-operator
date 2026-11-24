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

package k8s

import (
	"context"
	"testing"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Turning on skipDefaultSystemInit for an existing BE/compute group renders a pod template with
// no init container. Applying it must remove the "default-init" the live StatefulSet still has:
// an empty InitContainers is omitted from the object's JSON, so a merge patch built from the
// desired object alone would leave the old init container in place.
func TestApplyStatefulSetRemovesDroppedInitContainer(t *testing.T) {
	privileged := true
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "be"}}
	existing := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "be", Namespace: "test"},
		Spec: appv1.StatefulSetSpec{
			Selector: selector,
			Replicas: pointer.Int32(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "be"}},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Name:            "default-init",
						Image:           "be-image",
						SecurityContext: &corev1.SecurityContext{Privileged: &privileged},
					}},
					Containers: []corev1.Container{{Name: "compute", Image: "be-image"}},
				},
			},
		},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(existing).Build()

	desired := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "be", Namespace: "test"},
		Spec: appv1.StatefulSetSpec{
			Selector: selector,
			Replicas: pointer.Int32(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "be"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "compute",
						Image: "be-image",
						Env:   []corev1.EnvVar{{Name: "SKIP_CHECK_ULIMIT", Value: "true"}},
					}},
				},
			},
		},
	}

	notEqual := func(*appv1.StatefulSet, *appv1.StatefulSet) bool { return false }
	if err := ApplyStatefulSet(context.Background(), fakeClient, desired, notEqual); err != nil {
		t.Fatalf("apply statefulset: %v", err)
	}

	var got appv1.StatefulSet
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "test", Name: "be"}, &got); err != nil {
		t.Fatalf("get statefulset: %v", err)
	}
	if env := got.Spec.Template.Spec.Containers[0].Env; len(env) != 1 || env[0].Name != "SKIP_CHECK_ULIMIT" {
		t.Fatalf("the update was not applied at all: env = %v", env)
	}
	if n := len(got.Spec.Template.Spec.InitContainers); n != 0 {
		t.Errorf("expected no init containers after the update, got %d: %s", n, got.Spec.Template.Spec.InitContainers[0].Name)
	}
}
