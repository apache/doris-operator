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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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

func TestScaleDownRetainsRuntimeConfigPVC(t *testing.T) {
	for _, tc := range []struct {
		name      string
		recorded  string
		keepLog   bool
		wantError bool
	}{
		{name: "legacy keeps existing cleanup"},
		{name: "selected log survives", recorded: `{"volume":"be-log","claim":"template:be-log","mountPath":"/opt/apache-doris/be/log"}`, keepLog: true},
		{name: "invalid storage blocks cleanup", recorded: `{`, keepLog: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appv1.AddToScheme, corev1.AddToScheme, dv1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			ddc := newTestDDC()
			cg := newTestCG("cg1")
			ddc.Spec.ComputeGroups = []dv1.ComputeGroup{*cg}
			replicas := int32(1)
			st := newTestStatefulSet(ddc.Namespace, ddc.GetCGStatefulsetName(cg), "1Gi")
			st.Spec.Replicas = &replicas
			st.Spec.VolumeClaimTemplates[0].Name = "be-log"
			st.Annotations = map[string]string{"doris.apache.org/runtime-config-storage": tc.recorded}
			dcgs := &DisaggregatedComputeGroupsController{}
			labels := dcgs.newCGPodsSelector(ddc.Name, cg.UniqueId)
			objects := []client.Object{st}
			for _, volume := range []string{"be-log", "be-file-cache"} {
				for _, ordinal := range []string{"0", "1"} {
					objects = append(objects, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: volume + "-" + st.Name + "-" + ordinal, Namespace: ddc.Namespace, Labels: labels}})
				}
			}
			dcgs.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			err := dcgs.ClearStatefulsetUnusedPVCs(context.Background(), ddc, dv1.ComputeGroupStatus{UniqueId: cg.UniqueId})
			if (err != nil) != tc.wantError {
				t.Fatalf("cleanup error = %v", err)
			}
			for _, volume := range []string{"be-log", "be-file-cache"} {
				for _, ordinal := range []string{"0", "1"} {
					var pvc corev1.PersistentVolumeClaim
					err := dcgs.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: ddc.Namespace, Name: volume + "-" + st.Name + "-" + ordinal}, &pvc)
					want := ordinal == "0" || tc.wantError || (volume == "be-log" && tc.keepLog)
					if want && err != nil {
						t.Fatalf("expected retained PVC %s/%s: %v", volume, ordinal, err)
					}
					if !want && !apierrors.IsNotFound(err) {
						t.Fatalf("expected deleted PVC %s/%s: %v", volume, ordinal, err)
					}
				}
			}
		})
	}
}
