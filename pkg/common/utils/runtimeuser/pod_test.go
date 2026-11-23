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

package runtimeuser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	dv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func fixture(t *testing.T, uid int64) (*dv1.DorisCluster, *appsv1.StatefulSet, client.Client) {
	t.Helper()
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	_ = dv1.AddToScheme(s)
	owner := &dv1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "doris", Namespace: "test", UID: "owner"}}
	st := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "doris-be", Namespace: "test"}, Spec: appsv1.StatefulSetSpec{
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}}}},
		Template:             corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}}}}, Containers: []corev1.Container{{Name: "be", Image: "be:new", Command: []string{"/opt/apache-doris/be_entrypoint.sh"}, Args: []string{"$(ENV_FE_ADDR)"}, SecurityContext: &corev1.SecurityContext{RunAsUser: ptr.To(uid), RunAsGroup: ptr.To(uid)}, VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/opt/apache-doris/be/storage"}}}}, InitContainers: []corev1.Container{{Name: "default-init", SecurityContext: &corev1.SecurityContext{Privileged: ptr.To(true)}}}}},
	}}
	c := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&corev1.Pod{}).WithObjects(owner).Build()
	return owner, st, c
}
func hasInit(st *appsv1.StatefulSet, name string) bool {
	for _, v := range st.Spec.Template.Spec.InitContainers {
		if v.Name == name {
			return true
		}
	}
	return false
}

func TestIdentityTransitions(t *testing.T) {
	for _, tc := range []struct {
		name             string
		old, new         int64
		existing, repair bool
	}{{"fresh nonroot", 0, 10001, false, false}, {"root to root", 0, 0, true, false}, {"root to nonroot", 0, 10001, true, true}, {"nonroot to nonroot", 10001, 10001, true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			owner, st, c := fixture(t, tc.new)
			if tc.existing {
				old := st.DeepCopy()
				old.Spec.Template.Spec.Containers[0].Image = "be:old"
				raw, _ := json.Marshal(identity{tc.old, tc.old})
				old.Spec.Template.Annotations = map[string]string{userAnnotation: string(raw)}
				if err := c.Create(context.Background(), old); err != nil {
					t.Fatal(err)
				}
			}
			if err := Prepare(context.Background(), c, owner, st, false); err != nil {
				t.Fatal(err)
			}
			if hasInit(st, "migrate-volume-permissions") != tc.repair {
				t.Fatalf("unexpected repair init: %+v", st.Spec.Template.Spec.InitContainers)
			}
			if hasInit(st, "default-init") {
				t.Fatal("default-off still injects privileged system init")
			}
			for _, v := range st.Spec.Template.Spec.InitContainers {
				if v.SecurityContext != nil && v.SecurityContext.Privileged != nil && *v.SecurityContext.Privileged {
					t.Fatal("privileged init while helper disabled")
				}
			}
			if tc.repair {
				init := st.Spec.Template.Spec.InitContainers[0]
				if init.Name != "migrate-volume-permissions" || *init.SecurityContext.RunAsUser != 0 || init.Command[5] != "10001" || init.Command[7] != "/opt/apache-doris/be/storage" {
					t.Fatalf("bad repair arguments/order: %+v", init)
				}
			}
			if tc.new != 0 && (st.Spec.Template.Spec.SecurityContext.FSGroup == nil || *st.Spec.Template.Spec.SecurityContext.FSGroup != tc.new) {
				t.Fatal("missing fsGroup for a dedicated claim template")
			}
			if *st.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser != tc.new {
				t.Fatal("wrong user")
			}
		})
	}
}
func TestMigrationPersistsUntilNextImage(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	old := st.DeepCopy()
	raw, _ := json.Marshal(identity{10001, 10001})
	old.Spec.Template.Annotations = map[string]string{userAnnotation: string(raw), migrationAnnotation: "be:new"}
	_ = c.Create(context.Background(), old)
	if err := Prepare(context.Background(), c, owner, st, false); err != nil {
		t.Fatal(err)
	}
	if !hasInit(st, "migrate-volume-permissions") {
		t.Fatal("retry lost unfinished migration")
	}
	next := old.DeepCopy()
	next.Spec.Template.Annotations = nil
	next.Spec.Template.Spec.Containers[0].Image = "be:next"
	if err := Prepare(context.Background(), c, owner, next, false); err != nil {
		t.Fatal(err)
	}
	if hasInit(next, "migrate-volume-permissions") {
		t.Fatal("same-UID next image repeats full migration")
	}
}
func TestNonRootIdentityChangeBlocked(t *testing.T) {
	owner, st, c := fixture(t, 20001)
	old := st.DeepCopy()
	raw, _ := json.Marshal(identity{10001, 10001})
	old.Spec.Template.Annotations = map[string]string{userAnnotation: string(raw)}
	_ = c.Create(context.Background(), old)
	if err := Prepare(context.Background(), c, owner, st, false); err == nil || !strings.Contains(err.Error(), "explicit storage migration") {
		t.Fatalf("got %v", err)
	}
}
func TestProbeUsesImageIdentity(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	st.Spec.Template.Spec.Containers[0].SecurityContext = nil
	if err := Prepare(context.Background(), c, owner, st, false); err == nil {
		t.Fatal("unresolved identity accepted")
	}
	var pods corev1.PodList
	_ = c.List(context.Background(), &pods)
	if len(pods.Items) != 1 {
		t.Fatalf("probes=%d", len(pods.Items))
	}
	p := &pods.Items[0]
	if len(p.Spec.Volumes) != 0 || *p.Spec.AutomountServiceAccountToken {
		t.Fatal("probe mounts credentials/storage")
	}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "identity", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Message: `{"uid":10001,"gid":10001}`}}}}
	if err := c.Status().Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(context.Background(), c, owner, st, false); err != nil {
		t.Fatal(err)
	}
	if *st.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser != 10001 {
		t.Fatal("image user lost")
	}
}
func TestHelperNeverElevatesMain(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	if err := Prepare(context.Background(), c, owner, st, true); err != nil {
		t.Fatal(err)
	}
	if err := AttachHelper(st, true); err != nil {
		t.Fatal(err)
	}
	if *st.Spec.Template.Spec.Containers[0].SecurityContext.Privileged {
		t.Fatal("main privileged")
	}
	if !hasInit(st, "doris-cgroup-helper") {
		t.Fatal("missing helper")
	}
	last := st.Spec.Template.Spec.InitContainers[len(st.Spec.Template.Spec.InitContainers)-1]
	if last.RestartPolicy == nil || *last.RestartPolicy != corev1.ContainerRestartPolicyAlways || !*last.SecurityContext.Privileged {
		t.Fatal("helper must be a restartable privileged sidecar")
	}
	if !*st.Spec.Template.Spec.ShareProcessNamespace {
		t.Fatal("missing PID namespace")
	}
	if !hasInit(st, "default-init") {
		t.Fatal("opt-in lost the previous root system initialization")
	}
	for _, init := range st.Spec.Template.Spec.InitContainers {
		if init.Name == "default-init" && (*init.SecurityContext.RunAsUser != 0 || *init.SecurityContext.RunAsNonRoot) {
			t.Fatal("system initializer inherited the non-root Pod identity")
		}
	}
}

func TestHelperPreservesSystemInitOptOut(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	// The existing resource builder omits default-init and sets this environment
	// value when skipDefaultSystemInit is explicitly enabled.
	st.Spec.Template.Spec.InitContainers = nil
	setEnv(&st.Spec.Template.Spec.Containers[0], "SKIP_CHECK_ULIMIT", "true")
	if err := Prepare(context.Background(), c, owner, st, true); err != nil {
		t.Fatal(err)
	}
	if err := AttachHelper(st, true); err != nil {
		t.Fatal(err)
	}
	if hasInit(st, "default-init") || !hasInit(st, "doris-cgroup-helper") {
		t.Fatal("system-init opt-out must not disable CPU delegation")
	}
	for _, env := range st.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "SKIP_CHECK_ULIMIT" {
			if env.Value != "true" {
				t.Fatal("lost the explicit system-init opt-out")
			}
			return
		}
	}
	t.Fatal("lost the explicit system-init opt-out")
}

func TestProbeRejectsIncompleteIdentity(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	st.Spec.Template.Spec.Containers[0].SecurityContext = nil
	_ = Prepare(context.Background(), c, owner, st, false)
	var pods corev1.PodList
	if err := c.List(context.Background(), &pods); err != nil {
		t.Fatal(err)
	}
	p := &pods.Items[0]
	for _, message := range []string{`{}`, `{"uid":10001}`, `{"uid":-1,"gid":10001}`, `not-json`} {
		p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "identity", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Message: message}}}}
		if err := c.Status().Update(context.Background(), p); err != nil {
			t.Fatal(err)
		}
		if err := Prepare(context.Background(), c, owner, st, false); err == nil {
			t.Fatalf("accepted %s", message)
		}
	}
}

func TestChangingImageDuringMigrationIsBlocked(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	old := st.DeepCopy()
	old.Spec.Replicas = ptr.To(int32(3))
	old.Spec.Template.Annotations = map[string]string{userAnnotation: `{"uid":10001,"gid":10001}`, migrationAnnotation: "be:new"}
	old.Status.UpdatedReplicas = 1
	old.Status.ReadyReplicas = 1
	if err := c.Create(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	st.Spec.Template.Spec.Containers[0].Image = "be:next"
	if err := Prepare(context.Background(), c, owner, st, false); err == nil || !strings.Contains(err.Error(), "finish the existing PVC migration") {
		t.Fatalf("got %v", err)
	}
}

func TestReadOnlyClaimIsNotMigrated(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	st.Spec.VolumeClaimTemplates = nil
	st.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "readonly", ReadOnly: true}}}}
	old := st.DeepCopy()
	old.Spec.Template.Annotations = map[string]string{userAnnotation: `{"uid":0,"gid":0}`}
	if err := c.Create(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(context.Background(), c, owner, st, false); err != nil {
		t.Fatal(err)
	}
	if hasInit(st, "migrate-volume-permissions") || hasInit(st, "check-volume-permissions") {
		t.Fatal("attempted to mutate read-only claim")
	}
}

func TestSharedClaimNeverGetsAutomaticOwnershipChange(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	st.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "shared", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "shared"}}}}
	st.Spec.Template.Spec.Containers[0].VolumeMounts = append(st.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "shared", MountPath: "/shared"})
	old := st.DeepCopy()
	old.Spec.Template.Annotations = map[string]string{userAnnotation: `{"uid":0,"gid":0}`}
	if err := c.Create(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(context.Background(), c, owner, st, false); err != nil {
		t.Fatal(err)
	}
	if st.Spec.Template.Spec.SecurityContext.FSGroup != nil {
		t.Fatal("automatic fsGroup would mutate shared claims")
	}
	for _, init := range st.Spec.Template.Spec.InitContainers {
		if init.Name == "migrate-volume-permissions" && (len(init.VolumeMounts) != 1 || init.VolumeMounts[0].Name != "data") {
			t.Fatalf("repair mounts shared data: %+v", init.VolumeMounts)
		}
		if init.Name == "check-volume-permissions" && (len(init.VolumeMounts) != 2 || init.Command[4] != "verify-deep") {
			t.Fatal("shared data not checked")
		}
	}
}

func TestInvalidRecordedIdentityCannotTriggerRepair(t *testing.T) {
	owner, st, c := fixture(t, 10001)
	old := st.DeepCopy()
	old.Spec.Template.Annotations = map[string]string{userAnnotation: `{}`}
	if err := c.Create(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(context.Background(), c, owner, st, false); err == nil || !strings.Contains(err.Error(), "invalid recorded runtime identity") {
		t.Fatalf("got %v", err)
	}
}

func TestCompletedMigrationClearsThroughStatefulSetApply(t *testing.T) {
	owner, desired, c := fixture(t, 10001)
	old := desired.DeepCopy()
	old.Spec.Template.Spec.Containers[0].Image = "be:old"
	old.Spec.Template.Annotations = map[string]string{userAnnotation: `{"uid":10001,"gid":10001}`, migrationAnnotation: "be:old"}
	old.Spec.Replicas = ptr.To(int32(1))
	old.Status.ReadyReplicas = 1
	old.Status.UpdatedReplicas = 1
	old.Status.CurrentRevision = "done"
	old.Status.UpdateRevision = "done"
	if err := c.Create(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		next := desired.DeepCopy()
		if err := Prepare(context.Background(), c, owner, next, false); err != nil {
			t.Fatal(err)
		}
		if hasInit(next, "migrate-volume-permissions") {
			t.Fatalf("repeat migration on reconcile %d", attempt)
		}
		err := k8s.ApplyStatefulSet(context.Background(), c, next, func(n, o *appsv1.StatefulSet) bool { return resource.StatefulSetDeepEqual(n, o, false) })
		if err != nil {
			t.Fatal(err)
		}
		var got appsv1.StatefulSet
		if err := c.Get(context.Background(), client.ObjectKeyFromObject(next), &got); err != nil {
			t.Fatal(err)
		}
		if got.Spec.Template.Annotations[migrationAnnotation] != "" {
			t.Fatal("metadata merge restored completed migration")
		}
	}
}
