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

package be

import (
	"context"
	"testing"

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newBEReplicaTestStatefulSet(n int32) *appsv1.StatefulSet {
	bc := &Controller{}
	dcr := &dorisv1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "doris", Namespace: "test"}, Spec: dorisv1.DorisClusterSpec{
		FeSpec: &dorisv1.FeSpec{BaseSpec: dorisv1.BaseSpec{Replicas: ptr.To(int32(3)), Image: "doris:fe"}},
		BeSpec: &dorisv1.BeSpec{BaseSpec: dorisv1.BaseSpec{Replicas: ptr.To(n), Image: "doris:be"}}}}
	st := bc.buildBEStatefulSet(dcr, map[string]interface{}{})
	return &st
}
func cacheBEStatefulSetHash(st *appsv1.StatefulSet) {
	resource.StatefulsetDeepEqualWithKey(st, st.DeepCopy(), "", false)
}
func TestBEStatefulSetReconcilesReplicaDrift(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		desired, actual, cached int32
		legacy                  bool
	}{
		{"manual_replica_drift", 20, 21, 20, false},
		{"manual_replica_reduction", 20, 19, 20, false},
		{"normal_CR_scale_up", 21, 20, 20, false},
		{"normal_CR_scale_down", 20, 21, 21, false},
		{"legacy_env_scale_up", 21, 20, 20, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actual := newBEReplicaTestStatefulSet(tt.cached)
			if tt.legacy {
				actual.Spec.Template.Spec.Containers[0].Env = append(actual.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "CONFIGMAP_MOUNT_PATH", Value: "/etc/doris"})
			}
			cacheBEStatefulSetHash(actual)
			actual.Spec.Replicas = ptr.To(tt.actual)
			originalTemplate := actual.Spec.Template.DeepCopy()
			c := fake.NewClientBuilder().WithObjects(actual).Build()
			bc := &Controller{}
			desired := newBEReplicaTestStatefulSet(tt.desired)
			if err := k8s.ApplyStatefulSet(context.Background(), c, desired, bc.statefulSetsEqual); err != nil {
				t.Fatal(err)
			}
			got := &appsv1.StatefulSet{}
			if err := c.Get(context.Background(), types.NamespacedName{Name: actual.Name, Namespace: actual.Namespace}, got); err != nil {
				t.Fatal(err)
			}
			if *got.Spec.Replicas != tt.desired {
				t.Fatalf("replicas=%d want=%d", *got.Spec.Replicas, tt.desired)
			}
			if !equality.Semantic.DeepEqual(originalTemplate, &got.Spec.Template) {
				t.Fatal("replica-only update changed pod template")
			}
			t.Logf("replicas=%d pod template unchanged", *got.Spec.Replicas)
			if !tt.legacy && !bc.statefulSetsEqual(newBEReplicaTestStatefulSet(tt.desired), got) {
				t.Fatal("repeat comparison should be equal")
			}
		})
	}
}
func TestBEStatefulSetCreatesMissingResource(t *testing.T) {
	c := fake.NewClientBuilder().Build()
	bc := &Controller{}
	desired := newBEReplicaTestStatefulSet(20)
	if err := k8s.ApplyStatefulSet(context.Background(), c, desired, bc.statefulSetsEqual); err != nil {
		t.Fatal(err)
	}
	got := &appsv1.StatefulSet{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, got); err != nil {
		t.Fatal(err)
	}
	if *got.Spec.Replicas != 20 {
		t.Fatal("wrong replica count")
	}
}
