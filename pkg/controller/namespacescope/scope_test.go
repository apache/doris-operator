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

package namespacescope

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNamespaceOwnershipChangesWithoutRestart(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "run-1", Labels: map[string]string{"selectdb.com/operator-group": "qa"}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()
	qa, err := New(c, "selectdb.com/operator-group=qa")
	if err != nil {
		t.Fatal(err)
	}
	hubspot, _ := New(c, "selectdb.com/operator-group=hubspot")
	ctx := context.Background()
	assertAllowed := func(scope *Scope, name string, want bool) {
		t.Helper()
		got, err := scope.Allows(ctx, name)
		if err != nil || got != want {
			t.Fatalf("Allows(%s) = %v, %v; want %v", name, got, err, want)
		}
	}
	assertAllowed(qa, ns.Name, true)
	assertAllowed(hubspot, ns.Name, false)
	assertAllowed(qa, "missing", false)
	ns.Labels = nil
	if err := c.Update(ctx, ns); err != nil {
		t.Fatal(err)
	}
	assertAllowed(qa, ns.Name, false)
	ns.Labels = map[string]string{"selectdb.com/operator-group": "hubspot"}
	if err := c.Update(ctx, ns); err != nil {
		t.Fatal(err)
	}
	assertAllowed(qa, ns.Name, false)
	assertAllowed(hubspot, ns.Name, true)
}

func TestNamespaceSelectorValidationAndLegacyMode(t *testing.T) {
	if _, err := New(nil, "   "); err == nil {
		t.Fatal("empty expression accepted")
	}
	if _, err := New(nil, "key in ("); err == nil {
		t.Fatal("invalid selector accepted")
	}
	s, err := New(nil, "")
	if err != nil || s.Enabled() {
		t.Fatal("legacy mode changed")
	}
	if allowed, err := s.Allows(context.Background(), "any"); err != nil || !allowed {
		t.Fatal("legacy mode must not read Namespace objects")
	}
}
