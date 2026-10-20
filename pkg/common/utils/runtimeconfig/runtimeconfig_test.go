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

package runtimeconfig

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func fakeClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = dv1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}
func owner() *v1.DorisCluster {
	return &v1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "test", UID: "cluster-uid"}}
}
func workload(kind string) *appsv1.StatefulSet {
	n := int32(2)
	return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "cluster-" + kind, Namespace: "test", UID: types.UID(kind + "-uid"), Generation: 3, OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(owner(), v1.GroupVersion.WithKind("DorisCluster"))}}, Spec: appsv1.StatefulSetSpec{Replicas: &n, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: kind, Image: "doris:test", Env: []corev1.EnvVar{{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}}}, VolumeMounts: []corev1.VolumeMount{{Name: "log", MountPath: "/opt/apache-doris/" + kind + "/log"}, {Name: "config", MountPath: "/etc/doris", ReadOnly: true}}}}, Volumes: []corev1.Volume{{Name: "log", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "existing"}}}, {Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "user-config"}}}}}}}}}
}

func TestPolicyDefaultsAndMigration(t *testing.T) {
	ctx := context.Background()
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "existing"}[legacy], func(t *testing.T) {
			obj := owner()
			objects := []client.Object{obj}
			if legacy {
				st := workload("fe")
				st.OwnerReferences[0].Controller = nil
				objects = append(objects, st)
			}
			c := fakeClient(objects...)
			if err := EnsurePolicy(ctx, c, obj); err != nil {
				t.Fatal(err)
			}
			want := Auto
			if legacy {
				want = Legacy
			}
			if obj.Annotations[PolicyAnnotation] != want {
				t.Fatalf("policy=%v", obj.Annotations)
			}
			// A later resource deletion/recreation must not change the recorded decision.
			if legacy {
				if err := c.Delete(ctx, workload("fe")); err != nil {
					t.Fatal(err)
				}
			}
			if err := EnsurePolicy(ctx, c, obj); err != nil {
				t.Fatal(err)
			}
			if obj.Annotations[PolicyAnnotation] != want {
				t.Fatal("policy changed")
			}
		})
	}
	obj := owner()
	obj.Spec.RuntimeConfigPersistence = &v1.RuntimeConfigPersistenceSpec{Enabled: true}
	st := workload("fe")
	*st.Spec.Replicas = 0
	c := fakeClient(obj, st)
	if err := EnsurePolicy(ctx, c, obj); err == nil {
		t.Fatal("accepted migration without backup")
	}
	approval := migration{ClusterUID: string(obj.UID), Workloads: map[string]migrationWorkload{st.Name: {UID: string(st.UID), Generation: st.Generation, Replicas: *st.Spec.Replicas}}}
	raw, _ := json.Marshal(approval)
	obj.Annotations = map[string]string{MigrationAnnotation: string(raw)}
	if err := EnsurePolicy(ctx, c, obj); err != nil {
		t.Fatal(err)
	}
	if obj.Annotations[PolicyAnnotation] != Enabled {
		t.Fatal("not enabled")
	}
	obj.Spec.RuntimeConfigPersistence.Enabled = false
	if err := EnsurePolicy(ctx, c, obj); err == nil {
		t.Fatal("accepted lossy disable")
	}
	approval.Workloads[st.Name] = migrationWorkload{UID: "stale", Generation: st.Generation, Replicas: 2}
	raw, _ = json.Marshal(approval)
	obj.Annotations[MigrationAnnotation] = string(raw)
	if err := validateMigration(obj, []appsv1.StatefulSet{*st}); err == nil {
		t.Fatal("accepted stale migration")
	}
}

func TestDDCPolicyAndExistingStatus(t *testing.T) {
	obj := &dv1.DorisDisaggregatedCluster{ObjectMeta: metav1.ObjectMeta{Name: "ddc", Namespace: "test"}}
	c := fakeClient(obj)
	if err := EnsurePolicy(context.Background(), c, obj); err != nil {
		t.Fatal(err)
	}
	if obj.Annotations[PolicyAnnotation] != Auto {
		t.Fatal(obj.Annotations)
	}
	old := owner()
	old.Status.FEStatus = &v1.ComponentStatus{}
	c = fakeClient(old)
	if err := EnsurePolicy(context.Background(), c, old); err != nil {
		t.Fatal(err)
	}
	if old.Annotations[PolicyAnnotation] != Legacy {
		t.Fatal("existing status was ignored")
	}
}

func TestPrepareComponentsAndKeepTemplates(t *testing.T) {
	for _, kind := range []string{"fe", "be", "compute", "metaservice"} {
		t.Run(kind, func(t *testing.T) {
			obj := owner()
			obj.Annotations = map[string]string{PolicyAnnotation: Auto}
			st := workload(kind)
			before := st.DeepCopy()
			if err := Prepare(context.Background(), fakeClient(), nil, obj, st, nil); err != nil {
				t.Fatal(err)
			}
			pod := st.Spec.Template.Spec
			if len(pod.InitContainers) != 1 || pod.InitContainers[0].Image != "doris:test" {
				t.Fatalf("init=%+v", pod.InitContainers)
			}
			if !reflect.DeepEqual(before.Spec.VolumeClaimTemplates, st.Spec.VolumeClaimTemplates) {
				t.Fatal("PVC templates changed")
			}
			main := pod.Containers[0]
			if main.VolumeMounts[len(main.VolumeMounts)-1].SubPathExpr != StoreDir+"/$(POD_NAME)" {
				t.Fatal(main.VolumeMounts)
			}
			if main.VolumeMounts[1].Name != configVolume || main.VolumeMounts[1].ReadOnly {
				t.Fatal("base config not staged")
			}
			for _, v := range pod.Volumes {
				if v.Name == "config" && v.ConfigMap.Name != "user-config" {
					t.Fatal("user ConfigMap changed")
				}
			}
			// Reconcile should produce the same template and selected storage.
			again := before.DeepCopy()
			if err := Prepare(context.Background(), fakeClient(st), nil, obj, again, nil); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(st.Spec, again.Spec) || !reflect.DeepEqual(st.Annotations, again.Annotations) {
				t.Fatal("non-idempotent prepare")
			}
		})
	}
}

func TestStorageSafetyAndCompatibility(t *testing.T) {
	ctx := context.Background()
	obj := owner()
	obj.Annotations = map[string]string{PolicyAnnotation: Auto}
	st := workload("compute")
	st.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = "/cache"
	cfg := map[string]interface{}{"file_cache_path": `[{"path":"/cache"}]`}
	original := st.Spec.Template.DeepCopy()
	if err := Prepare(ctx, fakeClient(), nil, obj, st, cfg); err != nil {
		t.Fatal(err)
	}
	if st.Annotations[StorageAnnotation] != "unavailable" || !reflect.DeepEqual(original, &st.Spec.Template) {
		t.Fatal("cache root used")
	}
	obj.Annotations[PolicyAnnotation] = Enabled
	if err := Prepare(ctx, fakeClient(), nil, obj, st, cfg); err == nil {
		t.Fatal("explicit enable accepted cache-only PVC")
	}
	obj.Annotations[PolicyAnnotation] = Legacy
	st = workload("be")
	original = st.Spec.Template.DeepCopy()
	if err := Prepare(ctx, fakeClient(), nil, obj, st, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, &st.Spec.Template) {
		t.Fatal("legacy changed")
	}
	obj.Annotations[PolicyAnnotation] = Auto
	if err := Prepare(ctx, fakeClient(st), nil, obj, st, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, &st.Spec.Template) {
		t.Fatal("existing unmanaged workload changed")
	}
	// A user-owned durable path is respected.
	st = workload("fe")
	if err := Prepare(ctx, fakeClient(), nil, obj, st, map[string]interface{}{"custom_config_dir": "/opt/apache-doris/fe/log/custom"}); err != nil {
		t.Fatal(err)
	}
	if len(st.Spec.Template.Spec.InitContainers) != 0 || st.Annotations[StorageAnnotation] != "external" {
		t.Fatal("explicit path replaced")
	}
	if err := Prepare(ctx, fakeClient(), nil, obj, workload("fe"), map[string]interface{}{"custom_config_dir": "/tmp/custom"}); err == nil {
		t.Fatal("ephemeral custom path accepted")
	}
	// VolumeClaimTemplates also represent durable storage before the Pod exists.
	st = workload("fe")
	st.Spec.Template.Spec.Volumes = st.Spec.Template.Spec.Volumes[1:]
	st.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "log"}}}
	if err := Prepare(ctx, fakeClient(), nil, obj, st, nil); err != nil {
		t.Fatal(err)
	}
	if len(st.Spec.Template.Spec.InitContainers) == 0 {
		t.Fatal("claim template not recognized")
	}
}

func TestPrepareShellPreservesOverrides(t *testing.T) {
	for _, kind := range []string{"fe", "be", "ms"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			store := filepath.Join(root, "store")
			src := filepath.Join(root, "source")
			out := filepath.Join(root, "out")
			home := filepath.Join(root, "home")
			for _, dir := range []string{store, src, out, filepath.Join(home, "conf")} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			base := kind + ".conf"
			if kind == "ms" {
				base = "doris_cloud.conf"
			}
			if err := os.WriteFile(filepath.Join(src, base), []byte("port=1234\n"), 0644); err != nil {
				t.Fatal(err)
			}
			script := strings.NewReplacer("/runtime-store", store, "/runtime-source", src, "/runtime-prepared", out).Replace(prepareScript)
			run := func(pod, replicas string) error {
				cmd := exec.Command("bash", "-ec", script)
				cmd.Env = append(os.Environ(), "POD_NAME="+pod, "RUNTIME_MIGRATED_REPLICAS="+replicas, "RUNTIME_COMPONENT="+kind, "RUNTIME_BASE="+base, "RUNTIME_HOME="+home)
				output, err := cmd.CombinedOutput()
				if err != nil {
					return &shellError{err, string(output)}
				}
				return nil
			}
			if err := run("cluster-0", "0"); err != nil {
				t.Fatal(err)
			}
			custom := filepath.Join(store, StoreDir, "cluster-0", kind+"_custom.conf")
			if err := os.WriteFile(custom, []byte("dynamic=value\n"), 0644); err != nil {
				t.Fatal(err)
			}
			// A ConfigMap/image restart cannot overwrite runtime modifications.
			if err := run("cluster-0", "0"); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(custom)
			if string(got) != "dynamic=value\n" {
				t.Fatal(string(got))
			}
			got, _ = os.ReadFile(filepath.Join(out, base))
			if !strings.Contains(string(got), RuntimeDir) {
				t.Fatal(string(got))
			}
			if err := run("cluster-0", "1"); err == nil {
				t.Fatal("missing migration marker accepted")
			}
			if err := os.WriteFile(filepath.Join(filepath.Dir(custom), ".migration-complete"), nil, 0644); err != nil {
				t.Fatal(err)
			}
			if err := run("cluster-0", "1"); err != nil {
				t.Fatal(err)
			}
			if err := run("cluster-1", "1"); err != nil {
				t.Fatal("scale-out blocked:", err)
			}
		})
	}
}

type shellError struct {
	err    error
	output string
}

func (e *shellError) Error() string { return e.err.Error() + ": " + e.output }

func TestMigrationPodIdentityAndStorageIdentity(t *testing.T) {
	obj := owner()
	st := workload("fe")
	*st.Spec.Replicas = 1
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "cluster-fe-0", Namespace: "test", UID: "pod-new", OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(st, appsv1.SchemeGroupVersion.WithKind("StatefulSet"))}}}
	approval := migration{ClusterUID: string(obj.UID), Workloads: map[string]migrationWorkload{st.Name: {UID: string(st.UID), Generation: st.Generation, Replicas: 1}}, PodUIDs: map[string]string{pod.Name: "pod-old"}}
	raw, _ := json.Marshal(approval)
	obj.Annotations = map[string]string{MigrationAnnotation: string(raw)}
	c := fakeClient(obj, st, pod)
	if err := validateMigrationPods(context.Background(), c, obj, []appsv1.StatefulSet{*st}); err == nil {
		t.Fatal("replacement Pod accepted")
	}
	approval.PodUIDs[pod.Name] = string(pod.UID)
	raw, _ = json.Marshal(approval)
	obj.Annotations[MigrationAnnotation] = string(raw)
	if err := validateMigrationPods(context.Background(), c, obj, []appsv1.StatefulSet{*st}); err != nil {
		t.Fatal(err)
	}
	obj.Annotations[PolicyAnnotation] = Auto
	st = workload("fe")
	if err := Prepare(context.Background(), fakeClient(), nil, obj, st, nil); err != nil {
		t.Fatal(err)
	}
	desired := workload("fe")
	desired.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "different-disk"
	if err := Prepare(context.Background(), fakeClient(st), nil, obj, desired, nil); err == nil {
		t.Fatal("PVC replacement silently lost configuration")
	}
}

func TestShellImageDefaultsAndConfigMapSeed(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, "store")
	src := filepath.Join(root, "source")
	out := filepath.Join(root, "out")
	home := filepath.Join(root, "home")
	for _, dir := range []string{store, out, filepath.Join(home, "conf")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, "conf", "be.conf"), []byte("image_default = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	script := strings.NewReplacer("/runtime-store", store, "/runtime-source", src, "/runtime-prepared", out).Replace(prepareScript)
	run := func() {
		t.Helper()
		cmd := exec.Command("bash", "-ec", script)
		cmd.Env = append(os.Environ(), "POD_NAME=cluster-0", "RUNTIME_MIGRATED_REPLICAS=0", "RUNTIME_COMPONENT=be", "RUNTIME_BASE=be.conf", "RUNTIME_HOME="+home)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, output)
		}
	}
	run()
	base, _ := os.ReadFile(filepath.Join(out, "be.conf"))
	if !strings.Contains(string(base), "image_default = true") {
		t.Fatal("image defaults lost")
	}
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "be_custom.conf"), []byte("seed=initial\n"), 0644); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(store, StoreDir, "cluster-0", "be_custom.conf")
	if err := os.Remove(custom); err != nil {
		t.Fatal(err)
	}
	run()
	value, _ := os.ReadFile(custom)
	if string(value) != "seed=initial\n" {
		t.Fatal("ConfigMap seed lost")
	}
	if _, err := os.Stat(filepath.Join(out, "be_custom.conf")); !os.IsNotExist(err) {
		t.Fatal("mutable file exposed through source ConfigMap")
	}
	if err := os.WriteFile(custom, []byte("seed=changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run()
	value, _ = os.ReadFile(custom)
	if string(value) != "seed=changed\n" {
		t.Fatal("runtime override replaced by seed")
	}
}
