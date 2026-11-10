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
package cn

import (
	"context"
	"testing"
	"time"

	v1 "github.com/apache/doris-operator/api/doris/v1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newCnController(t *testing.T, objects ...client.Object) *Controller {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appsv1 to scheme failed, err=%s", err.Error())
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 to scheme failed, err=%s", err.Error())
	}
	return New(fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(), record.NewFakeRecorder(10))
}

// Sync only calls ClearResources when CnSpec is nil, so a cluster whose cnSpec was removed while
// status.cnStatus is still set must not dereference the spec.
func Test_ClearResources(t *testing.T) {
	cc := newCnController(t)
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1.DorisClusterSpec{
			CnSpec: nil,
		},
		Status: v1.DorisClusterStatus{
			CnStatus: &v1.CnStatus{
				ComponentStatus: v1.ComponentStatus{
					ComponentCondition: v1.ComponentCondition{
						Phase: v1.Reconciling,
					},
				},
			},
		},
	}

	cleared, err := cc.ClearResources(context.Background(), cluster)
	if err != nil {
		t.Errorf("Test_ClearResources clear resources failed, err=%s", err.Error())
	}
	if !cleared {
		t.Error("Test_ClearResources expected resources to be cleared")
	}
}

// A cluster that never had cn returns early on the nil status, before reaching the spec.
func Test_ClearResources_NoCnStatus(t *testing.T) {
	cc := newCnController(t)
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-no-cn",
			Namespace: "default",
		},
	}

	cleared, err := cc.ClearResources(context.Background(), cluster)
	if err != nil {
		t.Errorf("Test_ClearResources_NoCnStatus failed, err=%s", err.Error())
	}
	if !cleared {
		t.Error("Test_ClearResources_NoCnStatus expected resources to be cleared")
	}
}

// With enableRestartWhenConfigChange, a be.conf change in the cn configmap must stamp a restart on the
// cn pod template, and the next pass must keep it rather than roll the pods a second time.
func Test_Sync_RestartWhenConfigChange(t *testing.T) {
	replicas := int32(1)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cn-config", Namespace: "default"},
		Data:       map[string]string{"be.conf": "file_cache_path = old\n"},
	}
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1.DorisClusterSpec{
			EnableRestartWhenConfigChange: true,
			CnSpec: &v1.CnSpec{
				BaseSpec: v1.BaseSpec{
					Replicas:      &replicas,
					Image:         "apache/doris:be-test",
					ConfigMapInfo: v1.ConfigMapInfo{ConfigMapName: "cn-config"},
				},
			},
		},
	}
	feEndpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: v1.GenerateExternalServiceName(cluster, v1.Component_FE), Namespace: "default"},
		Subsets:    []corev1.EndpointSubset{{Addresses: []corev1.EndpointAddress{{IP: "127.0.0.1"}}}},
	}
	cc := newCnController(t, cm, feEndpoints)
	ctx := context.Background()
	stName := v1.GenerateComponentStatefulSetName(cluster, v1.Component_CN)
	restartedAt := func() string {
		var st appv1.StatefulSet
		if err := cc.K8sclient.Get(ctx, types.NamespacedName{Namespace: "default", Name: stName}, &st); err != nil {
			t.Fatalf("get cn statefulset failed, err=%s", err.Error())
		}
		return st.Spec.Template.Annotations[v1.CNRestartAt]
	}
	// the reconciler reads the cr fresh each pass and only writes status back, so drop in-memory spec edits.
	stored := cluster.DeepCopy()
	reconcile := func() {
		cluster = stored.DeepCopy()
		cluster.Status = stored.Status
		defer func() { stored.Status = *cluster.Status.DeepCopy() }()
		if err := cc.Sync(ctx, cluster); err != nil {
			t.Fatalf("cn sync failed, err=%s", err.Error())
		}
		if err := cc.UpdateComponentStatus(cluster); err != nil {
			t.Fatalf("cn update status failed, err=%s", err.Error())
		}
		// no pods exist under the fake client, so mark cn available as a running cluster would be.
		cluster.Status.CnStatus.ComponentCondition.Phase = v1.Available
	}

	reconcile()
	if cluster.Status.CnStatus.CoreConfigMapHashValue == "" {
		t.Fatal("cn status should record the core configmap hash")
	}
	if got := restartedAt(); got != "" {
		t.Fatalf("first deploy must not restart cn, restartedAt=%s", got)
	}

	cm.Data["be.conf"] = "file_cache_path = new\n"
	if err := cc.K8sclient.Update(ctx, cm); err != nil {
		t.Fatalf("update cn configmap failed, err=%s", err.Error())
	}
	cluster = stored.DeepCopy()
	if err := cc.Sync(ctx, cluster); err != nil {
		t.Fatalf("cn sync failed, err=%s", err.Error())
	}
	stored.Status = *cluster.Status.DeepCopy()
	if cluster.Status.CnStatus.ComponentCondition.Phase != v1.Restarting {
		t.Errorf("cn phase should be restarting, got %s", cluster.Status.CnStatus.ComponentCondition.Phase)
	}
	first := restartedAt()
	if _, err := time.Parse(time.RFC3339, first); err != nil {
		t.Fatalf("cn pod template should carry an RFC3339 restartedAt, got %q", first)
	}

	reconcile()
	if got := restartedAt(); got != first {
		t.Errorf("an unchanged configmap must leave restartedAt alone, was %s now %s", first, got)
	}
}
