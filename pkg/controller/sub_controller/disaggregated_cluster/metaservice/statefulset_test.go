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

package metaservice

import (
	"context"
	"fmt"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewPodTemplateSpec_KeepsPodInfoMount(t *testing.T) {
	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ddc",
			Namespace: "default",
		},
		Spec: dv1.DorisDisaggregatedClusterSpec{
			MetaService: dv1.MetaService{
				CommonSpec: dv1.CommonSpec{
					Replicas: pointer.Int32(1),
					Image:    "selectdb/doris.ms-ubuntu:latest",
				},
				FDB: dv1.FDB{
					Address: "127.0.0.1:4500",
				},
			},
		},
	}

	dms := &DisaggregatedMSController{}
	pts := dms.NewPodTemplateSpec(ddc, map[string]string{}, map[string]interface{}{}, ddc.Spec.MetaService.FDB.Address)

	foundPodInfoMount := false
	for _, c := range pts.Spec.Containers {
		if c.Name != resource.DISAGGREGATED_MS_MAIN_CONTAINER_NAME {
			continue
		}
		for _, vm := range c.VolumeMounts {
			if vm.Name == resource.POD_INFO_VOLUME_NAME && vm.MountPath == resource.POD_INFO_PATH {
				foundPodInfoMount = true
				break
			}
		}
	}
	if !foundPodInfoMount {
		t.Fatalf("expected metaservice container to keep podinfo mount %q at %q", resource.POD_INFO_VOLUME_NAME, resource.POD_INFO_PATH)
	}
}

func TestResolveFDBEndpointFromConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fdb-system", Name: "fdb-a-config"},
		Data:       map[string]string{FDBClusterFileKey: "fdb:new@fdb-0:4500"},
	}
	dms := &DisaggregatedMSController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
		K8sclient: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build(),
	}}
	ddc := newFDBConfigMapTestDDC()

	endpoint, err := dms.resolveFDBEndpoint(context.Background(), ddc)
	if err != nil {
		t.Fatalf("resolve FDB endpoint: %v", err)
	}
	if endpoint != cm.Data[FDBClusterFileKey] {
		t.Fatalf("endpoint = %q, want %q", endpoint, cm.Data[FDBClusterFileKey])
	}
}

func TestResolveFDBEndpointPrefersDirectAddress(t *testing.T) {
	ddc := newFDBConfigMapTestDDC()
	ddc.Spec.MetaService.FDB.Address = "fdb:direct@fdb-0:4500"
	dms := &DisaggregatedMSController{}

	endpoint, err := dms.resolveFDBEndpoint(context.Background(), ddc)
	if err != nil {
		t.Fatalf("resolve direct FDB endpoint: %v", err)
	}
	if endpoint != ddc.Spec.MetaService.FDB.Address {
		t.Fatalf("endpoint = %q, want direct address %q", endpoint, ddc.Spec.MetaService.FDB.Address)
	}
}

func TestSyncKeepsStatefulSetWhenFDBConfigMapIsInvalid(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}

	ddc := newFDBConfigMapTestDDC()
	oldEndpoint := "fdb:old@fdb-0:4500"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fdb-system", Name: "fdb-a-config"},
		Data:       map[string]string{},
	}
	sts := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: ddc.Namespace, Name: ddc.GetMSStatefulsetName()},
		Spec: appv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: resource.DISAGGREGATED_MS_MAIN_CONTAINER_NAME,
				Env:  []corev1.EnvVar{{Name: resource.FDB_ENDPOINT, Value: oldEndpoint}},
			}},
		}}},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, sts).Build()
	dms := &DisaggregatedMSController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
		K8sclient:   k8sClient,
		K8srecorder: record.NewFakeRecorder(1),
	}}

	if err := dms.Sync(context.Background(), ddc); err == nil {
		t.Fatal("Sync should fail when cluster-file is missing")
	}
	var live appv1.StatefulSet
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: sts.Namespace, Name: sts.Name}, &live); err != nil {
		t.Fatalf("get live StatefulSet: %v", err)
	}
	gotEndpoint := live.Spec.Template.Spec.Containers[0].Env[0].Value
	if gotEndpoint != oldEndpoint {
		t.Fatalf("live FDB endpoint = %q, want preserved endpoint %q", gotEndpoint, oldEndpoint)
	}
}

func newFDBConfigMapTestDDC() *dv1.DorisDisaggregatedCluster {
	return &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ddc", Namespace: "doris-system"},
		Spec: dv1.DorisDisaggregatedClusterSpec{MetaService: dv1.MetaService{
			FDB: dv1.FDB{ConfigMapNamespaceName: dv1.NamespaceName{
				Namespace: "fdb-system",
				Name:      "fdb-a-config",
			}},
		}},
	}
}

// Model the real namespace cache: foreign ConfigMap reads must not go through it.
type namespaceCachedClient struct {
	client.Client
	namespace string
}

func (c namespaceCachedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.ConfigMap); ok && key.Namespace != c.namespace {
		return fmt.Errorf("unknown namespace for the cache: %s", key.Namespace)
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

type forbiddenFDBReader struct{ client.Reader }

func (r forbiddenFDBReader) Get(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, key.Name, fmt.Errorf("cross-namespace access denied"))
}

func TestSyncUsesAPIReaderForExternalFDBAndRefreshesEndpoint(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	ddc := newFDBConfigMapTestDDC()
	ddc.Spec.MetaService.Replicas = pointer.Int32(1)
	ddc.Spec.MetaService.Image = "apache/doris:ms-4.1.1"
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "fdb-system", Name: "fdb-a-config"},
		Data: map[string]string{FDBClusterFileKey: "fdb:initial@fdb-0:4500"}}
	live := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	cached := namespaceCachedClient{Client: live, namespace: ddc.Namespace}
	dms := &DisaggregatedMSController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
		K8sclient: cached, APIReader: live, K8srecorder: record.NewFakeRecorder(20),
	}}
	assertEndpoint := func(want string) {
		t.Helper()
		var st appv1.StatefulSet
		if err := live.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.GetMSStatefulsetName()}, &st); err != nil {
			t.Fatal(err)
		}
		for _, c := range st.Spec.Template.Spec.Containers {
			for _, e := range c.Env {
				if e.Name == resource.FDB_ENDPOINT {
					if e.Value != want {
						t.Fatalf("endpoint=%q, want=%q", e.Value, want)
					}
					return
				}
			}
		}
		t.Fatal("missing FDB endpoint")
	}
	if err := dms.Sync(ctx, ddc); err != nil {
		t.Fatal(err)
	}
	assertEndpoint(cm.Data[FDBClusterFileKey])
	if err := live.Get(ctx, client.ObjectKeyFromObject(cm), cm); err != nil {
		t.Fatal(err)
	}
	cm.Data[FDBClusterFileKey] = "fdb:updated@fdb-1:4500"
	if err := live.Update(ctx, cm); err != nil {
		t.Fatal(err)
	}
	if err := dms.Sync(ctx, ddc); err != nil {
		t.Fatal(err)
	}
	assertEndpoint(cm.Data[FDBClusterFileKey])
	// APIReader must not bypass authorization or overwrite a working workload
	// when the reference becomes inaccessible or disappears.
	dms.APIReader = forbiddenFDBReader{Reader: live}
	if err := dms.Sync(ctx, ddc); !apierrors.IsForbidden(err) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	assertEndpoint(cm.Data[FDBClusterFileKey])
	dms.APIReader = live
	if err := live.Delete(ctx, cm); err != nil {
		t.Fatal(err)
	}
	if err := dms.Sync(ctx, ddc); !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
	assertEndpoint(cm.Data[FDBClusterFileKey])
}
