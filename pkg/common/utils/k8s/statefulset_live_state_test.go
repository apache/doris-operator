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
package k8s_test

import (
	"context"
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

func liveSTS() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "fe", Namespace: "test"}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(3)), ServiceName: "fe", Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "fe"}}, UpdateStrategy: appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType, RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{Partition: ptr.To(int32(0))}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "fe"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe", Image: "doris:good", Env: []corev1.EnvVar{{Name: "A", Value: "good"}}}}}}}}
}
func TestApplyStatefulSetRepairsCachedHashDrift(t *testing.T) {
	for _, key := range []string{v1.ComponentResourceHash, dv1.DisaggregatedSpecHashValueAnnotation} {
		for _, field := range []string{"image", "replicas", "strategy", "env"} {
			t.Run(key+"/"+field, func(t *testing.T) {
				old := liveSTS()
				resource.StatefulsetDeepEqualWithKey(old, old.DeepCopy(), key, false)
				switch field {
				case "image":
					old.Spec.Template.Spec.Containers[0].Image = "wrong"
				case "replicas":
					old.Spec.Replicas = ptr.To(int32(4))
				case "strategy":
					old.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
					old.Spec.UpdateStrategy.RollingUpdate = nil
				case "env":
					old.Spec.Template.Spec.Containers[0].Env[0].Value = "wrong"
				}
				c := fake.NewClientBuilder().WithObjects(old).Build()
				eq := func(n, o *appsv1.StatefulSet) bool { return resource.StatefulsetDeepEqualWithKey(n, o, key, false) }
				if err := k8s.ApplyStatefulSet(context.Background(), c, liveSTS(), eq); err != nil {
					t.Fatal(err)
				}
				got := &appsv1.StatefulSet{}
				if err := c.Get(context.Background(), client.ObjectKeyFromObject(old), got); err != nil {
					t.Fatal(err)
				}
				if *got.Spec.Replicas != 3 || got.Spec.Template.Spec.Containers[0].Image != "doris:good" || got.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType || got.Spec.Template.Spec.Containers[0].Env[0].Value != "good" {
					t.Fatalf("drift remains: %+v", got.Spec)
				}
				if !eq(liveSTS(), got) {
					t.Fatal("not idempotent after repair")
				}
			})
		}
	}
}
func TestLegacyConfigEnvReplicaUpdatePreservesTemplate(t *testing.T) {
	old := liveSTS()
	old.Spec.Template.Spec.Containers[0].Env = append(old.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "CONFIGMAP_MOUNT_PATH", Value: "/etc/doris"}, corev1.EnvVar{Name: "CONFIGMAP_MOUNT_PATH", Value: "/etc/doris"})
	resource.StatefulsetDeepEqualWithKey(old, old.DeepCopy(), "", false)
	old.Spec.Replicas = ptr.To(int32(4))
	c := fake.NewClientBuilder().WithObjects(old).Build()
	if err := k8s.ApplyStatefulSet(context.Background(), c, liveSTS(), func(n, o *appsv1.StatefulSet) bool { return resource.StatefulSetDeepEqual(n, o, false) }); err != nil {
		t.Fatal(err)
	}
	got := &appsv1.StatefulSet{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(old), got); err != nil {
		t.Fatal(err)
	}
	if *got.Spec.Replicas != 3 || len(got.Spec.Template.Spec.Containers[0].Env) != 3 {
		t.Fatal("legacy environment was changed during replica correction")
	}
}

func TestStatefulSetReconcileWithAPIDefaults(t *testing.T) {
	env := &envtest.Environment{}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Error(err)
		}
	})
	c, err := client.New(cfg, client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}); err != nil {
		t.Fatal(err)
	}
	desired := liveSTS()
	desired.Spec.Template.Spec.Containers[0].Env = append(desired.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}})
	desired.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"true"}}}}
	desired.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "config"}}}}, {Name: "podinfo", VolumeSource: corev1.VolumeSource{DownwardAPI: &corev1.DownwardAPIVolumeSource{Items: []corev1.DownwardAPIVolumeFile{{Path: "name", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}}}}}}
	resource.StatefulSetDeepEqual(desired, desired.DeepCopy(), false)
	created := desired.DeepCopy()
	if err := c.Create(ctx, created); err != nil {
		t.Fatal(err)
	}
	if !resource.StatefulSetDeepEqual(desired.DeepCopy(), created, false) {
		t.Fatalf("API defaults produce a difference: %+v", created.Spec.Template.Spec)
	}
	created.Spec.Template.Annotations = map[string]string{"external.example/rollout": "keep"}
	created.Spec.Template.Spec.Containers[0].Image = "doris:wrong"
	created.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.OnDeleteStatefulSetStrategyType}
	if err := c.Update(ctx, created); err != nil {
		t.Fatal(err)
	}
	equal := func(n, o *appsv1.StatefulSet) bool { return resource.StatefulSetDeepEqual(n, o, false) }
	if err := k8s.ApplyStatefulSet(ctx, c, desired.DeepCopy(), equal); err != nil {
		t.Fatal(err)
	}
	got := &appsv1.StatefulSet{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(created), got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.Template.Spec.Containers[0].Image != "doris:good" || got.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType || got.Spec.Template.Annotations["external.example/rollout"] != "keep" {
		t.Fatal("managed drift or external metadata incorrect after repair")
	}
	rv := got.ResourceVersion
	if err := k8s.ApplyStatefulSet(ctx, c, desired.DeepCopy(), equal); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(created), got); err != nil {
		t.Fatal(err)
	}
	if got.ResourceVersion != rv {
		t.Fatal("repeated reconcile patched unchanged workload")
	}
}
