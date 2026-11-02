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
package fe

import (
	"context"
	"testing"

	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kr "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newFeController(t *testing.T, objects ...client.Object) (*Controller, client.Client) {
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

// buildFePVC builds a pvc as the fe statefulset would name and label it.
func buildFePVC(name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				v1.OwnerReference:    "test-fe",
				v1.ComponentLabelKey: "fe",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources: corev1.VolumeResourceRequirements{
				Requests: map[corev1.ResourceName]kr.Quantity{
					"storage": kr.MustParse("500Gi"),
				},
			},
		},
	}
}

// Scaling fe down leaves pvcs above the new replica count behind, so ClearResources recycles them.
func Test_ClearResources(t *testing.T) {
	fc, k8sclient := newFeController(t, buildFePVC("test-fe-0"), buildFePVC("test-fe-1"), buildFePVC("test-fe-2"))
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1.DorisClusterSpec{
			FeSpec: &v1.FeSpec{
				BaseSpec: v1.BaseSpec{
					Replicas: resource.GetInt32Pointer(2),
					PersistentVolumes: []v1.PersistentVolume{
						{
							MountPath: "/opt/apache-doris/fe/doris-meta",
							PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": kr.MustParse("500Gi"),
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1.DorisClusterStatus{
			FEStatus: &v1.ComponentStatus{
				ComponentCondition: v1.ComponentCondition{
					Phase: v1.Available,
				},
			},
		},
	}

	cleared, err := fc.ClearResources(context.Background(), cluster)
	if err != nil {
		t.Errorf("Test_ClearResources clear resources failed, err=%s", err.Error())
	}
	if !cleared {
		t.Error("Test_ClearResources expected resources to be cleared")
	}

	for _, name := range []string{"test-fe-0", "test-fe-1"} {
		if err := k8sclient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, &corev1.PersistentVolumeClaim{}); err != nil {
			t.Errorf("Test_ClearResources the pvc %s should be kept, err=%s", name, err.Error())
		}
	}
	err = k8sclient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "test-fe-2"}, &corev1.PersistentVolumeClaim{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("Test_ClearResources the pvc test-fe-2 is not deleted, err=%v", err)
	}
}

// Removing feSpec while status.feStatus is still set must not dereference the nil spec in
// RecycleResources, and must clean up common FE resources.
func Test_ClearResources_NilFeSpec(t *testing.T) {
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1.DorisClusterSpec{
			FeSpec: nil,
		},
		Status: v1.DorisClusterStatus{
			FEStatus: &v1.ComponentStatus{
				ComponentCondition: v1.ComponentCondition{
					Phase: v1.Available,
				},
			},
		},
	}

	resources := []client.Object{
		&appv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-fe",
				Namespace: "default",
			},
			Spec: appv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1.OwnerReference:    "test",
						v1.ComponentLabelKey: "fe",
					},
				},
				Replicas: resource.GetInt32Pointer(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fe",
						Labels: map[string]string{
							v1.OwnerReference:    "test",
							v1.ComponentLabelKey: "fe",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "fe",
								Image: "test.fe:1",
							},
						},
					},
				},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-fe-internal",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					v1.OwnerReference:    "test",
					v1.ComponentLabelKey: "fe",
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
				Name:      "test-fe-service",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					v1.OwnerReference:    "test",
					v1.ComponentLabelKey: "fe",
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
	}

	fc, k8sclient := newFeController(t, resources...)
	cleared, err := fc.ClearResources(context.Background(), cluster)
	if err != nil {
		t.Errorf("Test_ClearResources_NilFeSpec clear resources failed, err=%s", err.Error())
	}
	if !cleared {
		t.Error("Test_ClearResources_NilFeSpec expected resources to be cleared")
	}

	for i := range resources {
		name := resources[i].GetName()
		err := k8sclient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, resources[i])
		if !apierrors.IsNotFound(err) {
			t.Errorf("Test_ClearResources_NilFeSpec the resource is not deleted, name=%s, err=%v", name, err)
		}
	}
}

// A cluster that never had fe returns early on the nil status, before reaching the spec.
func Test_ClearResources_NoFEStatus(t *testing.T) {
	fc, _ := newFeController(t)
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-no-fe",
			Namespace: "default",
		},
	}

	cleared, err := fc.ClearResources(context.Background(), cluster)
	if err != nil {
		t.Errorf("Test_ClearResources_NoFEStatus failed, err=%s", err.Error())
	}
	if !cleared {
		t.Error("Test_ClearResources_NoFEStatus expected resources to be cleared")
	}
}
