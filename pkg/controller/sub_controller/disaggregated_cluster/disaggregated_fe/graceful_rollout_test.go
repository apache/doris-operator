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

package disaggregated_fe

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type stubFESQLClient struct {
	frontends []*mysql.Frontend
	observers []*mysql.Frontend
	err       error
}

var errTestSQL = errors.New("sql unavailable")

func (s *stubFESQLClient) ShowFrontends() ([]*mysql.Frontend, error) {
	return s.frontends, s.err
}

func (s *stubFESQLClient) GetObservers() ([]*mysql.Frontend, error) {
	return s.observers, s.err
}

func (s *stubFESQLClient) Close() error { return nil }

func withStubFESQLClient(t *testing.T, stub *stubFESQLClient) {
	t.Helper()
	previous := newFEClusterSQLClient
	newFEClusterSQLClient = func(*DisaggregatedFEController, context.Context, *dv1.DorisDisaggregatedCluster) (feSQLClient, error) {
		if stub.err != nil {
			return nil, stub.err
		}
		return stub, nil
	}
	t.Cleanup(func() { newFEClusterSQLClient = previous })
}

func mustMarshalAction(t *testing.T, action *dv1.GracefulAction) string {
	t.Helper()
	raw, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func newTestFECluster(replicas int32) *dv1.DorisDisaggregatedCluster {
	return &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default", UID: "cluster-uid"},
		Spec: dv1.DorisDisaggregatedClusterSpec{
			FeSpec: dv1.FeSpec{
				ElectionNumber: ptr.To(replicas),
				CommonSpec: dv1.CommonSpec{
					Replicas: ptr.To(replicas),
					Image:    "fe:new",
				},
			},
		},
	}
}

func newTestFEStatefulset(replicas int32, revision string) *appv1.StatefulSet {
	return &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-fe", Namespace: "default"},
		Spec: appv1.StatefulSetSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-fe"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test-fe"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: resource.DISAGGREGATED_FE_MAIN_CONTAINER_NAME, Image: "fe:new"}},
				},
			},
			UpdateStrategy: appv1.StatefulSetUpdateStrategy{Type: appv1.OnDeleteStatefulSetStrategyType},
		},
		Status: appv1.StatefulSetStatus{
			UpdateRevision:  revision,
			CurrentRevision: revision,
		},
	}
}

func newTestFEPod(name, revision, uid string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			UID:       types.UID(uid),
			Labels:    map[string]string{"app": "test-fe", resource.POD_CONTROLLER_REVISION_HASH_KEY: revision},
		},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: resource.DISAGGREGATED_FE_MAIN_CONTAINER_NAME, Ready: true}},
			Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}
}

func frontendsFor(roles map[int]string, master int, alive func(int) bool, journal func(int) int64) []*mysql.Frontend {
	var result []*mysql.Frontend
	for ordinal, role := range roles {
		isAlive := true
		if alive != nil {
			isAlive = alive(ordinal)
		}
		replayed := int64(0)
		if journal != nil {
			replayed = journal(ordinal)
		}
		result = append(result, &mysql.Frontend{
			Host:              "test-fe-" + strconv.Itoa(ordinal) + ".test-fe-internal.default.svc.cluster.local",
			Role:              role,
			IsMaster:          ordinal == master,
			Alive:             isAlive,
			ReplayedJournalId: strconv.FormatInt(replayed, 10),
		})
	}
	return result
}

func TestSelectNextFEPod_RollsObserversThenFollowersThenMaster(t *testing.T) {
	cluster := newTestFECluster(3)
	action := &dv1.GracefulAction{}
	pods := []corev1.Pod{
		*newTestFEPod("test-fe-2", "old", "uid-2"),
		*newTestFEPod("test-fe-1", "old", "uid-1"),
		*newTestFEPod("test-fe-0", "old", "uid-0"),
	}

	frontends := []*mysql.Frontend{
		{Host: "test-fe-0.test-fe-internal.default.svc.cluster.local", Role: mysql.FE_OBSERVE_ROLE},
		{Host: "test-fe-1.test-fe-internal.default.svc.cluster.local", Role: mysql.FE_FOLLOWER_ROLE},
		{Host: "test-fe-2.test-fe-internal.default.svc.cluster.local", Role: mysql.FE_FOLLOWER_ROLE, IsMaster: true},
	}
	m, err := mysql.BuildSeqNumberToFrontendMap(frontends, nil, "test-fe")
	if err != nil {
		t.Fatal(err)
	}
	pod, _, found := (&DisaggregatedFEController{}).selectNextFEPod(pods, m, nil, action, cluster)
	if !found || pod.Name != "test-fe-0" {
		t.Fatalf("expected observer test-fe-0 first, got %q found=%v", pod.Name, found)
	}

	// Without the observer the master must be deferred to last.
	delete(m, 0)
	pod, _, found = (&DisaggregatedFEController{}).selectNextFEPod(pods, m, nil, action, cluster)
	if !found || pod.Name != "test-fe-1" {
		t.Fatalf("expected follower test-fe-1 before master, got %q found=%v", pod.Name, found)
	}

	pod, _, found = (&DisaggregatedFEController{}).selectNextFEPod([]corev1.Pod{*newTestFEPod("test-fe-2", "old", "uid-2")}, m, nil, action, cluster)
	if !found || pod.Name != "test-fe-2" {
		t.Fatalf("expected master test-fe-2 to be selected last, got %q found=%v", pod.Name, found)
	}
}

func TestSelectNextFEPod_DefersUnknownFrontends(t *testing.T) {
	cluster := newTestFECluster(3)
	action := &dv1.GracefulAction{}
	pods := []corev1.Pod{
		*newTestFEPod("test-fe-2", "old", "uid-2"),
		*newTestFEPod("test-fe-1", "old", "uid-1"),
		*newTestFEPod("test-fe-0", "old", "uid-0"),
	}
	m := map[int]*mysql.Frontend{
		0: {Host: "test-fe-0.test-fe-internal.default.svc.cluster.local", Role: mysql.FE_FOLLOWER_ROLE},
		1: {Host: "test-fe-1.test-fe-internal.default.svc.cluster.local", Role: mysql.FE_FOLLOWER_ROLE, IsMaster: true},
	}
	pod, _, found := (&DisaggregatedFEController{}).selectNextFEPod(pods, m, nil, action, cluster)
	if !found || pod.Name != "test-fe-0" {
		t.Fatalf("expected known follower test-fe-0 before unknown pod, got %q found=%v", pod.Name, found)
	}
}

func TestSelectNextFEPod_FallsBackToOrdinalWhenSQLLookbackExpires(t *testing.T) {
	cluster := newTestFECluster(3)
	pods := []corev1.Pod{
		*newTestFEPod("test-fe-2", "old", "uid-2"),
		*newTestFEPod("test-fe-1", "old", "uid-1"),
	}
	action := &dv1.GracefulAction{StartedAt: metav1.Now()}
	_, _, found := (&DisaggregatedFEController{}).selectNextFEPod(pods, nil, errTestSQL, action, cluster)
	if found {
		t.Fatal("expected rollout to wait before the SQL lookback expires")
	}

	action.StartedAt = metav1.NewTime(time.Now().Add(-feRolloutSQLLookback - time.Minute))
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8srecorder: record.NewFakeRecorder(10)}}
	pod, _, found := controller.selectNextFEPod(pods, nil, errTestSQL, action, cluster)
	if !found || pod.Name != "test-fe-2" {
		t.Fatalf("expected ordinal fallback to choose test-fe-2, got %q found=%v", pod.Name, found)
	}
}

func TestFeAliveAndCaughtUp(t *testing.T) {
	master := &mysql.Frontend{Role: mysql.FE_FOLLOWER_ROLE, IsMaster: true, Alive: true, ReplayedJournalId: "100"}
	followerBehind := &mysql.Frontend{Role: mysql.FE_FOLLOWER_ROLE, Alive: true, ReplayedJournalId: "90"}
	followerCaughtUp := &mysql.Frontend{Role: mysql.FE_FOLLOWER_ROLE, Alive: true, ReplayedJournalId: "100"}
	followerUnparseable := &mysql.Frontend{Role: mysql.FE_FOLLOWER_ROLE, Alive: true, ReplayedJournalId: "unknown"}

	frontends := map[int]*mysql.Frontend{0: followerBehind, 2: master}
	if alive, _ := feAliveAndCaughtUp(followerBehind, frontends); alive {
		t.Fatal("expected a follower behind the master journal to be rejected")
	}
	if alive, reason := feAliveAndCaughtUp(followerCaughtUp, map[int]*mysql.Frontend{2: master}); !alive {
		t.Fatalf("expected caught up follower to be accepted: %s", reason)
	}
	if alive, reason := feAliveAndCaughtUp(followerUnparseable, map[int]*mysql.Frontend{2: master}); !alive {
		t.Fatalf("expected journal parse failure to degrade to alive-only: %s", reason)
	}
	if alive, _ := feAliveAndCaughtUp(master, frontends); !alive {
		t.Fatal("expected master to be accepted without journal comparison")
	}
	notAlive := &mysql.Frontend{Role: mysql.FE_FOLLOWER_ROLE, Alive: false}
	if alive, _ := feAliveAndCaughtUp(notAlive, frontends); alive {
		t.Fatal("expected alive=false to be rejected")
	}
}

func TestFeQuorumPreserved(t *testing.T) {
	allAlive := map[int]*mysql.Frontend{
		0: {Role: mysql.FE_FOLLOWER_ROLE, Alive: true},
		1: {Role: mysql.FE_FOLLOWER_ROLE, Alive: true},
		2: {Role: mysql.FE_FOLLOWER_ROLE, Alive: true},
	}
	if !feQuorumPreserved(allAlive, 2, 3) {
		t.Fatal("expected quorum to be preserved with two remaining alive followers")
	}
	allAlive[0].Alive = false
	if feQuorumPreserved(allAlive, 2, 3) {
		t.Fatal("expected quorum loss to be detected")
	}
	if !feQuorumPreserved(nil, 0, 1) {
		t.Fatal("single-FE cluster should not be blocked by quorum")
	}
	if !feQuorumPreserved(nil, 0, 2) {
		t.Fatal("two-FE cluster has no fault tolerance and should not be blocked")
	}
	leaderAlive := map[int]*mysql.Frontend{
		0: {Role: "LEADER", Alive: true},
		1: {Role: mysql.FE_FOLLOWER_ROLE, Alive: true},
		2: {Role: mysql.FE_FOLLOWER_ROLE, Alive: true},
	}
	if !feQuorumPreserved(leaderAlive, 2, 3) {
		t.Fatal("expected a LEADER-role master to count towards quorum")
	}
}

func TestHandleFEDeletePod_DeletesFollowerBeforeMaster(t *testing.T) {
	cluster := newTestFECluster(3)
	sts := newTestFEStatefulset(3, "rev-new")
	pods := []*corev1.Pod{
		newTestFEPod("test-fe-0", "rev-old", "uid-0"),
		newTestFEPod("test-fe-1", "rev-old", "uid-1"),
		newTestFEPod("test-fe-2", "rev-old", "uid-2"),
	}
	client := fake.NewClientBuilder().WithObjects(pods[0], pods[1], pods[2]).Build()
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
		K8sclient:   client,
		K8srecorder: record.NewFakeRecorder(20),
	}}
	withStubFESQLClient(t, &stubFESQLClient{frontends: frontendsFor(map[int]string{
		0: mysql.FE_FOLLOWER_ROLE, 1: mysql.FE_FOLLOWER_ROLE, 2: mysql.FE_FOLLOWER_ROLE,
	}, 2, nil, nil)})

	action := &dv1.GracefulAction{Type: dv1.GracefulActionRollingUpdate, Phase: dv1.GracefulPhaseDeletePod, TargetRevision: "rev-new", StartedAt: metav1.Now()}
	done, err := controller.handleFEDeletePod(context.Background(), cluster, sts, action)
	if err != nil || done {
		t.Fatalf("unexpected result done=%v err=%v", done, err)
	}
	if action.CurrentPod != "test-fe-1" {
		t.Fatalf("expected follower test-fe-1 to be deleted before master, got %q", action.CurrentPod)
	}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "test-fe-2"}, &corev1.Pod{}); err != nil {
		t.Fatalf("master must not be deleted before followers: %v", err)
	}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "test-fe-1"}, &corev1.Pod{}); err == nil {
		t.Fatal("expected follower test-fe-1 to be deleted")
	}
}

func TestHandleFEDeletePod_DoneWhenNoOutdatedPods(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
		K8sclient:   fake.NewClientBuilder().WithScheme(scheme).Build(),
		K8srecorder: record.NewFakeRecorder(10),
	}}
	sts := newTestFEStatefulset(1, "rev-new")
	action := &dv1.GracefulAction{Type: dv1.GracefulActionRollingUpdate, Phase: dv1.GracefulPhaseDeletePod, TargetRevision: "rev-new"}
	done, err := controller.handleFEDeletePod(context.Background(), newTestFECluster(1), sts, action)
	if err != nil || !done {
		t.Fatalf("expected rollout completion, done=%v err=%v", done, err)
	}
}

func TestHandleFEDeletePod_AdoptsTerminatingPod(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	terminating := newTestFEPod("test-fe-1", "rev-old", "uid-1")
	now := metav1.Now()
	terminating.DeletionTimestamp = &now
	terminating.Finalizers = []string{"test/finalizer"}
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
		K8sclient:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(terminating).Build(),
		K8srecorder: record.NewFakeRecorder(10),
	}}
	sts := newTestFEStatefulset(3, "rev-new")
	action := &dv1.GracefulAction{Type: dv1.GracefulActionRollingUpdate, Phase: dv1.GracefulPhaseDeletePod, TargetRevision: "rev-new", StartedAt: metav1.Now()}
	done, err := controller.handleFEDeletePod(context.Background(), newTestFECluster(3), sts, action)
	if err != nil || done {
		t.Fatalf("unexpected result done=%v err=%v", done, err)
	}
	if action.Phase != dv1.GracefulPhaseWaitPodReady || action.CurrentPod != "test-fe-1" || action.InitialPodUID != "uid-1" {
		t.Fatalf("expected to adopt the terminating pod, phase=%s pod=%q uid=%q", action.Phase, action.CurrentPod, action.InitialPodUID)
	}
}

func TestHandleFEWaitPodReady_TransitionsToWaitFEAlive(t *testing.T) {
	cluster := newTestFECluster(3)
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8srecorder: record.NewFakeRecorder(10)}}
	pod := newTestFEPod("test-fe-1", "rev-new", "uid-new")
	client := fake.NewClientBuilder().WithObjects(pod).Build()
	controller.K8sclient = client

	action := &dv1.GracefulAction{
		Type:           dv1.GracefulActionRollingUpdate,
		Phase:          dv1.GracefulPhaseWaitPodReady,
		CurrentPod:     "test-fe-1",
		CurrentOrdinal: 1,
		InitialPodUID:  "uid-old",
		TargetRevision: "rev-new",
		DeadlineAt:     metav1.NewTime(time.Now().Add(time.Minute)),
	}
	if _, err := controller.handleFEWaitPodReady(context.Background(), cluster, action); err != nil {
		t.Fatal(err)
	}
	if action.Phase != dv1.GracefulPhaseWaitFEAlive {
		t.Fatalf("expected WaitFEAlive, got %s", action.Phase)
	}
	if action.ReplacementPodUID != "uid-new" {
		t.Fatalf("expected replacement pod uid, got %q", action.ReplacementPodUID)
	}
}

func TestHandleFEWaitFEAlive_RequiresStableObservations(t *testing.T) {
	cluster := newTestFECluster(3)
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8srecorder: record.NewFakeRecorder(10)}}
	frontends := frontendsFor(map[int]string{
		0: mysql.FE_FOLLOWER_ROLE, 1: mysql.FE_FOLLOWER_ROLE, 2: mysql.FE_FOLLOWER_ROLE,
	}, 2, nil, func(int) int64 { return 100 })
	withStubFESQLClient(t, &stubFESQLClient{frontends: frontends})

	action := &dv1.GracefulAction{
		Type:           dv1.GracefulActionRollingUpdate,
		Phase:          dv1.GracefulPhaseWaitFEAlive,
		CurrentPod:     "test-fe-1",
		CurrentOrdinal: 1,
		TargetRevision: "rev-new",
		DeadlineAt:     metav1.NewTime(time.Now().Add(time.Minute)),
	}
	if _, err := controller.handleFEWaitFEAlive(context.Background(), cluster, action); err != nil {
		t.Fatal(err)
	}
	if action.Phase != dv1.GracefulPhaseWaitFEAlive || action.StableBackendObservations != 1 {
		t.Fatalf("expected first stable observation, phase=%s observations=%d", action.Phase, action.StableBackendObservations)
	}
	if _, err := controller.handleFEWaitFEAlive(context.Background(), cluster, action); err != nil {
		t.Fatal(err)
	}
	if action.Phase != dv1.GracefulPhaseDeletePod || action.CurrentPod != "" {
		t.Fatalf("expected rollout to advance after stable observations, phase=%s pod=%q", action.Phase, action.CurrentPod)
	}
}

func TestHandleFEWaitFEAlive_TimeoutAdvances(t *testing.T) {
	cluster := newTestFECluster(3)
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8srecorder: record.NewFakeRecorder(10)}}
	withStubFESQLClient(t, &stubFESQLClient{frontends: frontendsFor(map[int]string{1: mysql.FE_FOLLOWER_ROLE}, 0, func(int) bool { return false }, nil)})

	action := &dv1.GracefulAction{
		Type:           dv1.GracefulActionRollingUpdate,
		Phase:          dv1.GracefulPhaseWaitFEAlive,
		CurrentPod:     "test-fe-1",
		CurrentOrdinal: 1,
		TargetRevision: "rev-new",
		DeadlineAt:     metav1.NewTime(time.Now().Add(-time.Minute)),
	}
	if _, err := controller.handleFEWaitFEAlive(context.Background(), cluster, action); err != nil {
		t.Fatal(err)
	}
	if action.Phase != dv1.GracefulPhaseDeletePod || action.CurrentPod != "" {
		t.Fatalf("expected timeout to advance rollout, phase=%s pod=%q", action.Phase, action.CurrentPod)
	}
}

func TestReconcileGracefulStatefulSet_StartsOnTemplateChange(t *testing.T) {
	cluster := newTestFECluster(3)
	desired := newTestFEStatefulset(3, "rev-new")
	desired.Status = appv1.StatefulSetStatus{}
	live := newTestFEStatefulset(3, "rev-old")
	live.Spec.Template.Spec.Containers[0].Image = "fe:old"
	live.Status.UpdateRevision = "rev-old"
	live.Status.CurrentRevision = "rev-old"

	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8srecorder: record.NewFakeRecorder(10)}}
	if pending := controller.reconcileGracefulStatefulSet(context.Background(), desired, live, cluster); !pending {
		t.Fatal("expected rollout to start and request another reconcile")
	}
	action, err := getFEGracefulAction(desired)
	if err != nil || action == nil {
		t.Fatalf("expected rollout annotation, action=%v err=%v", action, err)
	}
	if action.Phase != dv1.GracefulPhaseDeletePod {
		t.Fatalf("expected DeletePod phase, got %s", action.Phase)
	}
}

func TestReconcileGracefulStatefulSet_AbortsWhenReplicasChange(t *testing.T) {
	cluster := newTestFECluster(3)
	desired := newTestFEStatefulset(2, "rev-new")
	live := newTestFEStatefulset(3, "rev-old")
	live.Spec.Template.Spec.Containers[0].Image = "fe:old"
	live.Annotations = map[string]string{sc.GracefulActionAnnotation: `{"type":"RollingUpdate","phase":"DeletePod"}`}

	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live).Build()
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8sclient: client, K8srecorder: record.NewFakeRecorder(10)}}
	if pending := controller.reconcileGracefulStatefulSet(context.Background(), desired, live, cluster); pending {
		t.Fatal("scale changes must not be owned by the FE rollout")
	}
	if hasFEGracefulAction(desired) {
		t.Fatal("expected rollout annotation to be cleared on replica change")
	}
	var stored appv1.StatefulSet
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: live.Name}, &stored); err != nil {
		t.Fatal(err)
	}
	if hasFEGracefulAction(&stored) {
		t.Fatal("expected rollout annotation to be removed from the live statefulset")
	}
}

func TestReconcileGracefulStatefulSet_RetargetsChangedRevision(t *testing.T) {
	cluster := newTestFECluster(3)
	desired := newTestFEStatefulset(3, "rev-newer")
	live := newTestFEStatefulset(3, "rev-newest")
	action := &dv1.GracefulAction{
		Type:           dv1.GracefulActionRollingUpdate,
		Phase:          dv1.GracefulPhaseWaitFEAlive,
		CurrentPod:     "test-fe-1",
		CurrentOrdinal: 1,
		TargetRevision: "rev-old",
		StartedAt:      metav1.Now(),
	}
	live.Annotations = map[string]string{sc.GracefulActionAnnotation: mustMarshalAction(t, action)}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	outdated := newTestFEPod("test-fe-1", "rev-old", "uid-1")
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
		K8sclient:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(outdated).Build(),
		K8srecorder: record.NewFakeRecorder(10),
	}}
	withStubFESQLClient(t, &stubFESQLClient{err: errTestSQL})
	if pending := controller.reconcileGracefulStatefulSet(context.Background(), desired, live, cluster); !pending {
		t.Fatal("expected rollout to remain pending after retargeting")
	}
	stored, err := getFEGracefulAction(desired)
	if err != nil || stored == nil {
		t.Fatalf("expected rollout annotation, action=%v err=%v", stored, err)
	}
	if stored.TargetRevision != "rev-newest" || stored.Phase != dv1.GracefulPhaseDeletePod || stored.CurrentPod != "" {
		t.Fatalf("expected retargeted rollout, got target=%s phase=%s pod=%q", stored.TargetRevision, stored.Phase, stored.CurrentPod)
	}
}

func TestUpdateComponentStatus_KeepsReconcilingDuringRollout(t *testing.T) {
	cluster := newTestFECluster(3)
	sts := newTestFEStatefulset(3, "rev-new")
	sts.Annotations = map[string]string{sc.GracefulActionAnnotation: `{"type":"RollingUpdate","phase":"WaitFEAlive"}`}
	pods := []*corev1.Pod{
		newTestFEPod("test-fe-0", "rev-new", "uid-0"),
		newTestFEPod("test-fe-1", "rev-new", "uid-1"),
		newTestFEPod("test-fe-2", "rev-new", "uid-2"),
	}
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts, pods[0], pods[1], pods[2]).Build()
	controller := &DisaggregatedFEController{DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{K8sclient: client, K8srecorder: record.NewFakeRecorder(10)}}
	cluster.Status.FEStatus.Phase = dv1.Reconciling
	if err := controller.UpdateComponentStatus(cluster); err != nil {
		t.Fatal(err)
	}
	if cluster.Status.FEStatus.Phase != dv1.Reconciling {
		t.Fatalf("expected FE to stay Reconciling during rollout, got %s", cluster.Status.FEStatus.Phase)
	}
}

func TestExtractOrdinal(t *testing.T) {
	if got := extractOrdinal("test-fe-2"); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
	if got := extractOrdinal("test-fe"); got != -1 {
		t.Fatalf("expected -1 for a name without ordinal, got %d", got)
	}
}
