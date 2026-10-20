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

package computegroups

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	ms "github.com/apache/doris-operator/pkg/common/utils/metaservice"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/jmoiron/sqlx"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type decommissionSQLRecorder struct {
	decommissionCalls, dropCalls   int
	decommissionErr, dropErr       error
	beforeDecommission, beforeDrop func()
}

func (r *decommissionSQLRecorder) DecommissionBE([]*mysql.Backend) error {
	if r.beforeDecommission != nil {
		r.beforeDecommission()
	}
	r.decommissionCalls++
	return r.decommissionErr
}
func (r *decommissionSQLRecorder) DropBE([]*mysql.Backend) error {
	if r.beforeDrop != nil {
		r.beforeDrop()
	}
	r.dropCalls++
	return r.dropErr
}

func decommissionFixture() (*decommissionAction, *decommissionSnapshot) {
	backend := &mysql.Backend{BackendID: "101", Host: "doris-cg-1.example", HeartbeatPort: 9050, Tag: `{"cloud_unique_id":"be-1"}`}
	action := &decommissionAction{ComputeGroupID: "cg", FromReplicas: 2, Replicas: 1, CloudUniqueID: "1:instance:operator", InstanceID: "instance", Targets: []decommissionTarget{{Ordinal: 1, Phase: decommissionPrepared}}}
	view := map[int32]*mysql.Backend{1: backend}
	snapshot := &decommissionSnapshot{Nodes: map[int32]ms.Node{1: {CloudUniqueID: "be-1", Host: backend.Host, HeartbeatPort: 9050, Status: ms.Running}}, Backends: view, FEViews: []map[int32]*mysql.Backend{view}, SyncWait: 20 * time.Second}
	return action, snapshot
}

func setMSState(snapshot *decommissionSnapshot, state ms.NodeStatus) {
	node := snapshot.Nodes[1]
	node.Status = state
	snapshot.Nodes[1] = node
}

func TestDecommissionPersistsBeforeSQLAndDoesNotResubmitAfterRestart(t *testing.T) {
	action, snapshot := decommissionFixture()
	var persisted []byte
	save := func() error { var err error; persisted, err = json.Marshal(action); return err }
	sql := &decommissionSQLRecorder{beforeDecommission: func() {
		var saved decommissionAction
		if err := json.Unmarshal(persisted, &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Targets[0].Phase != decommissionSubmitted || saved.Targets[0].BackendID != "101" {
			t.Fatalf("SQL preceded durable intent: %+v", saved)
		}
	}}
	if done, err := advanceDecommission(action, snapshot, time.Now(), sql, save); done || err != nil {
		t.Fatalf("done=%v err=%v", done, err)
	}
	restarted := new(decommissionAction)
	if err := json.Unmarshal(persisted, restarted); err != nil {
		t.Fatal(err)
	}
	if done, err := advanceDecommission(restarted, snapshot, time.Now(), sql, func() error { return nil }); done || err != nil {
		t.Fatalf("done=%v err=%v", done, err)
	}
	if sql.decommissionCalls != 1 || sql.dropCalls != 0 {
		t.Fatalf("SQL calls=%+v", sql)
	}
}

func TestDecommissionAdoptsManualSQLAcrossKernelVersions(t *testing.T) {
	for _, tt := range []struct {
		name      string
		state     ms.NodeStatus
		feStarted bool
	}{
		{"3.x manual SQL before MS update", ms.Running, true},
		{"early 4.0 manual SQL before MS update", ms.Running, true},
		{"new 4.x manual SQL", ms.Decommissioning, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			action, snapshot := decommissionFixture()
			setMSState(snapshot, tt.state)
			snapshot.Backends[1].SystemDecommissioned = tt.feStarted
			sql := &decommissionSQLRecorder{}
			done, err := advanceDecommission(action, snapshot, time.Now(), sql, func() error { return nil })
			if err != nil || done || sql.decommissionCalls != 0 || sql.dropCalls != 0 || action.Targets[0].Phase != decommissionSubmitted {
				t.Fatalf("done=%v err=%v action=%+v SQL=%+v", done, err, action, sql)
			}
		})
	}
}

func TestDecommissionWaitsForMSCompletionAndFESync(t *testing.T) {
	action, snapshot := decommissionFixture()
	sql := &decommissionSQLRecorder{}
	now := time.Now().Truncate(time.Second)
	snapshot.Backends[1].SystemDecommissioned = true
	snapshot.Backends[1].TabletNum = 0
	if _, err := advanceDecommission(action, snapshot, now, sql, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if sql.dropCalls != 0 {
		t.Fatal("legacy FE true with zero tablets is not MS completion")
	}
	setMSState(snapshot, ms.Decommissioned)
	// A completed MS state is authoritative even if SHOW BACKENDS still reports tablets.
	snapshot.Backends[1].TabletNum = 100
	if _, err := advanceDecommission(action, snapshot, now, sql, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, elapsed := range []time.Duration{0, 10 * time.Second, 19 * time.Second} {
		if _, err := advanceDecommission(action, snapshot, now.Add(elapsed), sql, func() error { return nil }); err != nil {
			t.Fatal(err)
		}
		if sql.dropCalls != 0 {
			t.Fatal("DROP sent before FE sync delay")
		}
	}
	stale := *snapshot.Backends[1]
	stale.SystemDecommissioned = false
	snapshot.FEViews = append(snapshot.FEViews, map[int32]*mysql.Backend{1: &stale})
	if _, err := advanceDecommission(action, snapshot, now.Add(time.Minute), sql, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if sql.dropCalls != 0 {
		t.Fatal("DROP sent while an FE still has an unsynchronized state")
	}
	stale.SystemDecommissioned = true
	sql.beforeDrop = func() {
		if action.Targets[0].Phase != decommissionDropping {
			t.Fatal("DROP intent not persisted")
		}
	}
	if _, err := advanceDecommission(action, snapshot, now.Add(time.Minute), sql, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if sql.dropCalls != 1 {
		t.Fatal("synchronized completed target was not dropped")
	}
	delete(snapshot.Nodes, 1)
	if done, err := advanceDecommission(action, snapshot, now.Add(2*time.Minute), sql, func() error { return nil }); err != nil || done {
		t.Fatalf("FE still has target: done=%v err=%v", done, err)
	}
	delete(snapshot.Backends, 1)
	if done, err := advanceDecommission(action, snapshot, now.Add(2*time.Minute), sql, func() error { return nil }); err != nil || done {
		t.Fatalf("observer still has target: done=%v err=%v", done, err)
	}
	delete(snapshot.FEViews[1], 1)
	if done, err := advanceDecommission(action, snapshot, now.Add(2*time.Minute), sql, func() error { return nil }); err != nil || done {
		t.Fatalf("first missing observation must wait: done=%v err=%v", done, err)
	}
	if done, err := advanceDecommission(action, snapshot, now.Add(3*time.Minute), sql, func() error { return nil }); err != nil || !done {
		t.Fatalf("stable absence: done=%v err=%v", done, err)
	}
}

func TestDecommissionBlocksUnsafeTransitions(t *testing.T) {
	tests := []struct {
		name            string
		prepare         func(*decommissionAction, *decommissionSnapshot)
		sqlErr, saveErr bool
	}{
		{name: "unknown MS state", prepare: func(a *decommissionAction, s *decommissionSnapshot) { setMSState(s, "NODE_STATUS_UNKNOWN") }},
		{name: "MS identity replaced", prepare: func(a *decommissionAction, s *decommissionSnapshot) { a.Targets[0].CloudUniqueID = "old" }},
		{name: "FE identity replaced", prepare: func(a *decommissionAction, s *decommissionSnapshot) { a.Targets[0].BackendID = "old" }},
		{name: "FE and MS disagree", prepare: func(a *decommissionAction, s *decommissionSnapshot) {
			s.Backends[1].Tag = `{"cloud_unique_id":"other"}`
		}},
		{name: "MS completion regressed", prepare: func(a *decommissionAction, s *decommissionSnapshot) {
			now := metav1.Now()
			a.Targets[0].CompletedAt = &now
		}},
		{name: "no FE observations", prepare: func(a *decommissionAction, s *decommissionSnapshot) { s.FEViews = nil }},
		{name: "persistence failed", saveErr: true},
		{name: "SQL submission uncertain", sqlErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, snapshot := decommissionFixture()
			if tt.prepare != nil {
				tt.prepare(action, snapshot)
			}
			sql := &decommissionSQLRecorder{}
			if tt.sqlErr {
				sql.decommissionErr = errors.New("connection lost")
			}
			save := func() error {
				if tt.saveErr {
					return errors.New("conflict")
				}
				return nil
			}
			done, err := advanceDecommission(action, snapshot, time.Now(), sql, save)
			if err == nil || done || sql.dropCalls != 0 {
				t.Fatalf("unsafe transition: done=%v err=%v SQL=%+v", done, err, sql)
			}
			if tt.saveErr && sql.decommissionCalls != 0 {
				t.Fatal("SQL sent after persistence failure")
			}
			if tt.sqlErr {
				sql.decommissionErr = nil
				if _, err := advanceDecommission(action, snapshot, time.Now(), sql, save); err != nil {
					t.Fatal(err)
				}
				if sql.decommissionCalls != 1 {
					t.Fatal("uncertain SQL was retried")
				}
			}
		})
	}
}

func TestDecommissionDropFailureBlocksScaleDown(t *testing.T) {
	action, snapshot := decommissionFixture()
	setMSState(snapshot, ms.Decommissioned)
	now := time.Now()
	completedAt := metav1.NewTime(now.Add(-time.Minute))
	action.Targets[0].CompletedAt = &completedAt
	action.Targets[0].Phase = decommissionWaitFESync
	snapshot.Backends[1].SystemDecommissioned = true
	sql := &decommissionSQLRecorder{dropErr: errors.New("DROP failed")}
	if done, err := advanceDecommission(action, snapshot, now, sql, func() error { return nil }); err == nil || done {
		t.Fatalf("done=%v err=%v", done, err)
	}
	// A lost DROP response can be recovered by observing absence, without SQL.
	delete(snapshot.Nodes, 1)
	delete(snapshot.Backends, 1)
	if done, err := advanceDecommission(action, snapshot, now, sql, func() error { return nil }); err != nil || done {
		t.Fatalf("first absence: done=%v err=%v", done, err)
	}
	if done, err := advanceDecommission(action, snapshot, now.Add(time.Minute), sql, func() error { return nil }); err != nil || !done || sql.dropCalls != 1 {
		t.Fatalf("confirmed absence: done=%v err=%v calls=%d", done, err, sql.dropCalls)
	}
}

func TestDecommissionPartialCompletionDoesNotTouchRetainedNodes(t *testing.T) {
	action, snapshot := decommissionFixture()
	action.FromReplicas = 3
	action.Targets = append([]decommissionTarget{{Ordinal: 2, Phase: decommissionPrepared}}, action.Targets...)
	snapshot.Nodes[0] = ms.Node{CloudUniqueID: "retained", Status: ms.Decommissioned}
	snapshot.Nodes[2] = ms.Node{CloudUniqueID: "be-2", Status: ms.Decommissioning}
	snapshot.Backends[0] = &mysql.Backend{BackendID: "100", Host: "doris-cg-0", HeartbeatPort: 9050, SystemDecommissioned: true}
	sql := &decommissionSQLRecorder{}
	if done, err := advanceDecommission(action, snapshot, time.Now(), sql, func() error { return nil }); err != nil || done {
		t.Fatalf("done=%v err=%v", done, err)
	}
	if sql.decommissionCalls != 1 || sql.dropCalls != 0 {
		t.Fatalf("SQL=%+v", sql)
	}
	delete(snapshot.Nodes, 2)
	if done, err := advanceDecommission(action, snapshot, time.Now(), sql, func() error { return nil }); err != nil || done {
		t.Fatalf("another target is pending: done=%v err=%v", done, err)
	}
}

func TestDecommissionAnnotationSurvivesRestartAndPreservesOtherMetadata(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	replicas := int32(2)
	sts := &appv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "doris-cg", Namespace: "default", Annotations: map[string]string{"other": "keep"}}, Spec: appv1.StatefulSetSpec{Replicas: &replicas}}
	controller := &DisaggregatedComputeGroupsController{}
	controller.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()
	key := types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}
	if err := controller.K8sclient.Get(context.Background(), key, sts); err != nil {
		t.Fatal(err)
	}
	action, _ := decommissionFixture()
	if err := controller.saveDecommissionAction(context.Background(), sts, action); err != nil {
		t.Fatal(err)
	}
	var restored appv1.StatefulSet
	if err := controller.K8sclient.Get(context.Background(), key, &restored); err != nil {
		t.Fatal(err)
	}
	var saved decommissionAction
	if err := json.Unmarshal([]byte(restored.Annotations[decommissionActionAnnotation]), &saved); err != nil {
		t.Fatal(err)
	}
	if err := saved.validate(1, 2, "cg"); err != nil {
		t.Fatal(err)
	}
	if restored.Annotations["other"] != "keep" || *restored.Spec.Replicas != 2 {
		t.Fatalf("unexpected resource mutation: %+v", restored)
	}
	if err := saved.validate(2, 2, "cg"); err == nil {
		t.Fatal("changed target was accepted")
	}
	if err := saved.validate(1, 2, "other"); err == nil {
		t.Fatal("changed compute group was accepted")
	}
	saved.Targets = nil
	if err := saved.validate(1, 2, "cg"); err == nil {
		t.Fatal("truncated task was accepted")
	}
	if err := controller.finishDecommissionAction(context.Background(), &restored, 1); err != nil {
		t.Fatal(err)
	}
	var finished appv1.StatefulSet
	if err := controller.K8sclient.Get(context.Background(), key, &finished); err != nil {
		t.Fatal(err)
	}
	if *finished.Spec.Replicas != 1 || finished.Annotations[decommissionActionAnnotation] != "" || finished.Annotations["other"] != "keep" {
		t.Fatalf("unexpected completed resource: %+v", finished)
	}
}

func TestDecommissionOrdinalMapping(t *testing.T) {
	for _, host := range []string{"doris-cg-12", "doris-cg-12.doris-cg.default.svc.cluster.local", "10.0.0.12"} {
		ordinal, err := decommissionOrdinal("doris-cg", host, map[string]int32{"10.0.0.12": 12})
		if err != nil || ordinal != 12 {
			t.Fatalf("host=%s ordinal=%d err=%v", host, ordinal, err)
		}
	}
	for _, host := range []string{"other-cg-12.example", "doris-cg-x", "doris-cg--1", "10.0.0.13"} {
		if _, err := decommissionOrdinal("doris-cg", host, nil); err == nil {
			t.Fatalf("unrecognized host %s accepted", host)
		}
	}
}

func TestFrontendSyncIntervalAcrossKernelVersions(t *testing.T) {
	for _, tt := range []struct {
		name, value    string
		empty, wantErr bool
		want           time.Duration
	}{
		{name: "3.x constant", empty: true, want: 10 * time.Second},
		{name: "4.x default", value: "10", want: 10 * time.Second},
		{name: "4.x SQL override", value: "45", want: 45 * time.Second},
		{name: "invalid", value: "bad", wantErr: true},
		{name: "nonpositive", value: "0", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows := sqlmock.NewRows([]string{"Key", "Value", "Type", "IsMutable"})
			if !tt.empty {
				rows.AddRow("cloud_cluster_check_interval_second", tt.value, "int", true)
			}
			mock.ExpectQuery(regexp.QuoteMeta("ADMIN SHOW FRONTEND CONFIG LIKE 'cloud_cluster_check_interval_second'")).WillReturnRows(rows)
			interval, err := frontendSyncInterval(&mysql.DB{DB: sqlx.NewDb(db, "mysql")})
			if (err != nil) != tt.wantErr || interval != tt.want {
				t.Fatalf("interval=%v err=%v", interval, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDecommissionConflictDoesNotChangeReplicas(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	replicas := int32(2)
	sts := &appv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "doris-cg", Namespace: "default"}, Spec: appv1.StatefulSetSpec{Replicas: &replicas}}
	controller := &DisaggregatedComputeGroupsController{}
	controller.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()
	key := types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}
	if err := controller.K8sclient.Get(context.Background(), key, sts); err != nil {
		t.Fatal(err)
	}
	stale := sts.DeepCopy()
	action, _ := decommissionFixture()
	if err := controller.saveDecommissionAction(context.Background(), sts, action); err != nil {
		t.Fatal(err)
	}
	if err := controller.finishDecommissionAction(context.Background(), stale, 1); err == nil {
		t.Fatal("stale replica patch succeeded")
	}
	var actual appv1.StatefulSet
	if err := controller.K8sclient.Get(context.Background(), key, &actual); err != nil {
		t.Fatal(err)
	}
	if *actual.Spec.Replicas != 2 || actual.Annotations[decommissionActionAnnotation] == "" {
		t.Fatal("conflicting patch changed replica count or lost task")
	}
}

func TestReconcileStatefulsetBlocksDisablingActiveDecommission(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	ddc := newTestDDC()
	cg := newTestCG("cg1")
	ddc.Status.ComputeGroupStatuses = []dv1.ComputeGroupStatus{{UniqueId: cg.UniqueId, ComputeGroupId: "cg"}}
	existing := newTestStatefulSet(ddc.Namespace, ddc.GetCGStatefulsetName(cg), "100Gi")
	desired := existing.DeepCopy()
	current, target := int32(2), int32(1)
	existing.Spec.Replicas = &current
	desired.Spec.Replicas = &target
	existing.Annotations = map[string]string{decommissionActionAnnotation: `{"active":true}`}
	controller := &DisaggregatedComputeGroupsController{}
	controller.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	event, err := controller.reconcileStatefulset(context.Background(), desired, ddc, cg)
	if err == nil || event == nil {
		t.Fatal("active task bypassed by disabling decommission")
	}
	var actual appv1.StatefulSet
	if err := controller.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: existing.Namespace, Name: existing.Name}, &actual); err != nil {
		t.Fatal(err)
	}
	if *actual.Spec.Replicas != 2 || actual.Annotations[decommissionActionAnnotation] == "" {
		t.Fatal("blocked task changed StatefulSet")
	}
}

func TestDecommissionTransientMSRemovalDoesNotShrinkOrResubmit(t *testing.T) {
	action, snapshot := decommissionFixture()
	action.Targets[0].Phase = decommissionSubmitted
	now := time.Now()
	node, backend := snapshot.Nodes[1], snapshot.Backends[1]
	delete(snapshot.Nodes, 1)
	delete(snapshot.Backends, 1)
	sql := &decommissionSQLRecorder{}
	if done, err := advanceDecommission(action, snapshot, now, sql, func() error { return nil }); err != nil || done {
		t.Fatalf("transient absence: done=%v err=%v", done, err)
	}
	snapshot.Nodes[1] = node
	snapshot.Backends[1] = backend
	if done, err := advanceDecommission(action, snapshot, now.Add(10*time.Second), sql, func() error { return nil }); err != nil || done {
		t.Fatalf("reappeared: done=%v err=%v", done, err)
	}
	if action.Targets[0].AbsentSince != nil || sql.decommissionCalls != 0 || sql.dropCalls != 0 {
		t.Fatal("transient MS removal advanced the task or repeated SQL")
	}
}
