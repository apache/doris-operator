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
	"fmt"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	operatorresource "github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestReconcileStatefulsetRejectsStorageTemplateChange(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme failed: %v", err)
	}

	ddc := newTestDDC()
	cg := newTestCG("cg1")
	existing := newTestStatefulSet(ddc.Namespace, ddc.GetCGStatefulsetName(cg), "100Gi")
	desired := newTestStatefulSet(ddc.Namespace, ddc.GetCGStatefulsetName(cg), "100Gi")
	desired.Spec.VolumeClaimTemplates[0].Spec.StorageClassName = stringPointer("different-class")
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

func TestVolumeClaimTemplatesEqualIgnoresRuntimeFields(t *testing.T) {
	desired := newTestStatefulSet("default", "doris-cg1", "100Gi").Spec.VolumeClaimTemplates
	existing := newTestStatefulSet("default", "doris-cg1", "100Gi").Spec.VolumeClaimTemplates

	existing[0].TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
	}
	existing[0].Status.Phase = corev1.ClaimPending

	if !volumeClaimTemplatesEqual(desired, existing) {
		t.Fatal("volumeClaimTemplatesEqual should ignore runtime-only pvc template fields")
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
			cg.Replicas = operatorresource.GetInt32Pointer(1)
			ddc.Spec.ComputeGroups = []dv1.ComputeGroup{*cg}
			replicas := int32(1)
			st := newTestStatefulSet(ddc.Namespace, ddc.GetCGStatefulsetName(cg), "1Gi")
			st.Spec.Replicas = &replicas
			st.Spec.VolumeClaimTemplates[0].Name = "be-log"
			cacheTemplate := *st.Spec.VolumeClaimTemplates[0].DeepCopy()
			cacheTemplate.Name = "be-file-cache"
			st.Spec.VolumeClaimTemplates = append(st.Spec.VolumeClaimTemplates, cacheTemplate)
			st.Annotations = map[string]string{"doris.apache.org/runtime-config-storage": tc.recorded}
			dcgs := &DisaggregatedComputeGroupsController{}
			labels := dcgs.newCGPodsSelector(ddc.Name, cg.UniqueId)
			objects := []client.Object{st, ddc}
			for _, volume := range []string{"be-log", "be-file-cache"} {
				for _, ordinal := range []string{"0", "1"} {
					objects = append(objects, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: volume + "-" + st.Name + "-" + ordinal, Namespace: ddc.Namespace, Labels: labels, Annotations: map[string]string{operatorresource.PVCManagerAnnotationApache: "operator"}}})
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

func TestCleanupProtectsScaleTransitions(t *testing.T) {
	for _, tc := range []struct {
		name            string
		desired, actual int32
		staleDesired    bool
		pod             bool
		managed         bool
		keep            bool
		wantError       bool
	}{
		{name: "scale up with stale StatefulSet", desired: 3, actual: 2, managed: true, keep: true},
		{name: "fresh CR overrides stale reconcile input", desired: 3, actual: 2, staleDesired: true, managed: true, keep: true},
		{name: "graceful scale down still owns ordinal", desired: 2, actual: 3, managed: true, keep: true},
		{name: "scale down waits for terminating Pod", desired: 2, actual: 2, pod: true, managed: true, keep: true, wantError: true},
		{name: "scale down completes finalizer", desired: 2, actual: 2, managed: true},
		{name: "user managed claim is untouched", desired: 2, actual: 2, keep: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appv1.AddToScheme, corev1.AddToScheme, dv1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			ddc := newTestDDC()
			cg := newTestCG("cg1")
			cg.Replicas = &tc.desired
			ddc.Spec.ComputeGroups = []dv1.ComputeGroup{*cg}
			st := newTestStatefulSet(ddc.Namespace, ddc.GetCGStatefulsetName(cg), "20Gi")
			st.Spec.Replicas = &tc.actual
			dcgs := &DisaggregatedComputeGroupsController{}
			dcgs.K8srecorder = record.NewFakeRecorder(10)
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
				Name: "be-storage0-" + st.Name + "-2", Namespace: ddc.Namespace,
				UID: "original-claim", Labels: dcgs.newCGPodsSelector(ddc.Name, cg.UniqueId),
				Finalizers: []string{operatorresource.PVCFinalizerApache},
			}}
			if tc.managed {
				pvc.Annotations = map[string]string{operatorresource.PVCManagerAnnotationApache: "operator"}
			}
			objects := []client.Object{ddc, st, pvc}
			if tc.pod {
				now := metav1.Now()
				objects = append(objects, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: st.Name + "-2", Namespace: ddc.Namespace, DeletionTimestamp: &now, Finalizers: []string{"test/pod"}},
					Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}}})
			}
			live := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			dcgs.K8sclient = live
			dcgs.APIReader = live
			input := ddc.DeepCopy()
			if tc.staleDesired {
				input.Spec.ComputeGroups[0].Replicas = operatorresource.GetInt32Pointer(2)
			}
			err := dcgs.ClearStatefulsetUnusedPVCs(ctx, input, dv1.ComputeGroupStatus{UniqueId: cg.UniqueId})
			if (err != nil) != tc.wantError {
				t.Fatalf("cleanup error = %v", err)
			}
			var got corev1.PersistentVolumeClaim
			err = live.Get(ctx, client.ObjectKeyFromObject(pvc), &got)
			if tc.keep && (err != nil || !got.DeletionTimestamp.IsZero()) {
				t.Fatalf("required PVC was deleted: %v, %v", err, got.DeletionTimestamp)
			}
			if !tc.keep && !apierrors.IsNotFound(err) {
				t.Fatalf("unused claim should be fully deleted, got %v, finalizers %v", err, got.Finalizers)
			}
		})
	}
}

func stringPointer(value string) *string { return &value }

func TestVolumeClaimTemplatesAllowOnlyOperatorCapacityGrowth(t *testing.T) {
	base := newTestStatefulSet("default", "cg1", "20Gi").Spec.VolumeClaimTemplates[0]
	base.Annotations = map[string]string{
		operatorresource.PVCManagerAnnotationApache: "operator",
		dorisv1.ComponentResourceHash:               "old-spec-hash",
	}
	for _, tc := range []struct {
		name    string
		mutate  func(*corev1.PersistentVolumeClaim)
		allowed bool
	}{
		{name: "capacity growth with regenerated hash", allowed: true, mutate: func(p *corev1.PersistentVolumeClaim) {
			p.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("25Gi")
			p.Annotations[dorisv1.ComponentResourceHash] = "new-spec-hash"
		}},
		{name: "capacity unchanged", allowed: true, mutate: func(p *corev1.PersistentVolumeClaim) {}},
		{name: "capacity shrink", mutate: func(p *corev1.PersistentVolumeClaim) {
			p.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("19Gi")
		}},
		{name: "capacity removed", mutate: func(p *corev1.PersistentVolumeClaim) { delete(p.Spec.Resources.Requests, corev1.ResourceStorage) }},
		{name: "storage class changed alongside growth", mutate: func(p *corev1.PersistentVolumeClaim) {
			p.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("25Gi")
			p.Spec.StorageClassName = stringPointer("another-class")
		}},
		{name: "access mode changed", mutate: func(p *corev1.PersistentVolumeClaim) {
			p.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		}},
		{name: "volume mode changed", mutate: func(p *corev1.PersistentVolumeClaim) { mode := corev1.PersistentVolumeBlock; p.Spec.VolumeMode = &mode }},
		{name: "name changed", mutate: func(p *corev1.PersistentVolumeClaim) { p.Name = "different-cache-root" }},
		{name: "user annotation changed", mutate: func(p *corev1.PersistentVolumeClaim) { p.Annotations["user/setting"] = "different" }},
		{name: "provisioner changed", mutate: func(p *corev1.PersistentVolumeClaim) {
			delete(p.Annotations, operatorresource.PVCManagerAnnotationApache)
		}},
		{name: "claim selector changed", mutate: func(p *corev1.PersistentVolumeClaim) {
			p.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"disk": "different"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			desired := base.DeepCopy()
			tc.mutate(desired)
			before := desired.DeepCopy()
			if got := volumeClaimTemplatesEqual([]corev1.PersistentVolumeClaim{*desired}, []corev1.PersistentVolumeClaim{base}); got != tc.allowed {
				t.Fatalf("allowed=%v, want %v", got, tc.allowed)
			}
			if !equality.Semantic.DeepEqual(before, desired) {
				t.Fatal("comparison mutated desired template")
			}
			if q := base.Spec.Resources.Requests[corev1.ResourceStorage]; q.Cmp(resource.MustParse("20Gi")) != 0 {
				t.Fatal("comparison mutated existing template")
			}
		})
	}
	unmanaged := *base.DeepCopy()
	unmanaged.Annotations = nil
	desired := unmanaged.DeepCopy()
	desired.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("25Gi")
	if volumeClaimTemplatesEqual([]corev1.PersistentVolumeClaim{*desired}, []corev1.PersistentVolumeClaim{unmanaged}) {
		t.Fatal("unmanaged template capacity change should be rejected")
	}
}

// Exercise the complete reconciliation order, including the immutable StatefulSet
// template and actual claim patches. A native StatefulSet controller may race
// ahead and provision a new ordinal using the original 20Gi template.
func TestComputeGroupExpandsPVCsThenScalesUp(t *testing.T) {
	for _, nativeCreatesClaim := range []bool{false, true} {
		t.Run(fmt.Sprintf("nativeCreatesClaim=%v", nativeCreatesClaim), func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appv1.AddToScheme, corev1.AddToScheme, dv1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			ddc := newTestDDC()
			cg := newTestCG("cg1")
			cg.Replicas = operatorresource.GetInt32Pointer(2)
			cg.LogNotStore = true
			cg.Image = "apache/doris:be-4.1.1"
			cg.PersistentVolumes = []dv1.PersistentVolume{{
				PVCProvisioner: dv1.PVCProvisionerOperator,
				MountPaths:     []string{"/opt/apache-doris/be/file_cache"},
				PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					StorageClassName: stringPointer("doris-disk"),
					Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")}},
				},
			}}
			c := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					if incoming, ok := obj.(*appv1.StatefulSet); ok {
						var current appv1.StatefulSet
						if err := c.Get(ctx, client.ObjectKeyFromObject(incoming), &current); err != nil {
							return err
						}
						if !equality.Semantic.DeepEqual(current.Spec.VolumeClaimTemplates, incoming.Spec.VolumeClaimTemplates) {
							return fmt.Errorf("StatefulSet volumeClaimTemplates is immutable")
						}
					}
					return c.Patch(ctx, obj, patch, opts...)
				},
			}).Build()
			dcgs := &DisaggregatedComputeGroupsController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
				K8sclient: c, APIReader: c, K8srecorder: record.NewFakeRecorder(30),
			}}
			sync := func() error { _, err := dcgs.computeGroupSync(ctx, ddc, cg); return err }
			if err := sync(); err != nil {
				t.Fatal(err)
			}
			var initial appv1.StatefulSet
			key := types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.GetCGStatefulsetName(cg)}
			if err := c.Get(ctx, key, &initial); err != nil {
				t.Fatal(err)
			}
			if len(initial.Spec.VolumeClaimTemplates) != 1 {
				t.Fatalf("unexpected templates: %v", initial.Spec.VolumeClaimTemplates)
			}
			var claims corev1.PersistentVolumeClaimList
			if err := c.List(ctx, &claims, client.InNamespace(ddc.Namespace)); err != nil {
				t.Fatal(err)
			}
			originalUIDs := map[string]types.UID{}
			for i := range claims.Items {
				pvc := &claims.Items[i]
				pvc.UID = types.UID("original-" + pvc.Name)
				if err := c.Update(ctx, pvc); err != nil {
					t.Fatal(err)
				}
				originalUIDs[pvc.Name] = pvc.UID
			}
			cg.PersistentVolumes[0].Resources.Requests[corev1.ResourceStorage] = resource.MustParse("25Gi")
			if err := sync(); err != nil {
				t.Fatalf("20Gi -> 25Gi expansion failed: %v", err)
			}
			assertState := func(replicas int32) {
				t.Helper()
				var st appv1.StatefulSet
				if err := c.Get(ctx, key, &st); err != nil {
					t.Fatal(err)
				}
				if *st.Spec.Replicas != replicas {
					t.Fatalf("replicas=%d", *st.Spec.Replicas)
				}
				if !equality.Semantic.DeepEqual(initial.Spec.VolumeClaimTemplates, st.Spec.VolumeClaimTemplates) {
					t.Fatal("immutable template changed")
				}
				if !equality.Semantic.DeepEqual(initial.Spec.Template, st.Spec.Template) {
					t.Fatal("storage expansion changed the Pod template")
				}
				if err := c.List(ctx, &claims, client.InNamespace(ddc.Namespace)); err != nil {
					t.Fatal(err)
				}
				if len(claims.Items) != int(replicas) {
					t.Fatalf("claims=%d, expected=%d", len(claims.Items), replicas)
				}
				for _, pvc := range claims.Items {
					requested := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
					if requested.Cmp(resource.MustParse("25Gi")) != 0 {
						t.Fatalf("%s capacity=%s", pvc.Name, requested.String())
					}
					if uid, ok := originalUIDs[pvc.Name]; ok && uid != pvc.UID {
						t.Fatalf("claim %s was replaced", pvc.Name)
					}
					if !pvc.DeletionTimestamp.IsZero() {
						t.Fatalf("claim %s is deleting", pvc.Name)
					}
				}
			}
			assertState(2)
			if nativeCreatesClaim {
				pvc := operatorresource.BuildDisaggregatedPVC(initial.Spec.VolumeClaimTemplates[0], initial.Spec.Selector.MatchLabels, ddc.Namespace, key.Name, "2")
				if err := c.Create(ctx, &pvc); err != nil {
					t.Fatal(err)
				}
			}
			cg.Replicas = operatorresource.GetInt32Pointer(3)
			if err := sync(); err != nil {
				t.Fatalf("scale up after expansion failed: %v", err)
			}
			assertState(3)
			cg.PersistentVolumes[0].Resources.Requests[corev1.ResourceStorage] = resource.MustParse("24Gi")
			if err := sync(); err == nil {
				t.Fatal("shrinking an already expanded claim should fail even though it exceeds the original 20Gi template")
			}
			assertState(3)
		})
	}
}
