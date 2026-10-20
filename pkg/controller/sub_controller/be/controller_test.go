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
package be

import (
	"context"
	"testing"

	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newBeController(t *testing.T, objects ...client.Object) (*Controller, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appsv1 to scheme failed, err=%s", err.Error())
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 to scheme failed, err=%s", err.Error())
	}
	k8sclient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	return New(k8sclient, record.NewFakeRecorder(10)), k8sclient
}

func Test_ClearResources(t *testing.T) {
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Status: v1.DorisClusterStatus{
			BEStatus: &v1.ComponentStatus{
				ComponentCondition: v1.ComponentCondition{
					Phase: v1.Available,
				},
			},
		},
	}

	resources := []client.Object{&appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-be",
			Namespace: "default",
		},
		Spec: appv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1.OwnerReference:    "test",
					v1.ComponentLabelKey: "be",
				},
			},
			Replicas: resource.GetInt32Pointer(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "be",
					Labels: map[string]string{
						v1.OwnerReference:    "test",
						v1.ComponentLabelKey: "be",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "be",
							Image: "test.fe:1",
						},
					},
				},
			},
		},
	},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-be-internal",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					v1.OwnerReference:    "test",
					v1.ComponentLabelKey: "be",
				},
				Ports: []corev1.ServicePort{
					{
						Name:     "test",
						Protocol: corev1.ProtocolTCP,
						Port:     9030,
					},
				},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-be-service",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					v1.OwnerReference:    "test",
					v1.ComponentLabelKey: "be",
				},
				Ports: []corev1.ServicePort{
					{
						Name:     "test",
						Protocol: corev1.ProtocolTCP,
						Port:     9030,
					},
				},
			},
		}}

	bc, k8sclient := newBeController(t, resources...)
	if _, err := bc.ClearResources(context.Background(), cluster); err != nil {
		t.Errorf("Test_ClearResources clear resources failed, err=%s", err.Error())
	}

	for i := range resources {
		name := resources[i].GetName()
		err := k8sclient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, resources[i])
		if !apierrors.IsNotFound(err) {
			t.Errorf("Test_ClearResources the resource is not deleted, name=%s, err=%v", name, err)
		}
	}
}
