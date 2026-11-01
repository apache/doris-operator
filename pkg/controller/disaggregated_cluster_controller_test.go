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
	"errors"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	"github.com/apache/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/metaservice"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type fakeDisaggregatedSubController struct {
	name     string
	clearErr error
}

func (f fakeDisaggregatedSubController) Sync(ctx context.Context, obj client.Object) error {
	return nil
}

func (f fakeDisaggregatedSubController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	return true, f.clearErr
}

func (f fakeDisaggregatedSubController) GetControllerName() string {
	return f.name
}

func (f fakeDisaggregatedSubController) UpdateComponentStatus(obj client.Object) error {
	return nil
}

var _ sc.DisaggregatedSubController = fakeDisaggregatedSubController{}

func TestReorganizeStatusConsidersMetaServiceHealth(t *testing.T) {
	tests := []struct {
		name              string
		metaServiceStatus dv1.MetaServiceStatus
		wantHealth        dv1.Health
	}{
		{
			name: "meta service not fully ready makes cluster yellow",
			metaServiceStatus: dv1.MetaServiceStatus{
				AvailableStatus: dv1.Available,
				Phase:           dv1.Reconciling,
			},
			wantHealth: dv1.Yellow,
		},
		{
			name: "meta service unavailable makes cluster red",
			metaServiceStatus: dv1.MetaServiceStatus{
				AvailableStatus: dv1.UnAvailable,
				Phase:           dv1.Reconciling,
			},
			wantHealth: dv1.Red,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ddc := &dv1.DorisDisaggregatedCluster{}
			ddc.Status.MetaServiceStatus = tt.metaServiceStatus
			ddc.Status.FEStatus.AvailableStatus = dv1.Available
			ddc.Status.FEStatus.Phase = dv1.Ready
			ddc.Status.ClusterHealth.CGCount = 1
			ddc.Status.ClusterHealth.CGAvailableCount = 1

			reconciler := &DisaggregatedClusterReconciler{
				Scs: map[string]sc.DisaggregatedSubController{
					"fake": fakeDisaggregatedSubController{name: "fake"},
				},
			}

			_, err := reconciler.reorganizeStatus(ddc)
			if err != nil {
				t.Fatalf("reorganizeStatus returned error: %v", err)
			}
			if ddc.Status.ClusterHealth.Health != tt.wantHealth {
				t.Fatalf("health = %s, want %s", ddc.Status.ClusterHealth.Health, tt.wantHealth)
			}
		})
	}
}

func TestMapFDBConfigMapToDDCs(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := dv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add DDC scheme: %v", err)
	}

	referencing := newTestDDC("doris-a", "cluster-a", "fdb-system", "fdb-a-config")
	unrelated := newTestDDC("doris-b", "cluster-b", "fdb-system", "fdb-b-config")
	directAddress := newTestDDC("doris-c", "cluster-c", "fdb-system", "fdb-a-config")
	directAddress.Spec.MetaService.FDB.Address = "fdb:direct@127.0.0.1:4500"
	reconciler := &DisaggregatedClusterReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(referencing, unrelated, directAddress).Build(),
	}

	requests := reconciler.mapFDBConfigMapToDDCs(context.Background(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fdb-system", Name: "fdb-a-config"},
	})

	if len(requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(requests))
	}
	if requests[0].Namespace != referencing.Namespace || requests[0].Name != referencing.Name {
		t.Fatalf("request = %s/%s, want %s/%s", requests[0].Namespace, requests[0].Name, referencing.Namespace, referencing.Name)
	}
}

func TestFDBConfigMapPredicate(t *testing.T) {
	p := fdbConfigMapPredicate()
	oldConfigMap := newTestFDBConfigMap("old-cluster-file")
	newConfigMap := newTestFDBConfigMap("new-cluster-file")
	sameConfigMap := oldConfigMap.DeepCopy()
	sameConfigMap.Annotations = map[string]string{"updated": "true"}
	withoutClusterFile := oldConfigMap.DeepCopy()
	delete(withoutClusterFile.Data, metaservice.FDBClusterFileKey)

	if !p.Create(event.CreateEvent{Object: oldConfigMap}) {
		t.Fatal("create event with cluster-file should pass")
	}
	if p.Update(event.UpdateEvent{ObjectOld: oldConfigMap, ObjectNew: sameConfigMap}) {
		t.Fatal("metadata-only update should not pass")
	}
	if !p.Update(event.UpdateEvent{ObjectOld: oldConfigMap, ObjectNew: newConfigMap}) {
		t.Fatal("cluster-file update should pass")
	}
	if !p.Update(event.UpdateEvent{ObjectOld: oldConfigMap, ObjectNew: withoutClusterFile}) {
		t.Fatal("cluster-file removal should pass")
	}
	if !p.Delete(event.DeleteEvent{Object: oldConfigMap}) {
		t.Fatal("delete event with cluster-file should pass")
	}
}

func newTestDDC(namespace, name, fdbNamespace, fdbConfigMap string) *dv1.DorisDisaggregatedCluster {
	return &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: dv1.DorisDisaggregatedClusterSpec{
			MetaService: dv1.MetaService{
				FDB: dv1.FDB{ConfigMapNamespaceName: dv1.NamespaceName{
					Namespace: fdbNamespace,
					Name:      fdbConfigMap,
				}},
			},
		},
	}
}

func newTestFDBConfigMap(clusterFile string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fdb-system", Name: "fdb-a-config"},
		Data:       map[string]string{metaservice.FDBClusterFileKey: clusterFile},
	}
}

func TestCleanupErrorsAreReturned(t *testing.T) {
	failure := errors.New("PVC cleanup conflict")
	r := &DisaggregatedClusterReconciler{Scs: map[string]sc.DisaggregatedSubController{
		"compute": fakeDisaggregatedSubController{name: "compute", clearErr: failure},
	}}
	_, err := r.clearUnusedResources(context.Background(), &dv1.DorisDisaggregatedCluster{})
	if !errors.Is(err, failure) {
		t.Fatalf("cleanup error was lost: %v", err)
	}
}
