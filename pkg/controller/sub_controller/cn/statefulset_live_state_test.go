// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package cn

import (
	"context"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestHPAOwnedCNTemplateUpdatePreservesActualReplicas(t *testing.T) {
	for _, initial := range []*int32{nil, ptr.To(int32(3))} {
		t.Run(func() string {
			if initial == nil {
				return "omitted initial replicas"
			}
			return "explicit initial replicas"
		}(), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("HPA update panicked: %v", r)
				}
			}()
			old := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "cn", Namespace: "test"}, Spec: appsv1.StatefulSetSpec{Replicas: initial, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "cn", Image: "old"}}}}}}
			resource.StatefulsetDeepEqualWithKey(old, old.DeepCopy(), "", false)
			old.Spec.Replicas = ptr.To(int32(5))
			c := newCnController(t, old)
			desired := old.DeepCopy()
			desired.ResourceVersion = ""
			desired.Annotations = nil
			desired.Spec.Replicas = initial
			desired.Spec.Template.Spec.Containers[0].Image = "new"
			if err := c.applyStatefulSet(context.Background(), desired, true); err != nil {
				t.Fatal(err)
			}
			got := &appsv1.StatefulSet{}
			if err := c.K8sclient.Get(context.Background(), client.ObjectKeyFromObject(old), got); err != nil {
				t.Fatal(err)
			}
			if *got.Spec.Replicas != 5 || got.Spec.Template.Spec.Containers[0].Image != "new" {
				t.Fatalf("replicas=%d image=%s", *got.Spec.Replicas, got.Spec.Template.Spec.Containers[0].Image)
			}
		})
	}
}
