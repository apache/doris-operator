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

package sub_controller

import (
	"context"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFinishPVCDeletion(t *testing.T) {
	for _, tc := range []struct {
		name                                                       string
		pod, scheduled, foreignFinalizer, userManaged, notDeleting bool
		keepOurFinalizer, wantError                                bool
	}{
		{name: "deleting idle claim is released"},
		{name: "pending unscheduled Pod can recover", pod: true},
		{name: "scheduled Pod stays protected", pod: true, scheduled: true, keepOurFinalizer: true, wantError: true},
		{name: "other finalizers are preserved", foreignFinalizer: true},
		{name: "user managed claim stays protected", userManaged: true, keepOurFinalizer: true},
		{name: "live claim stays protected", notDeleting: true, keepOurFinalizer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			now := metav1.Now()
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
				Name: "cache-cg1-2", Namespace: "default", UID: "claim-uid", DeletionTimestamp: &now,
				Annotations: map[string]string{resource.PVCManagerAnnotationApache: "operator"},
				Finalizers:  []string{resource.PVCFinalizerApache},
			}}
			if tc.notDeleting {
				pvc.DeletionTimestamp = nil
			}
			if tc.userManaged {
				pvc.Annotations = nil
			}
			if tc.foreignFinalizer {
				pvc.Finalizers = append(pvc.Finalizers, "test/foreign")
			}
			objects := []client.Object{pvc}
			if tc.pod {
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "cg1-2", Namespace: pvc.Namespace},
					Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}}}
				if tc.scheduled {
					pod.Spec.NodeName = "node"
				}
				objects = append(objects, pod)
			}
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			d := &DisaggregatedSubDefaultController{K8sclient: c, APIReader: c}
			err := d.FinishPVCDeletion(ctx, pvc)
			if (err != nil) != tc.wantError {
				t.Fatalf("finish error: %v", err)
			}
			var got corev1.PersistentVolumeClaim
			err = c.Get(ctx, client.ObjectKeyFromObject(pvc), &got)
			if tc.keepOurFinalizer {
				if err != nil || len(got.Finalizers) != 1 || got.Finalizers[0] != resource.PVCFinalizerApache {
					t.Fatalf("claim lost protection: %v, %v", err, got.Finalizers)
				}
			} else if tc.foreignFinalizer {
				if err != nil || len(got.Finalizers) != 1 || got.Finalizers[0] != "test/foreign" {
					t.Fatalf("foreign finalizer was changed: %v, %v", err, got.Finalizers)
				}
			} else if !apierrors.IsNotFound(err) {
				t.Fatalf("deleting claim not released: %v", err)
			}
		})
	}
}

func TestReconcilePVCRecoversTerminatingClaim(t *testing.T) {
	ctx := context.Background()
	replicas := int32(1)
	cg := &dv1.ComputeGroup{CommonSpec: dv1.CommonSpec{
		Replicas: &replicas,
		PersistentVolumes: []dv1.PersistentVolume{{
			MountPaths:     []string{"/opt/apache-doris/be/file_cache"},
			PVCProvisioner: dv1.PVCProvisionerOperator,
		}},
	}}
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &DisaggregatedSubDefaultController{K8sclient: c, APIReader: c, K8srecorder: record.NewFakeRecorder(10)}
	_, _, templates := d.BuildVolumesVolumeMountsAndPVCs(nil, dv1.DisaggregatedBE, &cg.CommonSpec)
	if len(templates) != 1 {
		t.Fatalf("unexpected templates: %v", templates)
	}
	labels := map[string]string{"app": "cg1"}
	old := resource.BuildDisaggregatedPVC(templates[0], labels, "default", "cg1", "0")
	old.UID = "deleted-claim"
	if err := c.Create(ctx, &old); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, &old); err != nil {
		t.Fatal(err)
	}
	sts := &appv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "cg1", Namespace: "default"},
		Spec: appv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: labels}}}
	ddc := &dv1.DorisDisaggregatedCluster{ObjectMeta: metav1.ObjectMeta{Name: "ddc", Namespace: "default"}}
	if _, err := d.ReconcilePVC(ctx, ddc, nil, dv1.DisaggregatedBE, sts, cg); err == nil {
		t.Fatal("expected retry while deleting old claim")
	}
	if _, err := d.ReconcilePVC(ctx, ddc, nil, dv1.DisaggregatedBE, sts, cg); err != nil {
		t.Fatal(err)
	}
	var got corev1.PersistentVolumeClaim
	if err := c.Get(ctx, client.ObjectKeyFromObject(&old), &got); err != nil {
		t.Fatal(err)
	}
	if got.UID == old.UID || !got.DeletionTimestamp.IsZero() {
		t.Fatalf("old claim was not replaced: uid=%s deletion=%v", got.UID, got.DeletionTimestamp)
	}
}
