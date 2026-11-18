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
package k8s_test

import (
	"context"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestApplyRejectsForeignOwnedResource(t *testing.T) {
	expected := metav1.OwnerReference{APIVersion: "doris.selectdb.com/v1", Kind: "DorisCluster", Name: "doris", UID: "current"}
	for _, test := range []struct {
		name   string
		owners []metav1.OwnerReference
		ok     bool
	}{
		{"legacy owner", []metav1.OwnerReference{expected}, true},
		{"old cluster UID", []metav1.OwnerReference{{APIVersion: expected.APIVersion, Kind: expected.Kind, Name: expected.Name, UID: "old"}}, false},
		{"unowned collision", nil, false},
		{"foreign controller", []metav1.OwnerReference{expected, {APIVersion: "other/v1", Kind: "Other", Name: "other", UID: "other", Controller: ptr.To(true)}}, false},
	} {
		for _, kind := range []string{"Service", "StatefulSet"} {
			t.Run(test.name+"/"+kind, func(t *testing.T) {
				var old, desired client.Object
				if kind == "Service" {
					old = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "collision", Namespace: "test", OwnerReferences: test.owners}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "original"}}}
					desired = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "collision", Namespace: "test", OwnerReferences: []metav1.OwnerReference{expected}}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "new"}}}
				} else {
					old = &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "collision", Namespace: "test", OwnerReferences: test.owners}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(1))}}
					desired = &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "collision", Namespace: "test", OwnerReferences: []metav1.OwnerReference{expected}}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(2))}}
				}
				c := fake.NewClientBuilder().WithObjects(old).Build()
				var err error
				if kind == "Service" {
					err = k8s.ApplyService(context.Background(), c, desired.(*corev1.Service), resource.ServiceDeepEqual)
				} else {
					err = k8s.ApplyStatefulSet(context.Background(), c, desired.(*appsv1.StatefulSet), func(*appsv1.StatefulSet, *appsv1.StatefulSet) bool { return false })
				}
				if (err == nil) != test.ok {
					t.Fatalf("allowed=%v err=%v", test.ok, err)
				}
				if !test.ok {
					got := old.DeepCopyObject().(client.Object)
					if err := c.Get(context.Background(), client.ObjectKeyFromObject(old), got); err != nil {
						t.Fatal(err)
					}
					if kind == "Service" && got.(*corev1.Service).Spec.Selector["app"] != "original" {
						t.Fatal("foreign selector overwritten")
					}
					if kind == "StatefulSet" && *got.(*appsv1.StatefulSet).Spec.Replicas != 1 {
						t.Fatal("foreign replicas overwritten")
					}
				}
			})
		}
	}
}

func TestHPAUpdateRejectsForeignOwner(t *testing.T) {
	old := &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "cn-hpa", Namespace: "test", OwnerReferences: []metav1.OwnerReference{{Name: "other", UID: "other"}}}, Spec: autoscalingv2.HorizontalPodAutoscalerSpec{MaxReplicas: 3}}
	desired := old.DeepCopy()
	desired.OwnerReferences = []metav1.OwnerReference{{Name: "doris", UID: "current"}}
	desired.Spec.MaxReplicas = 7
	c := fake.NewClientBuilder().WithObjects(old).Build()
	if err := k8s.CreateOrUpdateClientObject(context.Background(), c, desired); err == nil {
		t.Fatal("foreign HPA was adopted")
	}
	got := &autoscalingv2.HorizontalPodAutoscaler{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(old), got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.MaxReplicas != 3 {
		t.Fatal("foreign HPA was overwritten")
	}
}
