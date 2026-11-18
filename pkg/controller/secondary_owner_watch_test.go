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
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appsv1 "k8s.io/api/apps/v1"
	av2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
	"time"
)

func TestSecondaryResourcesEnqueueLegacyOwners(t *testing.T) {
	environment := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")}, ErrorIfCRDPathMissing: true}
	cfg, err := environment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{v1.AddToScheme, dv1.AddToScheme, appsv1.AddToScheme, corev1.AddToScheme, av2.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "secondary-watch-test"}}); err != nil {
		t.Fatal(err)
	}
	owner := &dv1.DorisDisaggregatedCluster{TypeMeta: metav1.TypeMeta{APIVersion: dv1.GroupVersion.String(), Kind: "DorisDisaggregatedCluster"}, ObjectMeta: metav1.ObjectMeta{Name: "ddc", Namespace: "secondary-watch-test", UID: "ddc-uid"}}
	defaults := &sc.DisaggregatedSubDefaultController{}
	svc := defaults.NewDefaultService(owner)
	svc.Name = "ddc-fe"
	svc.Spec.Ports = []corev1.ServicePort{{Name: "query", Port: 9030}}
	st := defaults.NewDefaultStatefulset(owner)
	st.Name = "ddc-ms"
	st.Spec.ServiceName = svc.Name
	st.Spec.Replicas = ptr.To(int32(1))
	st.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "ms"}}
	st.Spec.Template = corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "ms"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "ms", Image: "ms:test"}}}}
	hpa := &av2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "cn-hpa", Namespace: owner.Namespace, OwnerReferences: []metav1.OwnerReference{{APIVersion: v1.GroupVersion.String(), Kind: "DorisCluster", Name: "native", UID: "native-uid"}}}, Spec: av2.HorizontalPodAutoscalerSpec{MinReplicas: ptr.To(int32(1)), MaxReplicas: 5, ScaleTargetRef: av2.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "cn"}}}
	for _, obj := range []client.Object{svc, st, hpa} {
		if metav1.GetControllerOf(obj) != nil {
			t.Fatal("fixture must retain legacy owner")
		}
		if err := c.Create(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: "0"}, HealthProbeBindAddress: "0"})
	if err != nil {
		t.Fatal(err)
	}
	ddcRequests, nativeRequests := make(chan reconcile.Request, 100), make(chan reconcile.Request, 100)
	bind := func(b *ctrl.Builder, ch chan reconcile.Request) {
		t.Helper()
		if err := b.Complete(reconcile.Func(func(_ context.Context, r reconcile.Request) (ctrl.Result, error) { ch <- r; return ctrl.Result{}, nil })); err != nil {
			t.Fatal(err)
		}
	}
	bind((&DisaggregatedClusterReconciler{}).resourceBuilder(ctrl.NewControllerManagedBy(mgr).Named("ddc-secondary-watch")), ddcRequests)
	bind((&DorisClusterReconciler{}).resourceBuilder(ctrl.NewControllerManagedBy(mgr).Named("native-secondary-watch")), nativeRequests)
	stopped := make(chan error, 1)
	go func() { stopped <- mgr.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-stopped:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(10 * time.Second):
			t.Error("manager stop timed out")
		}
	})
	syncCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	if !mgr.GetCache().WaitForCacheSync(syncCtx) {
		t.Fatal("cache sync failed")
	}
	drain := func(ch chan reconcile.Request) {
		for {
			select {
			case <-ch:
			case <-time.After(300 * time.Millisecond):
				return
			}
		}
	}
	expect := func(ch chan reconcile.Request, name string) {
		t.Helper()
		select {
		case r := <-ch:
			if r.Name != name || r.Namespace != owner.Namespace {
				t.Fatalf("unexpected request %v", r)
			}
		case <-time.After(4 * time.Second):
			t.Fatalf("child event did not enqueue %s", name)
		}
	}
	drain(ddcRequests)
	drain(nativeRequests)
	before := st.DeepCopy()
	st.Spec.Replicas = ptr.To(int32(2))
	if err := c.Patch(ctx, st, client.MergeFrom(before)); err != nil {
		t.Fatal(err)
	}
	expect(ddcRequests, "ddc")
	drain(ddcRequests)
	if err := c.Delete(ctx, svc); err != nil {
		t.Fatal(err)
	}
	expect(ddcRequests, "ddc")
	base := hpa.DeepCopy()
	hpa.Spec.MaxReplicas = 7
	if err := c.Patch(ctx, hpa, client.MergeFrom(base)); err != nil {
		t.Fatal(err)
	}
	expect(nativeRequests, "native")
	drain(nativeRequests)
	if err := c.Delete(ctx, hpa); err != nil {
		t.Fatal(err)
	}
	expect(nativeRequests, "native")
	drain(nativeRequests)
	unrelated := hpa.DeepCopy()
	unrelated.Name = "unowned-hpa"
	unrelated.ResourceVersion = ""
	unrelated.UID = ""
	unrelated.OwnerReferences = nil
	if err := c.Create(ctx, unrelated); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-nativeRequests:
		t.Fatalf("unrelated HPA enqueued %v", r)
	case <-time.After(300 * time.Millisecond):
	}
}
