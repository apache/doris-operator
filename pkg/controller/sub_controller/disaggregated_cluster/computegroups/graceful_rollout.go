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
	"fmt"
	"sort"
	"strings"
	"time"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// stopBEGraceCommand is the command to trigger graceful BE shutdown.
	// $DORIS_HOME for disaggregated BE is /opt/apache-doris/be
	stopBEGraceCommand = "/opt/apache-doris/be/bin/stop_be.sh"
	stopBEGraceArg     = "--grace"

	// execTimeout is the timeout for the exec call itself (not the drain).
	execTimeout = 30 * time.Second

	// drainPollInterval is how often we check if the container has exited during drain.
	drainPollInterval = 5 * time.Second

	// beMainContainerName is the name of the disaggregated BE main container.
	beMainContainerName = "compute"

	// gracefulActionAnnotation stores the rollout state on the StatefulSet.
	// CR status alone is not safe here because old CRDs prune unknown status fields.
	gracefulActionAnnotation = "doris.disaggregated.cluster/graceful-action"
)

// gracefulRolloutReconcile is the entry point for graceful two-phase restart/shutdown.
// It returns true if the caller should skip normal StatefulSet apply (because graceful action is in progress).
func (dcgs *DisaggregatedComputeGroupsController) gracefulRolloutReconcile(
	ctx context.Context,
	restConfig *rest.Config,
	st *appv1.StatefulSet,
	est *appv1.StatefulSet,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
) (skipApply bool, err error) {
	cgStatus.GracefulAction = nil

	// Determine what graceful action is needed.
	action := dcgs.detectGracefulAction(st, est, cgStatus)
	storedAction, err := getGracefulAction(est)
	if err != nil {
		return true, err
	}
	if action == nil && storedAction == nil {
		// No graceful action needed or in progress.
		return false, nil
	}

	// If we have a new action and no existing action, store the action first.
	// For rolling updates this lets the new StatefulSet template be applied with
	// OnDelete before any pod is deleted.
	if action != nil && storedAction == nil {
		switch action.Type {
		case dv1.GracefulActionRollingUpdate:
			cgStatus.Phase = dv1.GracefulRolling
		case dv1.GracefulActionScaleDown:
			cgStatus.Phase = dv1.GracefulScaling
		case dv1.GracefulActionDelete:
			cgStatus.Phase = dv1.GracefulDeleting
		}
		klog.Infof("gracefulRolloutReconcile: starting graceful action type=%s for cg=%s", action.Type, cg.UniqueId)
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulDrainStarted),
			"Starting graceful %s for compute group %s", action.Type, cg.UniqueId)
		prepareGracefulStatefulSet(st, est, action)
		setGracefulAction(st, action)
		return true, nil
	}

	// If there's a higher-priority action pending, abort current.
	if action != nil && storedAction != nil && action.Type != storedAction.Type {
		if gracefulActionPriority(action.Type) > gracefulActionPriority(storedAction.Type) {
			klog.Infof("gracefulRolloutReconcile: aborting %s in favor of higher-priority %s for cg=%s",
				storedAction.Type, action.Type, cg.UniqueId)
			storedAction = action
		}
	}

	ga := storedAction
	if ga == nil {
		return false, nil
	}

	switch ga.Type {
	case dv1.GracefulActionRollingUpdate:
		cgStatus.Phase = dv1.GracefulRolling
	case dv1.GracefulActionScaleDown:
		cgStatus.Phase = dv1.GracefulScaling
	case dv1.GracefulActionDelete:
		cgStatus.Phase = dv1.GracefulDeleting
	}

	if ga.Type == dv1.GracefulActionRollingUpdate {
		dcgs.refreshRollingUpdateTargetRevision(est, ga)
	}

	// Run the state machine.
	err = dcgs.runGracefulStateMachine(ctx, restConfig, cluster, cg, cgStatus, est, ga)
	if err != nil {
		ga.LastMessage = err.Error()
		klog.Errorf("gracefulRolloutReconcile: state machine error for cg=%s pod=%s phase=%s: %v",
			cg.UniqueId, ga.CurrentPod, ga.Phase, err)
		return true, err
	}

	// Check if done.
	if ga.Phase == dv1.GracefulPhaseDone {
		klog.Infof("gracefulRolloutReconcile: graceful action completed for cg=%s", cg.UniqueId)
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulActionCompleted),
			"Graceful %s completed for compute group %s", ga.Type, cg.UniqueId)
		cgStatus.Phase = dv1.Reconciling
		if err := dcgs.finalizeGracefulAction(ctx, st); err != nil {
			return true, err
		}
		return false, nil
	}

	// Still in progress, skip normal apply.
	prepareGracefulStatefulSet(st, est, ga)
	setGracefulAction(st, ga)
	return true, nil
}

// detectGracefulAction determines if a new graceful action is needed.
func (dcgs *DisaggregatedComputeGroupsController) detectGracefulAction(
	st *appv1.StatefulSet,
	est *appv1.StatefulSet,
	cgStatus *dv1.ComputeGroupStatus,
) *dv1.GracefulAction {
	// Check for scale down: desired replicas < existing replicas.
	if *st.Spec.Replicas < *est.Spec.Replicas {
		desiredReplicas := *st.Spec.Replicas
		return &dv1.GracefulAction{
			Type:            dv1.GracefulActionScaleDown,
			Phase:           dv1.GracefulPhaseTriggerDrain,
			DesiredReplicas: &desiredReplicas,
		}
	}

	// Check for rolling update before applying the new template. Waiting for
	// UpdateRevision != CurrentRevision is too late because native RollingUpdate
	// may have already started deleting pods.
	if dcgs.statefulSetSpecChanged(st, est) {
		return &dv1.GracefulAction{
			Type:  dv1.GracefulActionRollingUpdate,
			Phase: dv1.GracefulPhaseTriggerDrain,
		}
	}

	// Recover an already-applied OnDelete update.
	if est.Status.UpdateRevision != "" && est.Status.UpdateRevision != est.Status.CurrentRevision {
		recoverAction := &dv1.GracefulAction{
			Type:           dv1.GracefulActionRollingUpdate,
			Phase:          dv1.GracefulPhaseTriggerDrain,
			TargetRevision: est.Status.UpdateRevision,
		}
		if _, _, found := dcgs.selectNextRollingUpdatePod(context.Background(), cgStatus.StatefulsetName, est, recoverAction); found {
			return recoverAction
		}
		klog.Infof("detectGracefulAction: skip recovering rolling update for statefulset %s because no outdated pods remain", est.Name)
	}

	return nil
}

// runGracefulStateMachine executes the graceful action state machine.
func (dcgs *DisaggregatedComputeGroupsController) runGracefulStateMachine(
	ctx context.Context,
	restConfig *rest.Config,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	est *appv1.StatefulSet,
	ga *dv1.GracefulAction,
) error {
	switch ga.Phase {
	case dv1.GracefulPhaseTriggerDrain:
		return dcgs.handleTriggerDrain(ctx, restConfig, cluster, cg, cgStatus, est, ga)
	case dv1.GracefulPhaseWaitDrain:
		return dcgs.handleWaitDrain(ctx, cluster, cg, cgStatus, ga)
	case dv1.GracefulPhaseDeletePod:
		return dcgs.handleDeletePod(ctx, cluster, cg, cgStatus, est, ga)
	case dv1.GracefulPhaseWaitPodReady:
		return dcgs.handleWaitPodReady(ctx, cluster, cg, cgStatus, est, ga)
	case dv1.GracefulPhaseWaitBEAlive:
		return dcgs.handleWaitBEAlive(ctx, cluster, cg, cgStatus, ga)
	case dv1.GracefulPhaseDone, dv1.GracefulPhaseFailed:
		return nil
	default:
		return fmt.Errorf("unknown graceful action phase: %s", ga.Phase)
	}
}

// handleTriggerDrain selects the next pod and triggers stop_be.sh --grace.
func (dcgs *DisaggregatedComputeGroupsController) handleTriggerDrain(
	ctx context.Context,
	restConfig *rest.Config,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	est *appv1.StatefulSet,
	ga *dv1.GracefulAction,
) error {
	if ga.Type == dv1.GracefulActionRollingUpdate {
		dcgs.refreshRollingUpdateTargetRevision(est, ga)
	}

	// If no current pod selected, pick the next one.
	if ga.CurrentPod == "" {
		if ga.Type == dv1.GracefulActionRollingUpdate && ga.TargetRevision == "" {
			if est.Status.UpdateRevision == "" || est.Status.UpdateRevision == est.Status.CurrentRevision {
				ga.LastMessage = fmt.Sprintf("Waiting for StatefulSet %s/%s update revision to be ready", est.Namespace, est.Name)
				return nil
			}
			ga.TargetRevision = est.Status.UpdateRevision
		}

		podName, ordinal, found := dcgs.selectNextPod(ctx, cluster, cg, cgStatus, est, ga)
		if !found {
			// No more pods to process.
			ga.Phase = dv1.GracefulPhaseDone
			return nil
		}
		ga.CurrentPod = podName
		ga.CurrentOrdinal = ordinal
		ga.DrainTriggered = false
	}

	// Verify the pod exists and is running.
	pod, err := dcgs.getPod(ctx, cluster.Namespace, ga.CurrentPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Pod already deleted, move to next phase.
			klog.Infof("handleTriggerDrain: pod %s already deleted, moving to next phase", ga.CurrentPod)
			if ga.Type == dv1.GracefulActionRollingUpdate {
				ga.Phase = dv1.GracefulPhaseWaitPodReady
			} else {
				dcgs.advanceToNextPod(ga)
			}
			return nil
		}
		return fmt.Errorf("failed to get pod %s: %w", ga.CurrentPod, err)
	}

	if !ga.DrainTriggered {
		backend, err := dcgs.getBackendByPodName(ctx, cluster, cgStatus, ga.CurrentPod)
		if err != nil {
			ga.LastMessage = fmt.Sprintf("Waiting for backend %s before disabling query: %v", ga.CurrentPod, err)
			return nil
		}

		queryDisabled, err := backendIsQueryDisabled(backend)
		if err != nil {
			ga.LastMessage = fmt.Sprintf("Waiting for backend %s disable_query status to be parseable: %v", ga.CurrentPod, err)
			return nil
		}

		if !ga.QueryDisabledTriggered {
			if err := dcgs.setBackendQueryDisabled(ctx, cluster, cgStatus, ga.CurrentPod, true); err != nil {
				return err
			}
			ga.QueryDisabledTriggered = true
			ga.LastMessage = fmt.Sprintf("Requested disable_query=true for backend %s", ga.CurrentPod)
			return nil
		}

		if !queryDisabled {
			ga.LastMessage = fmt.Sprintf("Waiting for backend %s disable_query=true before drain", ga.CurrentPod)
			return nil
		}
	}

	// Record the initial restart count of the BE container.
	ga.InitialRestartCount = getContainerRestartCount(pod, beMainContainerName)

	// Set drain timeout based on terminationGracePeriodSeconds.
	drainTimeout := int64(resource.DEFAULT_BE_TERMINATION_GRACE_PERIOD_SECONDS)
	if pod.Spec.TerminationGracePeriodSeconds != nil {
		drainTimeout = *pod.Spec.TerminationGracePeriodSeconds
	}
	now := metav1.Now()
	ga.StartedAt = now
	ga.DeadlineAt = metav1.NewTime(now.Add(time.Duration(drainTimeout) * time.Second))

	// Exec stop_be.sh --grace.
	if !ga.DrainTriggered {
		klog.Infof("handleTriggerDrain: executing stop_be.sh --grace on pod %s", ga.CurrentPod)
		stdout, stderr, execErr := k8s.ExecInPod(ctx, restConfig,
			cluster.Namespace, ga.CurrentPod, beMainContainerName,
			[]string{stopBEGraceCommand, stopBEGraceArg},
			execTimeout)

		if execErr != nil {
			klog.Warningf("handleTriggerDrain: exec failed on pod %s: %v, stdout=%s, stderr=%s",
				ga.CurrentPod, execErr, stdout, stderr)
			dcgs.K8srecorder.Eventf(cluster, string(sc.EventWarning), string(sc.GracefulDrainExecFailed),
				"Failed to exec stop_be.sh --grace on pod %s: %v", ga.CurrentPod, execErr)
			// Even if exec fails, proceed to WaitDrain to handle timeout or already-exiting BE.
		}
		ga.DrainTriggered = true
		ga.LastMessage = fmt.Sprintf("Drain triggered on pod %s", ga.CurrentPod)
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulDrainStarted),
			"Triggered graceful drain on pod %s", ga.CurrentPod)
	}

	ga.Phase = dv1.GracefulPhaseWaitDrain
	return nil
}

// handleWaitDrain waits for the BE container to exit or the drain timeout.
func (dcgs *DisaggregatedComputeGroupsController) handleWaitDrain(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	ga *dv1.GracefulAction,
) error {
	pod, err := dcgs.getPod(ctx, cluster.Namespace, ga.CurrentPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("handleWaitDrain: pod %s already gone", ga.CurrentPod)
			if ga.Type == dv1.GracefulActionRollingUpdate {
				ga.Phase = dv1.GracefulPhaseWaitPodReady
			} else {
				dcgs.advanceToNextPod(ga)
			}
			return nil
		}
		return err
	}

	// Check if main container has terminated.
	if isContainerTerminated(pod, beMainContainerName) {
		klog.Infof("handleWaitDrain: BE container terminated on pod %s", ga.CurrentPod)
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulDrainCompleted),
			"Graceful drain completed on pod %s (container exited)", ga.CurrentPod)
		ga.Phase = dv1.GracefulPhaseDeletePod
		return nil
	}

	// Check if restartCount increased (kubelet restarted the container after BE exited).
	currentRestartCount := getContainerRestartCount(pod, beMainContainerName)
	if currentRestartCount > ga.InitialRestartCount {
		klog.Infof("handleWaitDrain: restart count increased for pod %s (%d -> %d), container was restarted by kubelet",
			ga.CurrentPod, ga.InitialRestartCount, currentRestartCount)
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulDrainCompleted),
			"Graceful drain completed on pod %s (container restarted by kubelet, restartCount %d -> %d)",
			ga.CurrentPod, ga.InitialRestartCount, currentRestartCount)
		ga.Phase = dv1.GracefulPhaseDeletePod
		return nil
	}

	// Check timeout.
	if time.Now().After(ga.DeadlineAt.Time) {
		klog.Warningf("handleWaitDrain: drain timeout reached for pod %s", ga.CurrentPod)
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventWarning), string(sc.GracefulDrainTimeout),
			"Graceful drain timeout on pod %s, continuing with deletion", ga.CurrentPod)
		ga.Phase = dv1.GracefulPhaseDeletePod
		ga.LastMessage = fmt.Sprintf("Drain timeout reached for pod %s", ga.CurrentPod)
		return nil
	}

	// Still waiting.
	ga.LastMessage = fmt.Sprintf("Waiting for BE container to exit on pod %s (deadline: %s)",
		ga.CurrentPod, ga.DeadlineAt.Format(time.RFC3339))
	return nil
}

// handleDeletePod deletes the current pod.
func (dcgs *DisaggregatedComputeGroupsController) handleDeletePod(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	est *appv1.StatefulSet,
	ga *dv1.GracefulAction,
) error {
	pod, err := dcgs.getPod(ctx, cluster.Namespace, ga.CurrentPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("handleDeletePod: pod %s already deleted", ga.CurrentPod)
			dcgs.afterPodDeleted(ga, cluster, cg, cgStatus, est)
			return nil
		}
		return err
	}

	// Delete the pod.
	klog.Infof("handleDeletePod: deleting pod %s", ga.CurrentPod)
	if err := dcgs.K8sclient.Delete(ctx, pod); err != nil {
		if apierrors.IsNotFound(err) {
			dcgs.afterPodDeleted(ga, cluster, cg, cgStatus, est)
			return nil
		}
		return fmt.Errorf("failed to delete pod %s: %w", ga.CurrentPod, err)
	}

	dcgs.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulPodDeleted),
		"Deleted pod %s during graceful %s", ga.CurrentPod, ga.Type)
	dcgs.afterPodDeleted(ga, cluster, cg, cgStatus, est)
	return nil
}

// afterPodDeleted determines the next phase after a pod is deleted.
func (dcgs *DisaggregatedComputeGroupsController) afterPodDeleted(
	ga *dv1.GracefulAction,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	est *appv1.StatefulSet,
) {
	switch ga.Type {
	case dv1.GracefulActionRollingUpdate:
		// Wait for replacement pod to become ready.
		ga.Phase = dv1.GracefulPhaseWaitPodReady
	case dv1.GracefulActionScaleDown:
		// For scale down, update StatefulSet replicas after pod is deleted.
		// This prevents StatefulSet from recreating the deleted pod.
		newReplicas := ga.CurrentOrdinal // replicas = current ordinal (0-indexed)
		dcgs.updateStatefulSetReplicas(context.Background(), est, newReplicas)
		dcgs.advanceToNextPod(ga)
	case dv1.GracefulActionDelete:
		newReplicas := ga.CurrentOrdinal
		dcgs.updateStatefulSetReplicas(context.Background(), est, newReplicas)
		dcgs.advanceToNextPod(ga)
	}
}

// updateStatefulSetReplicas patches the StatefulSet replicas to the given value.
func (dcgs *DisaggregatedComputeGroupsController) updateStatefulSetReplicas(ctx context.Context, est *appv1.StatefulSet, replicas int32) {
	var current appv1.StatefulSet
	if err := dcgs.K8sclient.Get(ctx, types.NamespacedName{Namespace: est.Namespace, Name: est.Name}, &current); err != nil {
		klog.Errorf("updateStatefulSetReplicas: failed to get StatefulSet %s/%s: %v", est.Namespace, est.Name, err)
		return
	}
	if *current.Spec.Replicas == replicas {
		return
	}
	current.Spec.Replicas = &replicas
	if err := dcgs.K8sclient.Update(ctx, &current); err != nil {
		klog.Errorf("updateStatefulSetReplicas: failed to update StatefulSet %s/%s replicas to %d: %v",
			est.Namespace, est.Name, replicas, err)
	} else {
		klog.Infof("updateStatefulSetReplicas: updated StatefulSet %s/%s replicas to %d", est.Namespace, est.Name, replicas)
	}
}

// handleWaitPodReady waits for the replacement pod (same ordinal) to become Ready.
func (dcgs *DisaggregatedComputeGroupsController) handleWaitPodReady(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	est *appv1.StatefulSet,
	ga *dv1.GracefulAction,
) error {
	pod, err := dcgs.getPod(ctx, cluster.Namespace, ga.CurrentPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Pod hasn't been recreated yet, wait.
			ga.LastMessage = fmt.Sprintf("Waiting for replacement pod %s to be created", ga.CurrentPod)
			return nil
		}
		return err
	}

	if k8s.PodIsReady(&pod.Status) {
		klog.Infof("handleWaitPodReady: replacement pod %s is ready", ga.CurrentPod)
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulReplacementReady),
			"Replacement pod %s is ready", ga.CurrentPod)

		ga.Phase = dv1.GracefulPhaseWaitBEAlive
		return nil
	}

	ga.LastMessage = fmt.Sprintf("Waiting for replacement pod %s to become ready", ga.CurrentPod)
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) refreshRollingUpdateTargetRevision(est *appv1.StatefulSet, ga *dv1.GracefulAction) {
	if ga == nil || ga.Type != dv1.GracefulActionRollingUpdate {
		return
	}
	if est == nil || est.Status.UpdateRevision == "" {
		return
	}
	if ga.TargetRevision != "" && ga.TargetRevision != est.Status.UpdateRevision {
		klog.Infof("refreshRollingUpdateTargetRevision: statefulset %s/%s target revision changed from %s to %s",
			est.Namespace, est.Name, ga.TargetRevision, est.Status.UpdateRevision)
	}
	ga.TargetRevision = est.Status.UpdateRevision
}

// handleWaitBEAlive waits until FE reports the replacement backend as alive.
func (dcgs *DisaggregatedComputeGroupsController) handleWaitBEAlive(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	ga *dv1.GracefulAction,
) error {
	backend, err := dcgs.getBackendByPodName(ctx, cluster, cgStatus, ga.CurrentPod)
	if err != nil {
		ga.LastMessage = fmt.Sprintf("Waiting for backend %s to appear in FE: %v", ga.CurrentPod, err)
		return nil
	}

	shutdown, err := backendIsShutdown(backend)
	if err != nil {
		ga.LastMessage = fmt.Sprintf("Waiting for backend %s status to be parseable: %v", ga.CurrentPod, err)
		return nil
	}

	queryDisabled, err := backendIsQueryDisabled(backend)
	if err != nil {
		ga.LastMessage = fmt.Sprintf("Waiting for backend %s query status to be parseable: %v", ga.CurrentPod, err)
		return nil
	}

	if backend.Alive && !shutdown && queryDisabled {
		if err := dcgs.setBackendQueryDisabled(ctx, cluster, cgStatus, ga.CurrentPod, false); err != nil {
			return err
		}
		ga.LastMessage = fmt.Sprintf("Requested disable_query=false for backend %s", ga.CurrentPod)
		return nil
	}

	if backend.Alive && !shutdown && !queryDisabled {
		klog.Infof("handleWaitBEAlive: backend %s is alive in FE", ga.CurrentPod)
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulReplacementReady),
			"Backend %s is alive in FE", ga.CurrentPod)
		dcgs.advanceToNextPod(ga)
		return nil
	}

	ga.LastMessage = fmt.Sprintf(
		"Waiting for backend %s to become ready in FE (alive=%t shutdown=%t queryDisabled=%t heartbeatFailures=%d err=%s)",
		ga.CurrentPod, backend.Alive, shutdown, queryDisabled, backend.HeartbeatFailureCounter, backend.ErrMsg)
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) getBackendByPodName(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cgStatus *dv1.ComputeGroupStatus,
	podName string,
) (*mysql.Backend, error) {
	sqlClient, err := dcgs.getMasterSqlClient(ctx, cluster)
	if err != nil {
		return nil, err
	}
	defer sqlClient.Close()

	backends, err := sqlClient.GetBackendsByComputeGroupId(cgStatus.ComputeGroupId)
	if err != nil {
		return nil, err
	}
	for _, backend := range backends {
		if backendMatchesPod(backend, podName) {
			return backend, nil
		}
	}
	return nil, fmt.Errorf("backend for pod %s not found", podName)
}

func backendIsShutdown(backend *mysql.Backend) (bool, error) {
	status, err := backendStatusMap(backend)
	if err != nil {
		return false, err
	}
	if status == nil {
		return false, nil
	}
	raw, ok := status["isShutdown"]
	if !ok {
		return false, nil
	}
	shutdown, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("backend status isShutdown has unexpected type %T", raw)
	}
	return shutdown, nil
}

func backendIsQueryDisabled(backend *mysql.Backend) (bool, error) {
	status, err := backendStatusMap(backend)
	if err != nil {
		return false, err
	}
	if status == nil {
		return false, nil
	}
	raw, ok := status["isQueryDisabled"]
	if !ok {
		return false, nil
	}
	queryDisabled, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("backend status isQueryDisabled has unexpected type %T", raw)
	}
	return queryDisabled, nil
}

func backendStatusMap(backend *mysql.Backend) (map[string]interface{}, error) {
	if backend == nil || backend.Status == "" {
		return nil, nil
	}
	var status map[string]interface{}
	if err := json.Unmarshal([]byte(backend.Status), &status); err != nil {
		return nil, err
	}
	return status, nil
}

func backendMatchesPod(backend *mysql.Backend, podName string) bool {
	if backend == nil {
		return false
	}
	return strings.HasPrefix(backend.Host, podName+".") || backend.Host == podName
}

// selectNextPod finds the next pod to process for the current graceful action.
func (dcgs *DisaggregatedComputeGroupsController) selectNextPod(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	est *appv1.StatefulSet,
	ga *dv1.GracefulAction,
) (podName string, ordinal int32, found bool) {
	stsName := cgStatus.StatefulsetName

	switch ga.Type {
	case dv1.GracefulActionRollingUpdate:
		// Find pods that don't match the target revision, process from highest ordinal.
		return dcgs.selectNextRollingUpdatePod(ctx, stsName, est, ga)

	case dv1.GracefulActionScaleDown:
		// Process from highest ordinal down to desired replicas.
		if ga.DesiredReplicas == nil {
			return "", 0, false
		}
		desiredReplicas := *ga.DesiredReplicas
		currentReplicas := *est.Spec.Replicas
		if desiredReplicas >= currentReplicas {
			return "", 0, false
		}
		// Highest ordinal that still exists.
		ordinal := currentReplicas - 1
		podName = fmt.Sprintf("%s-%d", stsName, ordinal)
		return podName, ordinal, true

	case dv1.GracefulActionDelete:
		// Process all pods from highest to lowest.
		currentReplicas := *est.Spec.Replicas
		if currentReplicas <= 0 {
			return "", 0, false
		}
		ordinal := currentReplicas - 1
		podName = fmt.Sprintf("%s-%d", stsName, ordinal)
		return podName, ordinal, true
	}

	return "", 0, false
}

// selectNextRollingUpdatePod finds the next pod that needs updating (not using the target revision).
func (dcgs *DisaggregatedComputeGroupsController) selectNextRollingUpdatePod(
	ctx context.Context,
	_ string,
	est *appv1.StatefulSet,
	ga *dv1.GracefulAction,
) (string, int32, bool) {
	targetRevision := ga.TargetRevision
	if targetRevision == "" {
		targetRevision = est.Status.UpdateRevision
	}

	selector, err := metav1.LabelSelectorAsSelector(est.Spec.Selector)
	if err != nil {
		klog.Errorf("selectNextRollingUpdatePod: failed to build selector for statefulset %s/%s: %v", est.Namespace, est.Name, err)
		return "", 0, false
	}
	var podList corev1.PodList
	if err := dcgs.K8sclient.List(ctx, &podList, client.InNamespace(est.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		klog.Errorf("selectNextRollingUpdatePod: failed to list pods: %v", err)
		return "", 0, false
	}

	// Sort pods by ordinal descending (process highest ordinal first).
	var outdatedPods []corev1.Pod
	for _, pod := range podList.Items {
		podRevision := pod.Labels[resource.POD_CONTROLLER_REVISION_HASH_KEY]
		if podRevision != targetRevision {
			outdatedPods = append(outdatedPods, pod)
		}
	}

	if len(outdatedPods) == 0 {
		return "", 0, false
	}

	// Sort by ordinal descending.
	sort.Slice(outdatedPods, func(i, j int) bool {
		oi := extractOrdinal(outdatedPods[i].Name)
		oj := extractOrdinal(outdatedPods[j].Name)
		return oi > oj
	})

	pod := outdatedPods[0]
	ordinal := extractOrdinal(pod.Name)
	return pod.Name, int32(ordinal), true
}

// advanceToNextPod resets current pod state and goes back to TriggerDrain for the next pod,
// or marks Done if no more pods.
func (dcgs *DisaggregatedComputeGroupsController) advanceToNextPod(ga *dv1.GracefulAction) {
	ga.CurrentPod = ""
	ga.CurrentOrdinal = 0
	ga.DrainTriggered = false
	ga.QueryDisabledTriggered = false
	ga.InitialRestartCount = 0
	ga.StartedAt = metav1.Time{}
	ga.DeadlineAt = metav1.Time{}
	ga.LastMessage = ""
	ga.Phase = dv1.GracefulPhaseTriggerDrain
	// The next reconcile loop will call selectNextPod; if none found, phase becomes Done.
}

// getPod gets a specific pod by namespace and name.
func (dcgs *DisaggregatedComputeGroupsController) getPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := dcgs.K8sclient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// isContainerTerminated checks if the named container in the pod has terminated.
func isContainerTerminated(pod *corev1.Pod, containerName string) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			return cs.State.Terminated != nil
		}
	}
	return false
}

// getContainerRestartCount returns the restart count for the named container.
func getContainerRestartCount(pod *corev1.Pod, containerName string) int32 {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			return cs.RestartCount
		}
	}
	return 0
}

func (dcgs *DisaggregatedComputeGroupsController) setBackendQueryDisabled(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cgStatus *dv1.ComputeGroupStatus,
	podName string,
	disabled bool,
) error {
	sqlClient, err := dcgs.getMasterSqlClient(ctx, cluster)
	if err != nil {
		return err
	}
	defer sqlClient.Close()

	backend, err := dcgs.getBackendByPodNameWithClient(sqlClient, cgStatus, podName)
	if err != nil {
		return err
	}
	if err := sqlClient.ModifyBackendQueryDisabled(backend, disabled); err != nil {
		return fmt.Errorf("failed to set disable_query=%t for backend %s: %w", disabled, podName, err)
	}
	klog.Infof("setBackendQueryDisabled: set disable_query=%t for backend %s", disabled, podName)
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) getBackendByPodNameWithClient(
	sqlClient *mysql.DB,
	cgStatus *dv1.ComputeGroupStatus,
	podName string,
) (*mysql.Backend, error) {
	backends, err := sqlClient.GetBackendsByComputeGroupId(cgStatus.ComputeGroupId)
	if err != nil {
		return nil, err
	}
	for _, backend := range backends {
		if backendMatchesPod(backend, podName) {
			return backend, nil
		}
	}
	return nil, fmt.Errorf("backend for pod %s not found", podName)
}

// extractOrdinal extracts the ordinal number from a StatefulSet pod name (e.g., "sts-name-2" -> 2).
func extractOrdinal(podName string) int {
	for i := len(podName) - 1; i >= 0; i-- {
		if podName[i] == '-' {
			ordinal := 0
			for j := i + 1; j < len(podName); j++ {
				ordinal = ordinal*10 + int(podName[j]-'0')
			}
			return ordinal
		}
	}
	return 0
}

// gracefulActionPriority returns the priority of a graceful action type (higher = more urgent).
func gracefulActionPriority(t dv1.GracefulActionType) int {
	switch t {
	case dv1.GracefulActionDelete:
		return 3
	case dv1.GracefulActionScaleDown:
		return 2
	case dv1.GracefulActionRollingUpdate:
		return 1
	default:
		return 0
	}
}

// ensureOnDeleteStrategy ensures the StatefulSet uses OnDelete update strategy.
// This prevents K8s from auto-deleting pods when the template changes.
func ensureOnDeleteStrategy(st *appv1.StatefulSet) {
	st.Spec.UpdateStrategy = appv1.StatefulSetUpdateStrategy{
		Type:          appv1.OnDeleteStatefulSetStrategyType,
		RollingUpdate: nil,
	}
}

func prepareGracefulStatefulSet(st, est *appv1.StatefulSet, ga *dv1.GracefulAction) {
	ensureOnDeleteStrategy(st)
	if ga.Type == dv1.GracefulActionScaleDown || ga.Type == dv1.GracefulActionDelete {
		st.Spec.Replicas = est.Spec.Replicas
	}
}

func (dcgs *DisaggregatedComputeGroupsController) statefulSetSpecChanged(st, est *appv1.StatefulSet) bool {
	nst := st.DeepCopy()
	eSt := est.DeepCopy()
	dcgs.RestrictConditionsEqual(nst, eSt)
	normalizeGracefulStatefulSetForCompare(nst)
	normalizeGracefulStatefulSetForCompare(eSt)
	return !resource.StatefulsetDeepEqualWithKey(nst, eSt, dv1.DisaggregatedSpecHashValueAnnotation, false)
}

func normalizeGracefulStatefulSetForCompare(st *appv1.StatefulSet) {
	st.Spec.UpdateStrategy = appv1.StatefulSetUpdateStrategy{}
	clearGracefulAction(st)
}

func getGracefulAction(st *appv1.StatefulSet) (*dv1.GracefulAction, error) {
	if st.Annotations == nil {
		return nil, nil
	}
	raw := st.Annotations[gracefulActionAnnotation]
	if raw == "" {
		return nil, nil
	}
	var ga dv1.GracefulAction
	if err := json.Unmarshal([]byte(raw), &ga); err != nil {
		return nil, fmt.Errorf("failed to decode graceful action annotation on statefulset %s/%s: %w", st.Namespace, st.Name, err)
	}
	return &ga, nil
}

func setGracefulAction(st *appv1.StatefulSet, ga *dv1.GracefulAction) {
	if st.Annotations == nil {
		st.Annotations = map[string]string{}
	}
	bs, err := json.Marshal(ga)
	if err != nil {
		klog.Errorf("setGracefulAction: failed to marshal graceful action for statefulset %s/%s: %v", st.Namespace, st.Name, err)
		return
	}
	st.Annotations[gracefulActionAnnotation] = string(bs)
}

func clearGracefulAction(st *appv1.StatefulSet) {
	if st.Annotations == nil {
		return
	}
	delete(st.Annotations, gracefulActionAnnotation)
}

func (dcgs *DisaggregatedComputeGroupsController) finalizeGracefulAction(ctx context.Context, st *appv1.StatefulSet) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":null}},"spec":{"updateStrategy":{"type":"RollingUpdate","rollingUpdate":{"partition":0}}}}`, gracefulActionAnnotation))
	live := &appv1.StatefulSet{}
	live.Namespace = st.Namespace
	live.Name = st.Name
	if err := dcgs.K8sclient.Patch(ctx, live, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return fmt.Errorf("failed to finalize graceful action for statefulset %s/%s: %w", st.Namespace, st.Name, err)
	}
	st.Spec.UpdateStrategy = appv1.StatefulSetUpdateStrategy{
		Type: appv1.RollingUpdateStatefulSetStrategyType,
		RollingUpdate: &appv1.RollingUpdateStatefulSetStrategy{
			Partition: func() *int32 {
				var partition int32
				return &partition
			}(),
		},
	}
	clearGracefulAction(st)
	return nil
}

func gracefulStatefulSetControlEqual(new, old *appv1.StatefulSet) bool {
	if new.Spec.UpdateStrategy.Type != old.Spec.UpdateStrategy.Type {
		return false
	}
	if gracefulAnnotationValue(new) != gracefulAnnotationValue(old) {
		return false
	}
	return true
}

func gracefulAnnotationValue(st *appv1.StatefulSet) string {
	if st.Annotations == nil {
		return ""
	}
	return st.Annotations[gracefulActionAnnotation]
}
