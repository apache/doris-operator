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

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/controller/sub_controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type recordingSubController struct {
	syncCalls, clearCalls, statusCalls int
}

func (r *recordingSubController) Sync(context.Context, *dorisv1.DorisCluster) error {
	r.syncCalls++
	return nil
}

func (r *recordingSubController) ClearResources(context.Context, *dorisv1.DorisCluster) (bool, error) {
	r.clearCalls++
	return true, nil
}

func (r *recordingSubController) GetControllerName() string { return "recording" }

func (r *recordingSubController) UpdateComponentStatus(*dorisv1.DorisCluster) error {
	r.statusCalls++
	return nil
}

func newTestReconciler(t *testing.T, dcr *dorisv1.DorisCluster, recorder *recordingSubController) *DorisClusterReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := dorisv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return &DorisClusterReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(dcr).Build(),
		Scs:    map[string]sub_controller.SubController{"recording": recorder},
	}
}

func TestReconcilePausedDorisCluster(t *testing.T) {
	for _, annotation := range []string{
		dorisv1.AnnotationReconcilePaused,
		dorisv1.AnnotationReconcilePausedLegacy,
	} {
		t.Run(annotation, func(t *testing.T) {
			dcr := &dorisv1.DorisCluster{ObjectMeta: metav1.ObjectMeta{
				Name: "paused", Namespace: "default",
				Annotations: map[string]string{annotation: "true"},
			}}
			recorder := &recordingSubController{}
			reconciler := newTestReconciler(t, dcr, recorder)
			result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: dcr.Name, Namespace: dcr.Namespace},
			})
			if err != nil || result.Requeue || result.RequeueAfter != 0 {
				t.Fatalf("Reconcile() result=%+v error=%v, want no requeue or error", result, err)
			}
			if recorder.syncCalls != 0 || recorder.clearCalls != 0 || recorder.statusCalls != 0 {
				t.Fatalf("paused cluster called subcontrollers: %+v", recorder)
			}
		})
	}
}

func TestPausedDorisClusterDeletionStillCleansResources(t *testing.T) {
	now := metav1.Now()
	dcr := &dorisv1.DorisCluster{ObjectMeta: metav1.ObjectMeta{
		Name: "deleting", Namespace: "default", DeletionTimestamp: &now,
		Finalizers:  []string{"test-finalizer"},
		Annotations: map[string]string{dorisv1.AnnotationReconcilePaused: "true"},
	}}
	recorder := &recordingSubController{}
	reconciler := newTestReconciler(t, dcr, recorder)
	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: dcr.Name, Namespace: dcr.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}
	if recorder.clearCalls != 1 || recorder.syncCalls != 0 || recorder.statusCalls != 0 {
		t.Fatalf("deleting paused cluster calls: %+v, want only one cleanup", recorder)
	}
}
