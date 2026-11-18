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
	"fmt"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func upgradeFixture(kind string) (client.Object, *appsv1.StatefulSet, *appsv1.StatefulSet, *corev1.Pod) {
	var cluster client.Object
	container := "be"
	if kind == "DCR" {
		cluster = &dorisv1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test", UID: "cluster", Generation: 99},
			Spec: dorisv1.DorisClusterSpec{
				FeSpec: &dorisv1.FeSpec{BaseSpec: dorisv1.BaseSpec{Image: "fe:new", Replicas: ptr.To(int32(1))}},
				BeSpec: &dorisv1.BeSpec{BaseSpec: dorisv1.BaseSpec{Image: "be:new", Replicas: ptr.To(int32(1))}},
			}}
	} else {
		container = "compute"
		cluster = &dv1.DorisDisaggregatedCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test", UID: "cluster", Generation: 99},
			Spec: dv1.DorisDisaggregatedClusterSpec{
				FeSpec:        dv1.FeSpec{CommonSpec: dv1.CommonSpec{Image: "fe:new", Replicas: ptr.To(int32(1))}},
				ComputeGroups: []dv1.ComputeGroup{{UniqueId: "be", CommonSpec: dv1.CommonSpec{Image: "be:new", Replicas: ptr.To(int32(1))}}},
			}}
	}
	fe := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "test-fe", Namespace: "test"}, Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe", Image: "fe:old"}}}}}}
	be := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "test-be", Namespace: "test", UID: "backend", Generation: 2}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(1)), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend"}}, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: container, Image: "be:new"}}}}}, Status: appsv1.StatefulSetStatus{ObservedGeneration: 2, UpdateRevision: "new", CurrentRevision: "new"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-be-0", Namespace: "test", UID: "pod", Labels: map[string]string{"app": "backend", appsv1.StatefulSetRevisionLabel: "new"}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "StatefulSet", Name: be.Name, UID: be.UID, Controller: ptr.To(true)}}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: container, Image: "be:new"}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
	return cluster, fe, be, pod
}

func upgradeReader(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestFrontendUpgradeSafety(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*appsv1.StatefulSet, *appsv1.StatefulSet, *corev1.Pod)
		allow  bool
	}{
		{"backend converged despite newer CR generation", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {}, true},
		{"BE-only change preserves existing FE reconciliation", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			fe.Spec.Template.Spec.Containers[0].Image = "fe:new"
			be.Spec.Template.Spec.Containers[0].Image = "be:old"
		}, true},
		{"old healthy StatefulSet cannot release FE", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			be.Spec.Template.Spec.Containers[0].Image = "be:old"
			p.Spec.Containers[0].Image = "be:old"
		}, false},
		{"StatefulSet controller has not observed new spec", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { be.Status.ObservedGeneration = 1 }, false},
		{"old Pod image", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { p.Spec.Containers[0].Image = "be:old" }, false},
		{"old Pod revision", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { p.Labels[appsv1.StatefulSetRevisionLabel] = "old" }, false},
		{"revision not assigned", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { be.Status.UpdateRevision = "" }, false},
		{"unready Pod", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { p.Status.Conditions[0].Status = corev1.ConditionFalse }, false},
		{"missing PodReady condition", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { p.Status.Conditions = nil }, false},
		{"terminating Pod", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			now := metav1.Now()
			p.DeletionTimestamp = &now
			p.Finalizers = []string{"test"}
		}, false},
		{"terminating StatefulSet", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			now := metav1.Now()
			be.DeletionTimestamp = &now
			be.Finalizers = []string{"test"}
		}, false},
		{"Pod controlled by another StatefulSet", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { p.OwnerReferences[0].UID = "another" }, false},
		{"replicas not applied", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { be.Spec.Replicas = ptr.To(int32(2)) }, false},
		{"Pod not running", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) { p.Status.Phase = corev1.PodFailed }, false},
		{"OnDelete completed with stale currentRevision", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			be.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
			be.Status.CurrentRevision = "old"
		}, true},
		{"graceful replacement waiting for BE Alive", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			be.Annotations = map[string]string{"doris.disaggregated.cluster/graceful-action": `{"type":"RollingUpdate","phase":"WaitBEAlive"}`}
		}, false},
		{"failed graceful action", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			be.Annotations = map[string]string{"doris.disaggregated.cluster/graceful-action": `{"phase":"Failed"}`}
		}, false},
		{"graceful action completed", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			be.Annotations = map[string]string{"doris.disaggregated.cluster/graceful-action": `{"phase":"Done"}`}
		}, true},
		{"malformed graceful action", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			be.Annotations = map[string]string{"doris.disaggregated.cluster/graceful-action": "invalid"}
		}, false},
		{"sidecar order does not affect main image comparison", func(fe, be *appsv1.StatefulSet, p *corev1.Pod) {
			p.Spec.Containers = append([]corev1.Container{{Name: "sidecar", Image: "sidecar"}}, p.Spec.Containers...)
		}, true},
	}
	for _, kind := range []string{"DCR", "DDC"} {
		for _, tt := range tests {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				cluster, fe, be, pod := upgradeFixture(kind)
				tt.mutate(fe, be, pod)
				err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, fe, be, pod), cluster)
				if tt.allow && err != nil {
					t.Fatalf("unexpected wait: %v", err)
				}
				if !tt.allow && !IsBackendUpgradePending(err) {
					t.Fatalf("expected backend wait, got %v", err)
				}
			})
		}
	}
}

func TestFrontendUpgradeMissingResourcesAndCreation(t *testing.T) {
	for _, kind := range []string{"DCR", "DDC"} {
		t.Run(kind, func(t *testing.T) {
			cluster, fe, be, pod := upgradeFixture(kind)
			for _, tt := range []struct {
				name    string
				objects []client.Object
				allow   bool
			}{
				{"first deployment", nil, true},
				{"missing backend StatefulSet", []client.Object{fe}, false},
				{"missing backend Pod", []client.Object{fe, be}, false},
				{"zero replicas", []client.Object{fe, be}, true},
			} {
				t.Run(tt.name, func(t *testing.T) {
					if tt.name == "zero replicas" {
						be.Spec.Replicas = ptr.To(int32(0))
						be.Status.UpdateRevision = ""
						switch c := cluster.(type) {
						case *dorisv1.DorisCluster:
							c.Spec.BeSpec.Replicas = ptr.To(int32(0))
						case *dv1.DorisDisaggregatedCluster:
							c.Spec.ComputeGroups[0].Replicas = ptr.To(int32(0))
						}
					}
					err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, tt.objects...), cluster)
					if tt.allow && err != nil || !tt.allow && !IsBackendUpgradePending(err) {
						t.Fatalf("allow=%v, got %v", tt.allow, err)
					}
				})
			}
			// No FE image change must not gate config/replica repair, even with no BE.
			fe.Spec.Template.Spec.Containers[0].Image = "fe:new"
			if err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, fe), cluster); err != nil {
				t.Fatal(err)
			}
			_ = pod
		})
	}
}

func TestFrontendUpgradeSeparatePatchesAndRestart(t *testing.T) {
	for _, kind := range []string{"DCR", "DDC"} {
		t.Run(kind, func(t *testing.T) {
			cluster, fe, be, pod := upgradeFixture(kind)
			// The BE patch arrives first: FE image is still unchanged.
			fe.Spec.Template.Spec.Containers[0].Image = "fe:new"
			be.Spec.Template.Spec.Containers[0].Image = "be:old"
			pod.Spec.Containers[0].Image = "be:old"
			if err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, fe, be, pod), cluster); err != nil {
				t.Fatal(err)
			}
			// FE patch arrives while BE is still old. Recreate the reader each time to
			// model restart/relist; the gate must not depend on previous process state.
			fe.Spec.Template.Spec.Containers[0].Image = "fe:old"
			for i := 0; i < 3; i++ {
				if err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, fe, be, pod), cluster); !IsBackendUpgradePending(err) {
					t.Fatalf("restart %d: %v", i, err)
				}
			}
			be.Spec.Template.Spec.Containers[0].Image = "be:new"
			if err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, fe, be, pod), cluster); !IsBackendUpgradePending(err) {
				t.Fatalf("old pod passed: %v", err)
			}
			pod.Spec.Containers[0].Image = "be:new"
			if err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, fe, be, pod), cluster); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFrontendUpgradeWaitsForEveryComputeGroup(t *testing.T) {
	obj, fe, be, pod := upgradeFixture("DDC")
	cluster := obj.(*dv1.DorisDisaggregatedCluster)
	second := cluster.Spec.ComputeGroups[0]
	second.UniqueId = "second"
	cluster.Spec.ComputeGroups = append(cluster.Spec.ComputeGroups, second)
	be2 := be.DeepCopy()
	be2.Name = "test-second"
	be2.UID = "second"
	be2.Spec.Selector.MatchLabels["app"] = "second"
	p2 := pod.DeepCopy()
	p2.Name = "test-second-0"
	p2.OwnerReferences[0].UID = be2.UID
	p2.Labels["app"] = "second"
	p2.Spec.Containers[0].Image = "be:old"
	if err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, fe, be, pod, be2, p2), cluster); !IsBackendUpgradePending(err) {
		t.Fatalf("second group bypassed: %v", err)
	}
	p2.Spec.Containers[0].Image = "be:new"
	if err := CheckFrontendUpgrade(context.Background(), upgradeReader(t, fe, be, pod, be2, p2), cluster); err != nil {
		t.Fatal(err)
	}
}

type failingUpgradeReader struct {
	client.Reader
	failList bool
}

func (r failingUpgradeReader) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if !r.failList && key.Name == "test-be" {
		return fmt.Errorf("backend read unavailable")
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}
func (r failingUpgradeReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return fmt.Errorf("pod list unavailable")
}
func TestFrontendUpgradeReadFailuresFailClosed(t *testing.T) {
	obj, fe, be, pod := upgradeFixture("DCR")
	for _, failList := range []bool{false, true} {
		reader := failingUpgradeReader{Reader: upgradeReader(t, fe, be, pod), failList: failList}
		if err := CheckFrontendUpgrade(context.Background(), reader, obj); err == nil {
			t.Fatal("read failure released FE")
		}
	}
}
