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
	"path/filepath"
	"testing"
	"time"

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestDorisClusterWatchesExistingNonControllerOwnedResources(t *testing.T) {
	environment := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")}, ErrorIfCRDPathMissing: true}
	config, err := environment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{dorisv1.AddToScheme, appsv1.AddToScheme, corev1.AddToScheme, autoscalingv2.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	direct, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "native-owner-watch-test"}}
	if err := direct.Create(ctx, ns); err != nil {
		t.Fatal(err)
	}
	owner := &dorisv1.DorisCluster{TypeMeta: metav1.TypeMeta{APIVersion: dorisv1.GroupVersion.String(), Kind: "DorisCluster"}, ObjectMeta: metav1.ObjectMeta{Name: "doris", Namespace: ns.Name}, Spec: dorisv1.DorisClusterSpec{
		FeSpec: &dorisv1.FeSpec{BaseSpec: dorisv1.BaseSpec{Replicas: ptr.To(int32(1)), Image: "doris:fe"}},
		BeSpec: &dorisv1.BeSpec{BaseSpec: dorisv1.BaseSpec{Replicas: ptr.To(int32(1)), Image: "doris:be"}},
	}}
	if err := direct.Create(ctx, owner); err != nil {
		t.Fatal(err)
	}
	owner.APIVersion = dorisv1.GroupVersion.String()
	owner.Kind = "DorisCluster"
	service := resource.BuildExternalService(owner, dorisv1.Component_FE, map[string]interface{}{})
	statefulSet := resource.NewStatefulSet(owner, map[string]interface{}{}, dorisv1.Component_BE)
	// Simulate upgrading an operator that already manages resources with legacy owners.
	for _, object := range []client.Object{&service, &statefulSet} {
		if metav1.GetControllerOf(object) != nil {
			t.Fatal("fixture must retain its non-controller owner")
		}
		if err := direct.Create(ctx, object); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := ctrl.NewManager(config, ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: "0"}, HealthProbeBindAddress: "0"})
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan reconcile.Request, 100)
	builder := (&DorisClusterReconciler{}).resourceBuilder(ctrl.NewControllerManagedBy(manager).Named("native-resource-owner-test"))
	if err := builder.Complete(reconcile.Func(func(ctx context.Context, request reconcile.Request) (ctrl.Result, error) {
		requests <- request
		return ctrl.Result{}, nil
	})); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- manager.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-stopped:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(10 * time.Second):
			t.Error("manager did not stop")
		}
	})
	syncCtx, syncCancel := context.WithTimeout(ctx, 10*time.Second)
	defer syncCancel()
	if !manager.GetCache().WaitForCacheSync(syncCtx) {
		t.Fatal("cache did not synchronize")
	}
	drain := func() {
		t.Helper()
		for {
			select {
			case <-requests:
			case <-time.After(300 * time.Millisecond):
				return
			}
		}
	}
	expectRequest := func() {
		t.Helper()
		select {
		case request := <-requests:
			if request.Name != owner.Name || request.Namespace != owner.Namespace {
				t.Fatalf("unexpected owner request: %v", request)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("resource event did not enqueue the existing DorisCluster")
		}
	}
	drain()
	if err := direct.Delete(ctx, &service); err != nil {
		t.Fatal(err)
	}
	expectRequest()
	drain()
	current := &appsv1.StatefulSet{}
	if err := direct.Get(ctx, client.ObjectKeyFromObject(&statefulSet), current); err != nil {
		t.Fatal(err)
	}
	base := current.DeepCopy()
	current.Spec.Replicas = ptr.To(int32(2))
	if err := direct.Patch(ctx, current, client.MergeFrom(base)); err != nil {
		t.Fatal(err)
	}
	expectRequest()
	drain()
	// Unrelated resources must not enqueue this CR merely because the type is watched.
	unrelated := service.DeepCopy()
	unrelated.ResourceVersion = ""
	unrelated.UID = ""
	unrelated.Name = "unrelated"
	unrelated.OwnerReferences = nil
	if err := direct.Create(ctx, unrelated); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-requests:
		t.Fatalf("unowned resource enqueued a CR: %v", request)
	case <-time.After(300 * time.Millisecond):
	}
}
