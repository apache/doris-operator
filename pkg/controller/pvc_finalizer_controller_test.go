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
	"fmt"
	"testing"
	"time"

	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func deletingOperatorPVC(legacy bool) *corev1.PersistentVolumeClaim {
	finalizer, annotation := resource.PVCFinalizerApache, resource.PVCManagerAnnotationApache
	if legacy {
		finalizer, annotation = resource.PVCFinalizerLegacy, resource.PVCManagerAnnotationLegacy
	}
	now := metav1.Now()
	return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: "data-doris-fe-0", Namespace: "qa", UID: "claim-uid", DeletionTimestamp: &now,
		Annotations: map[string]string{annotation: "operator"}, Finalizers: []string{finalizer},
	}}
}

func TestPVCFinalizerOutlivesParentCR(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		for _, scenario := range []string{"idle", "unscheduled", "scheduled", "terminating pod", "other finalizer", "retained", "user managed", "mismatched marker"} {
			t.Run(fmt.Sprintf("legacy=%v/%s", legacy, scenario), func(t *testing.T) {
				ctx := context.Background()
				// Only core types are registered. Finalization must not depend on
				// any surviving DorisCluster, DDC, or StatefulSet.
				scheme := runtime.NewScheme()
				if err := corev1.AddToScheme(scheme); err != nil {
					t.Fatal(err)
				}
				pvc := deletingOperatorPVC(legacy)
				ownFinalizer := pvc.Finalizers[0]
				if scenario == "other finalizer" {
					pvc.Finalizers = append(pvc.Finalizers, "kubernetes.io/pvc-protection", "example.com/backup")
				}
				if scenario == "retained" {
					pvc.DeletionTimestamp = nil
				}
				if scenario == "user managed" {
					pvc.Annotations = nil
				}
				if scenario == "mismatched marker" {
					if legacy {
						pvc.Annotations = map[string]string{resource.PVCManagerAnnotationApache: "operator"}
					} else {
						pvc.Annotations = map[string]string{resource.PVCManagerAnnotationLegacy: "operator"}
					}
				}
				objects := []client.Object{pvc}
				var pod *corev1.Pod
				if scenario == "scheduled" || scenario == "terminating pod" || scenario == "unscheduled" {
					pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "doris-fe-0", Namespace: pvc.Namespace}, Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}},
					}}
					if scenario != "unscheduled" {
						pod.Spec.NodeName = "node-a"
					}
					if scenario == "terminating pod" {
						now := metav1.Now()
						pod.DeletionTimestamp = &now
						pod.Finalizers = []string{"example.com/drain"}
					}
					objects = append(objects, pod)
				}
				c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
				r := &PVCFinalizerReconciler{Client: c, APIReader: c}
				req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(pvc)}
				result, err := r.Reconcile(ctx, req)
				if err != nil {
					t.Fatal(err)
				}
				var got corev1.PersistentVolumeClaim
				err = c.Get(ctx, req.NamespacedName, &got)
				blocked := scenario == "scheduled" || scenario == "terminating pod"
				untouched := scenario == "retained" || scenario == "user managed" || scenario == "mismatched marker"
				if blocked || untouched {
					if err != nil || len(got.Finalizers) != 1 || got.Finalizers[0] != ownFinalizer {
						t.Fatalf("protection changed: %v, %v", err, got.Finalizers)
					}
					if blocked {
						if result.RequeueAfter != 5*time.Second {
							t.Fatalf("missing retry: %v", result)
						}
						if scenario == "terminating pod" {
							pod.Finalizers = nil
							if err := c.Update(ctx, pod); err != nil {
								t.Fatal(err)
							}
						} else if err := c.Delete(ctx, pod); err != nil {
							t.Fatal(err)
						}
						if _, err := r.Reconcile(ctx, req); err != nil {
							t.Fatal(err)
						}
						if err := c.Get(ctx, req.NamespacedName, &got); !apierrors.IsNotFound(err) {
							t.Fatalf("claim did not converge after Pod exited: %v", err)
						}
					} else if !result.IsZero() {
						t.Fatalf("untouched claim was queued: %v", result)
					}
				} else if scenario == "other finalizer" {
					if err != nil || len(got.Finalizers) != 2 || got.Finalizers[0] != "kubernetes.io/pvc-protection" || got.Finalizers[1] != "example.com/backup" {
						t.Fatalf("other finalizers changed: %v, %v", err, got.Finalizers)
					}
				} else if !apierrors.IsNotFound(err) {
					t.Fatalf("claim not released: %v", err)
				}
			})
		}
	}
}

type noPodCache struct{ client.Client }

func (c noPodCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if pods, ok := list.(*corev1.PodList); ok {
		pods.Items = nil
		return nil
	}
	return c.Client.List(ctx, list, opts...)
}

func TestPVCFinalizerUsesFreshPodReferencesAndFailsClosed(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	pvc := deletingOperatorPVC(false)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "consumer", Namespace: pvc.Namespace}, Spec: corev1.PodSpec{NodeName: "node", Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}}}
	live := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc, pod).Build()
	r := &PVCFinalizerReconciler{Client: noPodCache{live}, APIReader: live}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(pvc)}
	res, err := r.Reconcile(ctx, req)
	if err != nil || res.RequeueAfter != 5*time.Second {
		t.Fatalf("stale cache lost protection: %v %v", res, err)
	}
	denied := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).WithInterceptorFuncs(interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", fmt.Errorf("denied"))
		},
	}).Build()
	r.APIReader = denied
	if _, err := r.Reconcile(ctx, req); !apierrors.IsForbidden(err) {
		t.Fatalf("lookup failure ignored: %v", err)
	}
	var got corev1.PersistentVolumeClaim
	if err := live.Get(ctx, req.NamespacedName, &got); err != nil || len(got.Finalizers) != 1 {
		t.Fatalf("lookup failure removed protection: %v", err)
	}
}

func TestPVCFinalizerConflictRetriesWithoutChangingProtection(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	pvc := deletingOperatorPVC(false)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(_ context.Context, _ client.WithWatch, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
			return apierrors.NewConflict(schema.GroupResource{Resource: "persistentvolumeclaims"}, obj.GetName(), fmt.Errorf("changed"))
		},
	}).Build()
	r := &PVCFinalizerReconciler{Client: c, APIReader: c}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(pvc)}
	if _, err := r.Reconcile(ctx, req); !apierrors.IsConflict(err) {
		t.Fatalf("conflict was lost: %v", err)
	}
	var got corev1.PersistentVolumeClaim
	if err := c.Get(ctx, req.NamespacedName, &got); err != nil || len(got.Finalizers) != 1 {
		t.Fatalf("conflict removed protection: %v", err)
	}
}

func TestPVCFinalizerDoesNotPatchReplacementClaim(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	replacement := deletingOperatorPVC(true)
	replacement.UID = "replacement-uid"
	gets := 0
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(replacement).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if err := c.Get(ctx, key, obj, opts...); err != nil {
				return err
			}
			gets++
			if gets == 1 {
				obj.SetUID("old-uid")
			}
			return nil
		},
	}).Build()
	r := &PVCFinalizerReconciler{Client: c, APIReader: c}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(replacement)}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	var got corev1.PersistentVolumeClaim
	if err := c.Get(ctx, req.NamespacedName, &got); err != nil || len(got.Finalizers) != 1 {
		t.Fatalf("replacement claim changed: %v", err)
	}
}
