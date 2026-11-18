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
package computegroups

import (
	"context"
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestReconcileStatefulsetCorrectsActualReplicaDrift(t *testing.T) {
	desired := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "component", Namespace: "audit"}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(3)), Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "good"}}}}}}
	old := desired.DeepCopy()
	resource.StatefulsetDeepEqualWithKey(old, old.DeepCopy(), dv1.DisaggregatedSpecHashValueAnnotation, false)
	old.Spec.Replicas = ptr.To(int32(2))
	c := fake.NewClientBuilder().WithObjects(old).Build()
	controller := &DisaggregatedComputeGroupsController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8sclient: c}}
	ddc := &dv1.DorisDisaggregatedCluster{Spec: dv1.DorisDisaggregatedClusterSpec{FeSpec: dv1.FeSpec{CommonSpec: dv1.CommonSpec{Replicas: ptr.To(int32(3))}}}}
	ddc.Status.ComputeGroupStatuses = []dv1.ComputeGroupStatus{{UniqueId: "cg", Phase: dv1.Ready}}
	if _, err := controller.reconcileStatefulset(context.Background(), desired, ddc, &dv1.ComputeGroup{UniqueId: "cg", CommonSpec: dv1.CommonSpec{Replicas: ptr.To(int32(3))}}); err != nil {
		t.Fatal(err)
	}
	got := &appsv1.StatefulSet{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(old), got); err != nil {
		t.Fatal(err)
	}
	if *got.Spec.Replicas != 3 {
		t.Fatalf("replica drift remains: %d", *got.Spec.Replicas)
	}
}
