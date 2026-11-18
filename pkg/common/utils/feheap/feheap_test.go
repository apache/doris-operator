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

package feheap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/runtimeconfig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = dv1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func testOwner(ddc bool) client.Object {
	meta := metav1.ObjectMeta{Name: "cluster", Namespace: "test", UID: "cluster-uid"}
	if ddc {
		return &dv1.DorisDisaggregatedCluster{ObjectMeta: meta}
	}
	return &v1.DorisCluster{ObjectMeta: meta}
}

func TestPolicySurvivesWorkloadRecreation(t *testing.T) {
	for _, ddc := range []bool{false, true} {
		for _, scenario := range []string{"new", "status", "legacy-owner", "foreign-owner"} {
			t.Run(map[bool]string{false: "dcr/", true: "ddc/"}[ddc]+scenario, func(t *testing.T) {
				owner := testOwner(ddc)
				st := testWorkload("8Gi")
				objects := []client.Object{owner}
				want := Auto
				switch scenario {
				case "status":
					switch o := owner.(type) {
					case *v1.DorisCluster:
						o.Status.FEStatus = &v1.ComponentStatus{}
					case *dv1.DorisDisaggregatedCluster:
						o.Status.FEStatus = dv1.FEStatus{ClusterId: "existing"}
					}
					want = Legacy
				case "legacy-owner", "foreign-owner":
					uid := owner.GetUID()
					if scenario == "foreign-owner" {
						uid = "other"
					} else {
						want = Legacy
					}
					st.OwnerReferences = []metav1.OwnerReference{{UID: uid}}
					objects = append(objects, st)
				}
				c := testClient(objects...)
				if err := EnsurePolicy(context.Background(), c, owner); err != nil {
					t.Fatal(err)
				}
				if owner.GetAnnotations()[PolicyAnnotation] != want {
					t.Fatalf("annotations: %v", owner.GetAnnotations())
				}
				if scenario == "legacy-owner" {
					if err := c.Delete(context.Background(), st); err != nil {
						t.Fatal(err)
					}
				}
				// Read back the persisted decision, as after an operator restart.
				reloaded := testOwner(ddc)
				if err := c.Get(context.Background(), client.ObjectKeyFromObject(owner), reloaded); err != nil {
					t.Fatal(err)
				}
				if err := EnsurePolicy(context.Background(), c, reloaded); err != nil {
					t.Fatal(err)
				}
				if reloaded.GetAnnotations()[PolicyAnnotation] != want {
					t.Fatal("decision changed")
				}
			})
		}
	}
}

func TestExplicitPolicy(t *testing.T) {
	for _, policy := range []string{Auto, Legacy, "invalid"} {
		owner := testOwner(false)
		owner.SetAnnotations(map[string]string{PolicyAnnotation: policy})
		err := EnsurePolicy(context.Background(), testClient(owner), owner)
		if (err != nil) != (policy == "invalid") {
			t.Fatalf("policy %s: %v", policy, err)
		}
	}
}

func testWorkload(limit string) *appsv1.StatefulSet {
	main := corev1.Container{Name: "fe", Image: "doris:test", ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{"/opt/apache-doris/fe_entrypoint.sh"}, Args: []string{"$(FE_ADDR)"},
		Env:          []corev1.EnvVar{{Name: "DORIS_ROOT", Value: "/opt/apache-doris"}},
		VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/etc/doris", ReadOnly: true}}}
	if limit != "" {
		main.Resources.Limits = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse(limit)}
	}
	return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "cluster-fe", Namespace: "test"}, Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		Containers: []corev1.Container{main}, Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "user-config"}}}}},
	}}}}
}

func TestPrepareBoundaries(t *testing.T) {
	for _, scenario := range []string{"legacy", "unset-policy", "no-limit", "request-only", "direct-conf", "custom-root", "env-override"} {
		t.Run(scenario, func(t *testing.T) {
			owner := testOwner(false)
			owner.SetAnnotations(map[string]string{PolicyAnnotation: Auto})
			st := testWorkload("8Gi")
			switch scenario {
			case "legacy":
				owner.SetAnnotations(map[string]string{PolicyAnnotation: Legacy})
			case "unset-policy":
				owner.SetAnnotations(nil)
			case "no-limit", "request-only":
				st.Spec.Template.Spec.Containers[0].Resources.Limits = nil
				if scenario == "request-only" {
					st.Spec.Template.Spec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")}
				}
			case "direct-conf":
				st.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = feHome + "/conf"
			case "custom-root":
				st.Spec.Template.Spec.Containers[0].Env[0].Value = "/custom"
			case "env-override":
				st.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "JAVA_OPTS", Value: "custom"}, {Name: "JAVA_OPTS_FOR_JDK_17", Value: "custom17"}}
			}
			before := st.DeepCopy()
			if err := Prepare(owner, st); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, st) {
				t.Fatal("unchanged path mutated workload")
			}
		})
	}
}

func TestPrepareLimitsAndIdempotentRendering(t *testing.T) {
	for _, limit := range []string{"4Gi", "8Gi", "16Gi", "5000Mi"} {
		t.Run(limit, func(t *testing.T) {
			owner := testOwner(false)
			owner.SetAnnotations(map[string]string{PolicyAnnotation: Auto})
			st := testWorkload(limit)
			before := st.DeepCopy()
			if err := Prepare(owner, st); err != nil {
				t.Fatal(err)
			}
			pod := st.Spec.Template.Spec
			wantHeap := map[string]string{"4Gi": "3276", "8Gi": "6553", "16Gi": "13107", "5000Mi": "4000"}[limit]
			foundHeap := false
			for _, env := range pod.InitContainers[0].Env {
				if env.Name == "FE_HEAP_JDK8_MIB" {
					foundHeap = env.Value == wantHeap
				}
			}
			if !foundHeap {
				t.Fatalf("wrong JDK8 heap for %s", limit)
			}
			if len(pod.InitContainers) != 1 || pod.InitContainers[0].Image != pod.Containers[0].Image {
				t.Fatal("missing same-image initializer")
			}
			if !reflect.DeepEqual(pod.Containers[0].Command, before.Spec.Template.Spec.Containers[0].Command) || !reflect.DeepEqual(pod.Containers[0].Args, before.Spec.Template.Spec.Containers[0].Args) {
				t.Fatal("entrypoint changed")
			}
			if pod.Volumes[0].ConfigMap.Name != "user-config" || pod.InitContainers[0].VolumeMounts[1].Name != "config" {
				t.Fatal("source ConfigMap changed")
			}
			if err := Prepare(owner, before); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, st) {
				t.Fatal("re-render changed workload")
			}
		})
	}
}

func TestComposesWithRuntimePersistence(t *testing.T) {
	owner := testOwner(false)
	owner.SetAnnotations(map[string]string{PolicyAnnotation: Auto, runtimeconfig.PolicyAnnotation: runtimeconfig.Auto})
	st := testWorkload("8Gi")
	st.Spec.Template.Spec.Containers[0].VolumeMounts = append(st.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "log", MountPath: feHome + "/log"})
	st.Spec.Template.Spec.Volumes = append(st.Spec.Template.Spec.Volumes, corev1.Volume{Name: "log", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "logs"}}})
	if err := Prepare(owner, st); err != nil {
		t.Fatal(err)
	}
	if err := runtimeconfig.Prepare(context.Background(), testClient(), nil, owner, st, nil); err != nil {
		t.Fatal(err)
	}
	pod := st.Spec.Template.Spec
	if len(pod.InitContainers) != 2 || pod.InitContainers[0].Name != initName {
		t.Fatal("wrong preparation order")
	}
	found := false
	for _, mount := range pod.InitContainers[1].VolumeMounts {
		if mount.MountPath == "/runtime-source" && mount.Name == configVolume {
			found = true
		}
	}
	if !found {
		t.Fatal("runtime preparation does not consume heap output")
	}
	found = false
	for _, mount := range pod.Containers[0].VolumeMounts {
		if mount.MountPath == runtimeconfig.RuntimeDir {
			found = true
		}
	}
	if !found {
		t.Fatal("runtime persistence mount lost")
	}

	root := t.TempDir()
	home, source, heapDir := filepath.Join(root, "fe"), filepath.Join(root, "source"), filepath.Join(root, "heap")
	store, prepared := filepath.Join(root, "store"), filepath.Join(root, "runtime")
	for _, dir := range []string{home + "/conf", source, heapDir, store, prepared} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(home+"/conf/fe.conf", []byte(imageConfig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source+"/fe_custom.conf", []byte("test_override=true"), 0644); err != nil {
		t.Fatal(err)
	}
	replace := strings.NewReplacer("/runtime-source", heapDir, "/runtime-prepared", prepared, "/runtime-store", store)
	for i, init := range pod.InitContainers {
		cmd := exec.Command("bash", "-ec", replace.Replace(init.Command[2]))
		cmd.Env = os.Environ()
		for _, env := range init.Env {
			value := env.Value
			switch env.Name {
			case "FE_HEAP_HOME", "RUNTIME_HOME":
				value = home
			case "FE_HEAP_SOURCE":
				value = source
			case "FE_HEAP_OUTPUT":
				value = heapDir
			case "POD_NAME":
				value = "cluster-fe-0"
			}
			cmd.Env = append(cmd.Env, env.Name+"="+value)
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("initializer %d: %s: %v", i, output, err)
		}
	}
	got, err := os.ReadFile(prepared + "/fe.conf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "MaxRAMPercentage=80") || !strings.Contains(string(got), "custom_config_dir = /var/lib/doris/runtime-config") {
		t.Fatalf("composed config: %s", got)
	}
	override, err := os.ReadFile(store + "/.doris-runtime-config/cluster-fe-0/fe_custom.conf")
	if err != nil || string(override) != "test_override=true" {
		t.Fatalf("runtime override lost: %s: %v", override, err)
	}
}

const imageConfig = `# image configuration
JAVA_OPTS="-Dfile.encoding=UTF-8 -Xms8192m -Xmx8192m -XX:+UseG1GC -Xloggc:$LOG_DIR/fe.gc.log.$CUR_DATE"
JAVA_OPTS_FOR_JDK_17="-Dfile.encoding=UTF-8 -Xmx8192m -Xms8192m -XX:+HeapDumpOnOutOfMemoryError --add-opens=java.base/java.lang=ALL-UNNAMED -Xlog:gc*:$LOG_DIR/fe.gc.log.$CUR_DATE:time"
http_port=8030
`

func runPrepareScript(t *testing.T, defaults, source, heap string, env ...string) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "fe")
	src := filepath.Join(root, "source")
	out := filepath.Join(root, "prepared")
	for _, dir := range []string{filepath.Join(home, "conf"), src, out} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, "conf", "fe.conf"), []byte(defaults), 0644); err != nil {
		t.Fatal(err)
	}
	if source != "" {
		if err := os.WriteFile(filepath.Join(src, "fe.conf"), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "log4j2.xml"), []byte("sibling"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-ec", prepareScript)
	cmd.Env = append(os.Environ(), "FE_HEAP_HOME="+home, "FE_HEAP_SOURCE="+src, "FE_HEAP_OUTPUT="+out, "FE_HEAP_JDK8_MIB="+heap, "FE_HEAP_PRESERVE_JDK8=false", "FE_HEAP_PRESERVE_JDK17=false")
	cmd.Env = append(cmd.Env, env...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script: %s: %v", output, err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "fe.conf"))
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(filepath.Join(home, "conf", "fe.conf"))
	if string(original) != defaults {
		t.Fatal("image source changed")
	}
	if source != "" {
		original, _ = os.ReadFile(filepath.Join(src, "fe.conf"))
		if string(original) != source {
			t.Fatal("user source changed")
		}
	}
	sibling, _ := os.ReadFile(filepath.Join(out, "log4j2.xml"))
	if string(sibling) != "sibling" {
		t.Fatal("sibling file lost")
	}
	return string(raw)
}

func TestScriptAdaptsImageDefaults(t *testing.T) {
	for _, heap := range []string{"3276", "6553", "13107"} {
		got := runPrepareScript(t, imageConfig, "", heap)
		for _, want := range []string{"-Xms" + heap + "m -Xmx" + heap + "m", "-XX:InitialRAMPercentage=80 -XX:MaxRAMPercentage=80", "--add-opens=java.base/java.lang=ALL-UNNAMED", "$LOG_DIR/fe.gc.log.$CUR_DATE", "http_port=8030"} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %q in %s", want, got)
			}
		}
		if strings.Count(got, "-Xms") != 1 || strings.Count(got, "-Xmx") != 1 {
			t.Fatalf("unexpected extra fixed heap flags: %s", got)
		}
	}
}

func TestScriptPreservesUserJVMSettings(t *testing.T) {
	for _, source := range []string{
		"JAVA_OPTS_FOR_JDK_17=\"-Xms8192m -Xmx8192m -Dcustom=true\"\nhttp_port=18030\n",
		"JAVA_OPTS_FOR_JDK_17='-XX:InitialRAMPercentage=10 -XX:MaxRAMPercentage=60'\n",
		"JAVA_OPTS=\"-Xmx512m\"\nJAVA_OPTS_FOR_JDK_17=\"-Xmx1g\"\n",
		"JAVA_OPTS_FOR_JDK_17=\n",
	} {
		got := runPrepareScript(t, imageConfig, source, "3276")
		for _, line := range strings.Split(strings.TrimSpace(source), "\n") {
			if !strings.Contains(got, line) {
				t.Fatalf("user setting lost: %s in %s", line, got)
			}
		}
		if strings.Contains(got, "MaxRAMPercentage=80") {
			t.Fatal("overwrote user JDK17 settings")
		}
	}
}

func TestScriptFillsMissingJVMSettings(t *testing.T) {
	got := runPrepareScript(t, imageConfig, "# JAVA_OPTS=ignored\nhttp_port=18030\ncustom_config_dir=/custom\n", "3276")
	if !strings.Contains(got, "-Xms3276m -Xmx3276m") || !strings.Contains(got, "MaxRAMPercentage=80") || !strings.Contains(got, "custom_config_dir=/custom") || !strings.Contains(got, "http_port=18030") {
		t.Fatal(got)
	}
	// JDK 8 images need not contain JDK 17 options; do not create an incomplete entry.
	got = runPrepareScript(t, "JAVA_OPTS='-Xmx8192m -XX:+UseG1GC'\n", "", "3276")
	if strings.Contains(got, "JAVA_OPTS_FOR_JDK_17") || !strings.Contains(got, "-Xms3276m -Xmx3276m") {
		t.Fatal(got)
	}
}
