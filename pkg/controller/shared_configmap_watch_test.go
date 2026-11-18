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

package controller

import (
	"context"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sort"
	"testing"
)

func TestSharedConfigMapEnqueuesAllCurrentReferences(t *testing.T) {
	scheme := runtime.NewScheme()
	v1.AddToScheme(scheme)
	fixture := func(name, ns string, enabled bool) *v1.DorisCluster {
		return &v1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}, Spec: v1.DorisClusterSpec{EnableRestartWhenConfigChange: enabled, FeSpec: &v1.FeSpec{BaseSpec: v1.BaseSpec{ConfigMapInfo: v1.ConfigMapInfo{ConfigMapName: "shared"}}}, BeSpec: &v1.BeSpec{BaseSpec: v1.BaseSpec{ConfigMapInfo: v1.ConfigMapInfo{ConfigMapName: "shared"}}}}}
	}
	a, b := fixture("a", "test", true), fixture("b", "test", true)
	c := fake.NewClientBuilder().WithScheme(scheme).WithIndex(&v1.DorisCluster{}, coreConfigMapIndex, coreConfigMapIndexKeys).WithObjects(a, b, fixture("disabled", "test", false), fixture("other-namespace", "other", true)).Build()
	r := &DorisClusterReconciler{Client: c}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "test"}}
	check := func(want string) {
		t.Helper()
		reqs := r.mapCoreConfigMapToClusters(context.Background(), cm)
		names := []string{}
		for _, req := range reqs {
			if req.Namespace != "test" {
				t.Fatal("wrong namespace")
			}
			names = append(names, req.Name)
		}
		sort.Strings(names)
		got := ""
		for _, name := range names {
			got += name
		}
		if got != want {
			t.Fatalf("got=%s want=%s", got, want)
		}
	}
	check("ab")
	if len(coreConfigMapIndexKeys(a)) != 1 {
		t.Fatal("duplicate FE/BE references were not deduplicated")
	}
	a.Spec.FeSpec.ConfigMapInfo.ConfigMapName = "different"
	a.Spec.BeSpec.ConfigMapInfo.ConfigMapName = "different"
	if err := c.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	check("b")
	b.Spec.EnableRestartWhenConfigChange = false
	if err := c.Update(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	check("")
	b.Spec.EnableRestartWhenConfigChange = true
	if err := c.Update(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	check("b")
	if err := c.Delete(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	check("")
}
func TestCoreConfigMapPredicateChecksContents(t *testing.T) {
	p := coreConfigMapPredicate()
	old := &corev1.ConfigMap{Data: map[string]string{"fe.conf": "a"}}
	same := old.DeepCopy()
	same.Annotations = map[string]string{"external": "update"}
	if p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: same}) {
		t.Fatal("metadata-only event should not trigger content reconcile")
	}
	changed := old.DeepCopy()
	changed.Data["fe.conf"] = "b"
	if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: changed}) {
		t.Fatal("data change ignored")
	}
	changed = old.DeepCopy()
	changed.BinaryData = map[string][]byte{"config": []byte("value")}
	if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: changed}) {
		t.Fatal("binary data change ignored")
	}
	if !p.Create(event.CreateEvent{Object: old}) || !p.Delete(event.DeleteEvent{Object: old}) {
		t.Fatal("create/delete should map existing references")
	}
}
