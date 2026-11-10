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

package disaggregated_fe

import (
	"context"
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestFrontendUpgradeWaitRepairsServices(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := &dv1.DorisDisaggregatedCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default", UID: "cluster", Annotations: map[string]string{"doris.apache.org/runtime-config-policy": "disabled"}}, Spec: dv1.DorisDisaggregatedClusterSpec{
		FeSpec:        dv1.FeSpec{ElectionNumber: ptr.To(int32(1)), CommonSpec: dv1.CommonSpec{Image: "fe:new", Replicas: ptr.To(int32(1))}},
		ComputeGroups: []dv1.ComputeGroup{{UniqueId: "be", CommonSpec: dv1.CommonSpec{Image: "be:new", Replicas: ptr.To(int32(1))}}},
	}}
	fe := &appv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "test-fe", Namespace: "default"}, Spec: appv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe", Image: "fe:old"}}}}}}
	ms := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: cluster.GetMSServiceName(), Namespace: "default"}, Subsets: []corev1.EndpointSubset{{Addresses: []corev1.EndpointAddress{{IP: "127.0.0.1"}}}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fe, ms).Build()
	fc := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8sclient: c, K8srecorder: record.NewFakeRecorder(10)}}
	if err := fc.Sync(context.Background(), cluster); !sc.IsBackendUpgradePending(err) {
		t.Fatalf("expected backend wait, got %v", err)
	}
	for _, name := range []string{cluster.GetFEServiceName(), cluster.GetFEInternalServiceName()} {
		if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, &corev1.Service{}); err != nil {
			t.Fatalf("Service repair blocked: %v", err)
		}
	}
	var actual appv1.StatefulSet
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: fe.Name}, &actual); err != nil {
		t.Fatal(err)
	}
	if actual.Spec.Template.Spec.Containers[0].Image != "fe:old" {
		t.Fatal("FE upgraded before BE")
	}
}
