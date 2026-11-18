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
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type pendingDcrController struct {
	pending bool
	calls   *int
}

func (f pendingDcrController) Sync(context.Context, *dorisv1.DorisCluster) error {
	*f.calls++
	if f.pending {
		return &sc.BackendUpgradePending{Message: "waiting"}
	}
	return nil
}
func (f pendingDcrController) ClearResources(context.Context, *dorisv1.DorisCluster) (bool, error) {
	return true, nil
}
func (f pendingDcrController) UpdateComponentStatus(*dorisv1.DorisCluster) error { return nil }
func (f pendingDcrController) GetControllerName() string                         { return "test" }

func TestDcrBackendWaitContinuesSyncAndRequeuesHealthyCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dorisv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	dcr := &dorisv1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}, Spec: dorisv1.DorisClusterSpec{FeSpec: &dorisv1.FeSpec{BaseSpec: dorisv1.BaseSpec{Replicas: ptr.To(int32(1))}}}, Status: dorisv1.DorisClusterStatus{FEStatus: &dorisv1.ComponentStatus{ComponentCondition: dorisv1.ComponentCondition{Phase: dorisv1.Available}}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(dcr).WithObjects(dcr).Build()
	feCalls, beCalls := 0, 0
	r := &DorisClusterReconciler{Client: c, Recorder: record.NewFakeRecorder(10), Scs: map[string]sc.SubController{"fe": pendingDcrController{true, &feCalls}, "be": pendingDcrController{false, &beCalls}}}
	for i := 0; i < 3; i++ {
		result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "test", Namespace: "test"}})
		if err != nil {
			t.Fatal(err)
		}
		if result.RequeueAfter != sc.UpgradeRequeueAfter {
			t.Fatalf("healthy old status must still requeue: %+v", result)
		}
	}
	if feCalls != 3 || beCalls != 3 {
		t.Fatalf("wait starved controllers: FE=%d BE=%d", feCalls, beCalls)
	}
}

type pendingDdcController struct {
	pending bool
	calls   *int
}

func (f pendingDdcController) Sync(context.Context, client.Object) error {
	*f.calls++
	if f.pending {
		return &sc.BackendUpgradePending{Message: "waiting"}
	}
	return nil
}
func (f pendingDdcController) ClearResources(context.Context, client.Object) (bool, error) {
	return true, nil
}
func (f pendingDdcController) UpdateComponentStatus(client.Object) error { return nil }
func (f pendingDdcController) GetControllerName() string                 { return "test" }

func TestDdcBackendWaitContinuesSyncAndSurvivesHealthyStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	ddc := &dv1.DorisDisaggregatedCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}, Status: dv1.DorisDisaggregatedClusterStatus{
		FEStatus: dv1.FEStatus{Phase: dv1.Ready, AvailableStatus: dv1.Available}, MetaServiceStatus: dv1.MetaServiceStatus{Phase: dv1.Ready, AvailableStatus: dv1.Available}, ClusterHealth: dv1.ClusterHealth{CGCount: 1, CGAvailableCount: 1},
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ddc).WithObjects(ddc).Build()
	feCalls, beCalls := 0, 0
	r := &DisaggregatedClusterReconciler{Client: c, Recorder: record.NewFakeRecorder(10), Scs: map[string]sc.DisaggregatedSubController{"fe": pendingDdcController{true, &feCalls}, "be": pendingDdcController{false, &beCalls}}}
	for i := 0; i < 3; i++ {
		result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "test", Namespace: "test"}})
		if err != nil {
			t.Fatal(err)
		}
		if result.RequeueAfter != sc.UpgradeRequeueAfter || result.Requeue {
			t.Fatalf("wait requeue was overwritten by status: %+v", result)
		}
	}
	if feCalls != 3 || beCalls != 3 {
		t.Fatalf("wait starved controllers: FE=%d BE=%d", feCalls, beCalls)
	}
}
