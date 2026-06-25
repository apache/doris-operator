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

package computegroups

import (
	"context"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileStatefulsetRejectsStorageTemplateChange(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme failed: %v", err)
	}

	ddc := newTestDDC()
	cg := newTestCG("cg1")
	existing := newTestStatefulSet(ddc.Namespace, ddc.GetCGStatefulsetName(cg), "100Gi")
	desired := newTestStatefulSet(ddc.Namespace, ddc.GetCGStatefulsetName(cg), "200Gi")
	dcgs := &DisaggregatedComputeGroupsController{}
	dcgs.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	event, err := dcgs.reconcileStatefulset(context.Background(), desired, ddc, cg)
	if err == nil {
		t.Fatal("reconcileStatefulset expected immutable storage template error")
	}
	if event == nil {
		t.Fatal("reconcileStatefulset expected event")
	}
	if event.Reason != sc.CGStorageTemplateImmutable {
		t.Fatalf("event reason = %s, want %s", event.Reason, sc.CGStorageTemplateImmutable)
	}
}

func newTestDDC() *dv1.DorisDisaggregatedCluster {
	return &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris",
			Namespace: "default",
		},
	}
}

func newTestCG(uniqueID string) *dv1.ComputeGroup {
	return &dv1.ComputeGroup{
		UniqueId: uniqueID,
	}
}

func TestVolumeClaimTemplatesEqualIgnoresDefaultVolumeMode(t *testing.T) {
	withDefault := newTestStatefulSet("default", "doris-cg1", "100Gi").Spec.VolumeClaimTemplates
	withoutDefault := newTestStatefulSet("default", "doris-cg1", "100Gi").Spec.VolumeClaimTemplates
	volumeMode := corev1.PersistentVolumeFilesystem
	withDefault[0].Spec.VolumeMode = &volumeMode

	if !volumeClaimTemplatesEqual(withoutDefault, withDefault) {
		t.Fatal("volumeClaimTemplatesEqual should ignore default filesystem volume mode")
	}
}

func newTestStatefulSet(namespace, name, storage string) *appv1.StatefulSet {
	return &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "be-storage0",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(storage),
						},
					},
				},
			}},
		},
	}
}
