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
	"fmt"
	"sort"
	"time"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
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

	// Determine what graceful action is needed.
	action := dcgs.detectGracefulAction(st, est, cgStatus)
	if action == nil && cgStatus.GracefulAction == nil {
		// No graceful action needed or in progress.
		return false, nil
	}

	// If we have a new action and no existing action, start it.
	if action != nil && cgStatus.GracefulAction == nil {
		cgStatus.GracefulAction = action
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
	}

	// If there's a higher-priority action pending, abort current.
	if action != nil && cgStatus.GracefulAction != nil && action.Type != cgStatus.GracefulAction.Type {
		if gracefulActionPriority(action.Type) > gracefulActionPriority(cgStatus.GracefulAction.Type) {
			klog.Infof("gracefulRolloutReconcile: aborting %s in favor of higher-priority %s for cg=%s",
				cgStatus.GracefulAction.Type, action.Type, cg.UniqueId)
			cgStatus.GracefulAction = action
		}
	}

	ga := cgStatus.GracefulAction
	if ga == nil {
		return false, nil
	}

	// Run the state machine.
	err = dcgs.runGracefulStateMachine(ctx, restConfig, cluster, cg, cgStatus, est)
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
		cgStatus.GracefulAction = nil
		cgStatus.Phase = dv1.Reconciling
		return false, nil
	}

	// Still in progress, skip normal apply.
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

	// Check for rolling update: revision mismatch.
	// We detect this by comparing the spec hash annotation.
	if est.Status.UpdateRevision != "" && est.Status.UpdateRevision != est.Status.CurrentRevision {
		return &dv1.GracefulAction{
			Type:           dv1.GracefulActionRollingUpdate,
			Phase:          dv1.GracefulPhaseTriggerDrain,
			TargetRevision: est.Status.UpdateRevision,
		}
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
) error {
	ga := cgStatus.GracefulAction

	switch ga.Phase {
	case dv1.GracefulPhaseTriggerDrain:
		return dcgs.handleTriggerDrain(ctx, restConfig, cluster, cg, cgStatus, est)
	case dv1.GracefulPhaseWaitDrain:
		return dcgs.handleWaitDrain(ctx, cluster, cg, cgStatus)
	case dv1.GracefulPhaseDeletePod:
		return dcgs.handleDeletePod(ctx, cluster, cg, cgStatus, est)
	case dv1.GracefulPhaseWaitPodReady:
		return dcgs.handleWaitPodReady(ctx, cluster, cg, cgStatus, est)
	case dv1.GracefulPhaseWaitBEAlive:
		// For first version, treat WaitBEAlive same as Done after WaitPodReady.
		return dcgs.handleWaitBEAlive(ctx, cluster, cg, cgStatus)
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
) error {
	ga := cgStatus.GracefulAction

	// If no current pod selected, pick the next one.
	if ga.CurrentPod == "" {
		podName, ordinal, found := dcgs.selectNextPod(ctx, cluster, cg, cgStatus, est)
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
) error {
	ga := cgStatus.GracefulAction

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
) error {
	ga := cgStatus.GracefulAction

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
) error {
	ga := cgStatus.GracefulAction

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

		// For first version, skip WaitBEAlive and advance directly.
		dcgs.advanceToNextPod(ga)
		return nil
	}

	ga.LastMessage = fmt.Sprintf("Waiting for replacement pod %s to become ready", ga.CurrentPod)
	return nil
}

// handleWaitBEAlive checks FE SHOW BACKENDS for BE alive status.
// First version: just advance to next pod after WaitPodReady.
func (dcgs *DisaggregatedComputeGroupsController) handleWaitBEAlive(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
) error {
	// For the first version, just advance.
	dcgs.advanceToNextPod(cgStatus.GracefulAction)
	return nil
}

// selectNextPod finds the next pod to process for the current graceful action.
func (dcgs *DisaggregatedComputeGroupsController) selectNextPod(
	ctx context.Context,
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	est *appv1.StatefulSet,
) (podName string, ordinal int32, found bool) {
	ga := cgStatus.GracefulAction
	stsName := cgStatus.StatefulsetName

	switch ga.Type {
	case dv1.GracefulActionRollingUpdate:
		// Find pods that don't match the target revision, process from highest ordinal.
		return dcgs.selectNextRollingUpdatePod(ctx, cluster, cg, cgStatus, est)

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
	cluster *dv1.DorisDisaggregatedCluster,
	cg *dv1.ComputeGroup,
	cgStatus *dv1.ComputeGroupStatus,
	est *appv1.StatefulSet,
) (string, int32, bool) {
	ga := cgStatus.GracefulAction
	targetRevision := ga.TargetRevision
	if targetRevision == "" {
		targetRevision = est.Status.UpdateRevision
	}

	selector := dcgs.newCGPodsSelector(cluster.Name, cg.UniqueId)
	var podList corev1.PodList
	if err := dcgs.K8sclient.List(ctx, &podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(selector)); err != nil {
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
	ga.InitialRestartCount = 0
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
		Type: appv1.OnDeleteStatefulSetStrategyType,
	}
}
