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
package sub_controller

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/apache/doris-operator/api/disaggregated/v1"
	operatorresource "github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcilePVCExpandsHistoricalClaims(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme failed: %v", err)
	}

	replicas := int32(2)
	ddc := &v1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "doris", Namespace: "default"},
	}
	cg := &v1.ComputeGroup{
		UniqueId: "cg1",
		CommonSpec: v1.CommonSpec{
			Replicas:    &replicas,
			LogNotStore: true,
			PersistentVolumes: []v1.PersistentVolume{{
				MountPaths:     []string{"/data"},
				PVCProvisioner: v1.PVCProvisionerOperator,
				PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("20Gi"),
					}},
				},
			}},
		},
	}
	selector := map[string]string{"app": "doris-cg1"}
	sts := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "doris-cg1", Namespace: ddc.Namespace},
		Spec:       appv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: selector}},
	}

	objects := make([]runtime.Object, 0, 4)
	uids := make(map[string]types.UID, 4)
	for ordinal := 0; ordinal < 4; ordinal++ {
		name := operatorresource.BuildPVCName(sts.Name, fmt.Sprintf("%d", ordinal), "data")
		uid := types.UID(fmt.Sprintf("uid-%d", ordinal))
		uids[name] = uid
		objects = append(objects, &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  ddc.Namespace,
				Labels:     selector,
				UID:        uid,
				Finalizers: []string{"apache.doris.org/pvc-finalizer", "kubernetes.io/pvc-protection"},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				}},
			},
		})
	}

	controller := &DisaggregatedSubDefaultController{
		K8sclient:   fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build(),
		K8srecorder: record.NewFakeRecorder(20),
	}
	if _, err := controller.ReconcilePVC(context.Background(), ddc, map[string]interface{}{}, v1.DisaggregatedBE, sts, cg); err != nil {
		t.Fatalf("reconcile pvc failed: %v", err)
	}

	for name, uid := range uids {
		var pvc corev1.PersistentVolumeClaim
		if err := controller.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: ddc.Namespace, Name: name}, &pvc); err != nil {
			t.Fatalf("get pvc %s failed: %v", name, err)
		}
		if pvc.UID != uid {
			t.Fatalf("pvc %s uid = %s, want %s", name, pvc.UID, uid)
		}
		if got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("20Gi")) != 0 {
			t.Fatalf("pvc %s storage = %s, want 20Gi", name, got.String())
		}
		if len(pvc.Finalizers) != 1 || pvc.Finalizers[0] != "kubernetes.io/pvc-protection" {
			t.Fatalf("pvc %s finalizers = %v", name, pvc.Finalizers)
		}
	}
}

func TestDisaggregatedSubDefaultController_BuildVolumesVolumeMountsAndPVCs_empty_persistentVolume(t *testing.T) {
	confMap := map[string]interface{}{}
	commonSpec := v1.CommonSpec{}
	d := &DisaggregatedSubDefaultController{}
	fevs, fevms, fepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedFE, &commonSpec)
	bevs, bevms, bepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedBE, &commonSpec)
	msvs, msvms, mspvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedMS, &commonSpec)
	if len(fevs) != 2 || len(fevms) != 2 || len(fepvcs) != 0 {
		t.Errorf("build fe default volumes volumemounts and pvcs failed, the number is not right.")
	}
	if len(bevs) != 2 || len(bevms) != 2 || len(bepvcs) != 0 {
		t.Errorf("build be default volumes volumemounts and pvcs failed, the number is not right.")
	}
	if len(msvs) != 1 || len(msvms) != 1 || len(mspvcs) != 0 {
		t.Errorf("build ms default volumes volumemounts and pvcs failed, the number is not right.")
	}
}

func TestDisaggregatedSubDefaultController_BuildVolumesVolumeMountsAndPVCs_persistentVolume(t *testing.T) {
	confMap := map[string]interface{}{
		"file_cache_path": "[{\"path\":\"/path/to/file_cache\",\"total_size\":21474836480},{\"path\":\"/path/to/file_cache2\",\"total_size\":21474836480}]",
	}
	commonSpec := v1.CommonSpec{
		PersistentVolume: &v1.PersistentVolume{
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource.MustParse("200Gi"),
					},
				},
			},
		},
	}

	beCommonSpec := commonSpec.DeepCopy()
	beCommonSpec.PersistentVolume.MountPaths = []string{"/path/to/file_cache12"}
	d := &DisaggregatedSubDefaultController{}
	fevs, fevms, fepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedFE, &commonSpec)
	bevs, bevms, bepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedBE, beCommonSpec)
	msvs, msvms, mspvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedMS, &commonSpec)
	if len(fevs) != 2 || len(fevms) != 2 || len(fepvcs) != 2 {
		t.Errorf("build fe default volumes volumemounts and pvcs failed, the number is not right.")
	}
	if len(bevs) != 4 || len(bevms) != 4 || len(bepvcs) != 4 {
		t.Errorf("build be default volumes volumemounts and pvcs failed, the number is not right.")
	}
	if len(msvs) != 1 || len(msvms) != 1 || len(mspvcs) != 1 {
		t.Errorf("build ms default volumes volumemounts and pvcs failed, the number is not right.")
	}
}

func TestDisaggregatedSubDefaultController_PersistentVolumeArrayBuildVolumesVolumeMountsAndPVCs(t *testing.T) {
	confMap := map[string]interface{}{
		"file_cache_path": "[{\"path\":\"/path/to/file_cache\",\"total_size\":21474836480},{\"path\":\"/path/to/file_cache2\",\"total_size\":21474836480}]",
	}

	commonSpec := v1.CommonSpec{
		PersistentVolumes: []v1.PersistentVolume{{
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource.MustParse("200Gi"),
					},
				},
			},
		}, {
			MountPaths: []string{"/path/to/file_cache"},
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource.MustParse("500Gi"),
					},
				},
			}},
		},
	}
	d := &DisaggregatedSubDefaultController{}
	fevs, fevms, fepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedFE, &commonSpec)
	bevs, bevms, bepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedBE, &commonSpec)
	msvs, msvms, mspvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedMS, &commonSpec)
	if len(fevs) != 3 || len(fevms) != 3 || len(fepvcs) != 3 {
		t.Errorf("build fe default volumes volumemounts and pvcs failed, the number is not right.")
	}
	if len(bevs) != 3 || len(bevms) != 3 || len(bepvcs) != 3 {
		t.Errorf("build be default volumes volumemounts and pvcs failed, the number is not right.")
	}
	if len(msvs) != 2 || len(msvms) != 2 || len(mspvcs) != 2 {
		t.Errorf("build ms default volumes volumemounts and pvcs failed, the number is not right.")
	}
}
