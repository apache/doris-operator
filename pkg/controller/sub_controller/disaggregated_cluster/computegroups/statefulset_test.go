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
	"strings"
	"testing"
	"time"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_NewPodTemplateSpec_TerminationGracePeriodSeconds(t *testing.T) {
	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ddc",
			Namespace: "default",
		},
	}
	cg := &dv1.ComputeGroup{
		UniqueId: "cg1",
		CommonSpec: dv1.CommonSpec{
			Replicas: pointer.Int32(1),
			Image:    "selectdb/doris.be-ubuntu:latest",
		},
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	pts := dcgs.NewPodTemplateSpec(ddc, map[string]string{}, map[string]interface{}{}, cg)
	if pts.Spec.TerminationGracePeriodSeconds == nil {
		t.Fatalf("expected BE terminationGracePeriodSeconds")
	}
	if *pts.Spec.TerminationGracePeriodSeconds != resource.DEFAULT_BE_TERMINATION_GRACE_PERIOD_SECONDS {
		t.Errorf("expected BE terminationGracePeriodSeconds=%d, got %d", resource.DEFAULT_BE_TERMINATION_GRACE_PERIOD_SECONDS, *pts.Spec.TerminationGracePeriodSeconds)
	}
	foundRuntimeVolume := false
	for _, v := range pts.Spec.Volumes {
		if v.Name == gracefulRuntimeVolumeName && v.EmptyDir != nil {
			foundRuntimeVolume = true
			break
		}
	}
	if !foundRuntimeVolume {
		t.Fatalf("expected pod template to include emptyDir volume %q", gracefulRuntimeVolumeName)
	}
	foundRuntimeMount := false
	for _, c := range pts.Spec.Containers {
		if c.Name != resource.DISAGGREGATED_BE_MAIN_CONTAINER_NAME {
			continue
		}
		for _, vm := range c.VolumeMounts {
			if vm.Name == gracefulRuntimeVolumeName && vm.MountPath == gracefulRuntimeMountPath {
				foundRuntimeMount = true
				break
			}
		}
	}
	if !foundRuntimeMount {
		t.Fatalf("expected compute container to mount %q at %q", gracefulRuntimeVolumeName, gracefulRuntimeMountPath)
	}
}

func Test_newSpecificEnvs_AlwaysUseFQDNHostType(t *testing.T) {
	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ddc",
			Namespace: "default",
		},
	}
	cg := &dv1.ComputeGroup{
		UniqueId: "cg1",
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	envs := dcgs.newSpecificEnvs(ddc, cg)

	if got := findEnvValue(envs, "HOST_TYPE"); got != resource.START_MODEL_FQDN {
		t.Fatalf("expected HOST_TYPE=%s, got %s", resource.START_MODEL_FQDN, got)
	}
}

func findEnvValue(envs []corev1.EnvVar, name string) string {
	for _, env := range envs {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}

func TestUpdateCGStatus_KeepGracefulPhaseWhenAnnotationExists(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appv1 scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	replicas := int32(3)
	sts := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris-cg1",
			Namespace: "default",
			Annotations: map[string]string{
				gracefulActionAnnotation: `{"type":"RollingUpdate","phase":"WaitBEAlive"}`,
			},
		},
		Spec: appv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					dv1.DorisDisaggregatedClusterName:          "doris",
					dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
					dv1.DorisDisaggregatedPodType:              "compute",
				},
			},
		},
		Status: appv1.StatefulSetStatus{
			UpdateRevision: "rev-new",
		},
	}

	pods := []runtime.Object{
		sts,
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "doris-cg1-0",
				Namespace: "default",
				Labels: map[string]string{
					dv1.DorisDisaggregatedClusterName:          "doris",
					dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
					dv1.DorisDisaggregatedPodType:              "compute",
					resource.POD_CONTROLLER_REVISION_HASH_KEY:  "rev-new",
				},
			},
			Status: corev1.PodStatus{
				Phase:             corev1.PodRunning,
				Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
				ContainerStatuses: []corev1.ContainerStatus{{Name: "compute", Ready: true}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "doris-cg1-1",
				Namespace: "default",
				Labels: map[string]string{
					dv1.DorisDisaggregatedClusterName:          "doris",
					dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
					dv1.DorisDisaggregatedPodType:              "compute",
					resource.POD_CONTROLLER_REVISION_HASH_KEY:  "rev-new",
				},
			},
			Status: corev1.PodStatus{
				Phase:             corev1.PodRunning,
				Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
				ContainerStatuses: []corev1.ContainerStatus{{Name: "compute", Ready: true}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "doris-cg1-2",
				Namespace: "default",
				Labels: map[string]string{
					dv1.DorisDisaggregatedClusterName:          "doris",
					dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
					dv1.DorisDisaggregatedPodType:              "compute",
					resource.POD_CONTROLLER_REVISION_HASH_KEY:  "rev-new",
				},
			},
			Status: corev1.PodStatus{
				Phase:             corev1.PodRunning,
				Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
				ContainerStatuses: []corev1.ContainerStatus{{Name: "compute", Ready: true}},
			},
		},
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	dcgs.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pods...).Build()

	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "doris",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
	}
	cgs := &dv1.ComputeGroupStatus{
		UniqueId:        "cg1",
		StatefulsetName: "doris-cg1",
		Replicas:        replicas,
		Phase:           dv1.Ready,
	}

	if err := dcgs.updateCGStatus(ddc, cgs); err != nil {
		t.Fatalf("updateCGStatus failed: %v", err)
	}
	if cgs.Phase != dv1.GracefulRolling {
		t.Fatalf("expected phase %s, got %s", dv1.GracefulRolling, cgs.Phase)
	}
	if cgs.AvailableReplicas != replicas {
		t.Fatalf("expected available replicas %d, got %d", replicas, cgs.AvailableReplicas)
	}
}

func TestNewStatefulset_UsesOnDeleteStrategyForComputeGroup(t *testing.T) {
	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris",
			Namespace: "default",
		},
	}
	replicas := int32(3)
	cg := &dv1.ComputeGroup{
		UniqueId: "cg1",
		CommonSpec: dv1.CommonSpec{
			Replicas: &replicas,
			Image:    "selectdb/doris.be-ubuntu:latest",
		},
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	st := dcgs.NewStatefulset(ddc, cg, map[string]interface{}{})

	if st.Spec.UpdateStrategy.Type != appv1.OnDeleteStatefulSetStrategyType {
		t.Fatalf("expected update strategy %s, got %s", appv1.OnDeleteStatefulSetStrategyType, st.Spec.UpdateStrategy.Type)
	}
	if st.Spec.UpdateStrategy.RollingUpdate != nil {
		t.Fatalf("expected rollingUpdate to be nil, got %#v", st.Spec.UpdateStrategy.RollingUpdate)
	}
}

func TestFinalizeGracefulAction_KeepsOnDeleteStrategy(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appv1 scheme: %v", err)
	}

	sts := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris-cg1",
			Namespace: "default",
			Annotations: map[string]string{
				gracefulActionAnnotation: `{"type":"RollingUpdate","phase":"Done"}`,
			},
		},
		Spec: appv1.StatefulSetSpec{
			UpdateStrategy: appv1.StatefulSetUpdateStrategy{
				Type: appv1.OnDeleteStatefulSetStrategyType,
			},
		},
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	dcgs.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(sts.DeepCopy()).Build()

	if err := dcgs.finalizeGracefulAction(context.Background(), sts); err != nil {
		t.Fatalf("finalizeGracefulAction failed: %v", err)
	}

	if sts.Spec.UpdateStrategy.Type != appv1.OnDeleteStatefulSetStrategyType {
		t.Fatalf("expected in-memory update strategy %s, got %s", appv1.OnDeleteStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	}

	live := &appv1.StatefulSet{}
	live.Namespace = sts.Namespace
	live.Name = sts.Name
	if err := dcgs.K8sclient.Get(context.Background(), client.ObjectKeyFromObject(live), live); err != nil {
		t.Fatalf("get live statefulset failed: %v", err)
	}
	if live.Spec.UpdateStrategy.Type != appv1.OnDeleteStatefulSetStrategyType {
		t.Fatalf("expected live update strategy %s, got %s", appv1.OnDeleteStatefulSetStrategyType, live.Spec.UpdateStrategy.Type)
	}
	if live.Annotations[gracefulActionAnnotation] != "" {
		t.Fatalf("expected graceful action annotation cleared, got %q", live.Annotations[gracefulActionAnnotation])
	}
}

func TestShouldFinalizeGracefulActionForOnDeleteRollingUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appv1 scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	replicas := int32(2)
	selector := map[string]string{
		dv1.DorisDisaggregatedClusterName:          "doris",
		dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
		dv1.DorisDisaggregatedPodType:              "compute",
	}
	sts := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris-cg1",
			Namespace: "default",
		},
		Spec: appv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: selector},
		},
		Status: appv1.StatefulSetStatus{
			CurrentRevision: "rev-old",
			UpdateRevision:  "rev-new",
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris-cg1-0",
			Namespace: "default",
			Labels: map[string]string{
				dv1.DorisDisaggregatedClusterName:          "doris",
				dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
				dv1.DorisDisaggregatedPodType:              "compute",
				resource.POD_CONTROLLER_REVISION_HASH_KEY:  "rev-old",
			},
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris-cg1-1",
			Namespace: "default",
			Labels: map[string]string{
				dv1.DorisDisaggregatedClusterName:          "doris",
				dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
				dv1.DorisDisaggregatedPodType:              "compute",
				resource.POD_CONTROLLER_REVISION_HASH_KEY:  "rev-new",
			},
		},
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	dcgs.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(sts, pod0, pod1).Build()

	ga := &dv1.GracefulAction{
		Type:           dv1.GracefulActionRollingUpdate,
		Phase:          dv1.GracefulPhaseDone,
		TargetRevision: "rev-new",
	}

	finished, err := dcgs.shouldFinalizeGracefulAction(context.Background(), sts, ga)
	if err != nil {
		t.Fatalf("shouldFinalizeGracefulAction failed: %v", err)
	}
	if finished {
		t.Fatalf("expected finalize gate blocked while outdated pod exists")
	}

	pod0.Labels[resource.POD_CONTROLLER_REVISION_HASH_KEY] = "rev-new"
	dcgs.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(sts, pod0, pod1).Build()

	finished, err = dcgs.shouldFinalizeGracefulAction(context.Background(), sts, ga)
	if err != nil {
		t.Fatalf("shouldFinalizeGracefulAction failed after revision update: %v", err)
	}
	if !finished {
		t.Fatalf("expected finalize gate pass once no outdated pod remains, even if currentRevision still lags")
	}
}

func TestHandleWaitPodReadyTimeoutExtendsDeadline(t *testing.T) {
	dcgs := &DisaggregatedComputeGroupsController{}
	cluster := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris",
			Namespace: "default",
		},
	}
	cg := &dv1.ComputeGroup{UniqueId: "cg1"}
	cgStatus := &dv1.ComputeGroupStatus{StatefulsetName: "doris-cg1"}
	est := &appv1.StatefulSet{}
	past := metav1.NewTime(time.Now().Add(-time.Minute))
	ga := &dv1.GracefulAction{
		Type:       dv1.GracefulActionRollingUpdate,
		Phase:      dv1.GracefulPhaseWaitPodReady,
		CurrentPod: "doris-cg1-0",
		StartedAt:  past,
		DeadlineAt: past,
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	dcgs.K8sclient = fake.NewClientBuilder().WithScheme(scheme).Build()
	dcgs.K8srecorder = record.NewFakeRecorder(10)

	before := ga.DeadlineAt
	if err := dcgs.handleWaitPodReady(context.Background(), cluster, cg, cgStatus, est, ga); err != nil {
		t.Fatalf("handleWaitPodReady failed: %v", err)
	}
	if !strings.Contains(ga.LastMessage, "Timed out waiting for replacement pod") {
		t.Fatalf("expected timeout message, got %q", ga.LastMessage)
	}
	if !ga.DeadlineAt.After(before.Time) {
		t.Fatalf("expected deadline extended, before=%s after=%s", before.Time, ga.DeadlineAt.Time)
	}
	if ga.Phase != dv1.GracefulPhaseWaitPodReady {
		t.Fatalf("expected phase to stay WaitPodReady, got %s", ga.Phase)
	}
}

func TestBackendProcessEpoch(t *testing.T) {
	be := &mysql.Backend{
		Status: `{"isShutdown":false,"processEpoch":"177","be_start_time":"177"}`,
	}
	epoch, ok := backendProcessEpoch(be)
	if !ok {
		t.Fatalf("expected process epoch to be parsed")
	}
	if epoch != "177" {
		t.Fatalf("expected epoch 177, got %q", epoch)
	}
}
