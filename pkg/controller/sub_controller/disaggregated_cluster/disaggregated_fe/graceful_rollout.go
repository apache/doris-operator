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
	"fmt"
	"sort"
	"strconv"
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
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// feRolloutStableObservations is the number of consecutive WaitFEAlive
	// polls that must observe the restarted FE alive and caught up before the
	// rollout advances. It filters out stale SHOW FRONTENDS views that still
	// describe the previous process generation.
	feRolloutStableObservations int32 = 2

	// feRolloutSQLTimeout bounds a single connection and query used by the
	// rollout. A slow or unreachable FE SQL endpoint must not stall reconcile.
	feRolloutSQLTimeout = 5 * time.Second

	// feRolloutSQLLookback is how long a rollout may wait for SHOW FRONTENDS
	// before falling back to StatefulSet ordinal order. This keeps a rollout
	// progressing when SQL is permanently unavailable.
	feRolloutSQLLookback = 3 * time.Minute

	// feRolloutQuorumTimeout bounds how long a rollout waits for enough alive
	// electable FEs. A cluster that is already below quorum will eventually
	// continue, with a warning event.
	feRolloutQuorumTimeout = 5 * time.Minute

	// feRolloutPhaseTimeoutFactor multiplies the FE termination grace period to
	// bound WaitPodReady and WaitFEAlive.
	feRolloutPhaseTimeoutFactor = 2
)

// feSQLClient is the subset of the Doris SQL client used by the FE rollout and
// the observer scale-down path. It is an interface so rollout decisions can be
// unit tested without a live cluster.
type feSQLClient interface {
	ShowFrontends() ([]*mysql.Frontend, error)
	Close() error
}

// newFEClusterSQLClient is overridable in tests.
var newFEClusterSQLClient = func(dfc *DisaggregatedFEController, ctx context.Context, cluster *dv1.DorisDisaggregatedCluster) (feSQLClient, error) {
	return dfc.newMasterSQLClient(ctx, cluster)
}

// newMasterSQLClient connects to the current FE master through the FE VIP
// service and follows the connection to the master when the VIP lands on a
// follower.
func (dfc *DisaggregatedFEController) newMasterSQLClient(ctx context.Context, cluster *dv1.DorisDisaggregatedCluster) (*mysql.DB, error) {
	adminUserName, password := dfc.GetManagementAdminUserAndPWD(ctx, cluster)

	// When the operator and the cluster are in different namespaces, the VIP
	// address is resolved in the cluster namespace, so qualify it.
	host := cluster.GetFEVIPAddresss()
	confMap := dfc.GetConfigValuesFromConfigMaps(cluster.Namespace, resource.FE_RESOLVEKEY, cluster.Spec.FeSpec.ConfigMaps)
	queryPort := resource.GetPort(confMap, resource.QUERY_PORT)
	tlsConfig, secretName := dfc.DisaggregatedSubDefaultController.FindSecretTLSConfig(confMap, cluster)
	secret, _ := k8s.GetSecret(ctx, dfc.K8sclient, cluster.Namespace, secretName)

	dbConf := mysql.DBConfig{
		User:     adminUserName,
		Password: password,
		Host:     host,
		Port:     strconv.FormatInt(int64(queryPort), 10),
		Database: "mysql",
		Timeout:  feRolloutSQLTimeout,
	}
	return mysql.NewDorisMasterSqlDB(dbConf, tlsConfig, secret)
}

// fetchFrontendOrdinals returns the SHOW FRONTENDS rows keyed by FE pod ordinal.
func (dfc *DisaggregatedFEController) fetchFrontendOrdinals(ctx context.Context, cluster *dv1.DorisDisaggregatedCluster) (map[int]*mysql.Frontend, error) {
	client, err := newFEClusterSQLClient(dfc, ctx, cluster)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	frontends, err := client.ShowFrontends()
	if err != nil {
		return nil, err
	}
	ordinals, err := dfc.frontendOrdinals(ctx, cluster, frontends)
	if err != nil {
		return nil, err
	}
	if len(ordinals) == 0 {
		return nil, fmt.Errorf("SHOW FRONTENDS returned no frontends that belong to statefulset %s", cluster.GetFEStatefulsetName())
	}
	return ordinals, nil
}

// frontendOrdinals maps FE pod ordinals to frontend rows. In FQDN mode the
// frontend host is the pod FQDN. Otherwise the pod IP must be resolved to a pod
// name first.
func (dfc *DisaggregatedFEController) frontendOrdinals(ctx context.Context, cluster *dv1.DorisDisaggregatedCluster, frontends []*mysql.Frontend) (map[int]*mysql.Frontend, error) {
	confMap := dfc.GetConfigValuesFromConfigMaps(cluster.Namespace, resource.FE_RESOLVEKEY, cluster.Spec.FeSpec.ConfigMaps)
	stsName := cluster.GetFEStatefulsetName()
	if resource.GetStartMode(confMap) == resource.START_MODEL_FQDN {
		return mysql.BuildSeqNumberToFrontendMap(frontends, nil, stsName)
	}

	podMap := make(map[string]string) // key is pod ip, value is pod name
	pods, err := k8s.GetPods(ctx, dfc.K8sclient, cluster.Namespace, dfc.getFEPodLabels(cluster))
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.GetName(), stsName) {
			podMap[pod.Status.PodIP] = pod.GetName()
		}
	}
	return mysql.BuildSeqNumberToFrontendMap(frontends, podMap, stsName)
}

// reconcileGracefulStatefulSet drives the master-aware FE rollout. It returns
// true while an annotation-tracked rollout still owns the FE Pod lifecycle.
func (dfc *DisaggregatedFEController) reconcileGracefulStatefulSet(ctx context.Context, st, est *appv1.StatefulSet, cluster *dv1.DorisDisaggregatedCluster) bool {
	replicasChanged := int32Value(st.Spec.Replicas) != int32Value(est.Spec.Replicas)
	stored, err := getFEGracefulAction(est)
	if err != nil {
		klog.Errorf("disaggregatedFEController reconcileGracefulStatefulSet decode annotation on statefulset %s/%s failed, err=%s", est.Namespace, est.Name, err.Error())
		// A corrupted annotation must not block every future reconcile. Remove it
		// explicitly because ApplyStatefulSet only merges annotations and would
		// otherwise leave the stale value on the live StatefulSet.
		clearFEGracefulAction(st)
		if removeErr := dfc.removeFEGracefulAction(ctx, st); removeErr != nil {
			klog.Errorf("disaggregatedFEController remove invalid FE rollout annotation from statefulset %s/%s failed, err=%s", st.Namespace, st.Name, removeErr.Error())
		}
		return false
	}

	// Scale changes are owned by the existing scale/drop-observer path. Abort an
	// in-progress rollout so replicas can converge first.
	if replicasChanged {
		if stored != nil {
			clearFEGracefulAction(st)
			if err := dfc.removeFEGracefulAction(ctx, st); err != nil {
				klog.Errorf("disaggregatedFEController abort FE rollout for statefulset %s/%s failed to remove annotation, err=%s", st.Namespace, st.Name, err.Error())
			}
			dfc.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.FERolloutTimeout),
				"Aborting FE rollout because replicas changed from %d to %d",
				int32Value(est.Spec.Replicas), int32Value(st.Spec.Replicas))
		}
		return false
	}

	if stored == nil {
		action := dfc.detectFEGracefulAction(st, est)
		if action == nil {
			return false
		}
		action.StartedAt = metav1.Now()
		setFEGracefulAction(st, action)
		dfc.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.FERolloutStarted),
			"Starting master-aware FE rollout for StatefulSet %s", est.Name)
		// The template and annotation must be applied before any Pod is deleted.
		return true
	}

	// Wait until ApplyStatefulSet has written the new template. The desired
	// StatefulSet must be re-decorated on every pass so the annotation survives
	// the apply comparison.
	if dfc.feStatefulSetTemplateChanged(st, est) {
		setFEGracefulAction(st, stored)
		return true
	}
	if est.Status.UpdateRevision == "" || est.Status.ObservedGeneration < est.Generation {
		stored.LastMessage = "Waiting for the StatefulSet controller to observe the new revision"
		setFEGracefulAction(st, stored)
		return true
	}
	if stored.TargetRevision == "" {
		stored.TargetRevision = est.Status.UpdateRevision
	} else if stored.TargetRevision != est.Status.UpdateRevision {
		// A new template was applied while this rollout was in flight. The old
		// target revision can never match again, so restart pod selection against
		// the newest revision instead of waiting for a timeout on every Pod.
		klog.Infof("disaggregatedFEController FE rollout target revision changed from %s to %s, restarting pod selection", stored.TargetRevision, est.Status.UpdateRevision)
		dfc.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.FERolloutStarted),
			"FE template changed during rollout; retargeting to revision %s", est.Status.UpdateRevision)
		resetFEGracefulTarget(stored, est.Status.UpdateRevision)
	}

	done, err := dfc.runFEGracefulStep(ctx, cluster, est, stored)
	if err != nil {
		stored.LastMessage = err.Error()
		klog.Errorf("disaggregatedFEController FE rollout step failed, namespace=%s name=%s pod=%s phase=%s err=%s",
			est.Namespace, est.Name, stored.CurrentPod, stored.Phase, err.Error())
	}
	if done {
		dfc.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulActionCompleted),
			"Master-aware FE rollout completed for StatefulSet %s", est.Name)
		if err := dfc.finalizeFEGracefulAction(ctx, st); err != nil {
			klog.Errorf("disaggregatedFEController finalize FE rollout for statefulset %s/%s failed, err=%s", st.Namespace, st.Name, err.Error())
			setFEGracefulAction(st, stored)
			return true
		}
		return false
	}

	setFEGracefulAction(st, stored)
	return true
}

// detectFEGracefulAction starts a master-aware rollout when the live template
// differs from the desired one. It also recovers a rollout whose StatefulSet
// revision already advanced before the operator restarted.
func (dfc *DisaggregatedFEController) detectFEGracefulAction(st, est *appv1.StatefulSet) *dv1.GracefulAction {
	if dfc.feStatefulSetTemplateChanged(st, est) {
		return &dv1.GracefulAction{
			Type:  dv1.GracefulActionRollingUpdate,
			Phase: dv1.GracefulPhaseDeletePod,
		}
	}
	if est.Status.UpdateRevision != "" && est.Status.UpdateRevision != est.Status.CurrentRevision {
		return &dv1.GracefulAction{
			Type:           dv1.GracefulActionRollingUpdate,
			Phase:          dv1.GracefulPhaseDeletePod,
			TargetRevision: est.Status.UpdateRevision,
		}
	}
	return nil
}

func (dfc *DisaggregatedFEController) feStatefulSetTemplateChanged(st, est *appv1.StatefulSet) bool {
	nst := st.DeepCopy()
	eSt := est.DeepCopy()
	dfc.RestrictConditionsEqual(nst, eSt)
	normalizeFEStatefulSetForCompare(nst)
	normalizeFEStatefulSetForCompare(eSt)
	return !resource.StatefulsetDeepEqualWithKey(nst, eSt, dv1.DisaggregatedSpecHashValueAnnotation, false)
}

func normalizeFEStatefulSetForCompare(st *appv1.StatefulSet) {
	st.Spec.UpdateStrategy = appv1.StatefulSetUpdateStrategy{}
	clearFEGracefulAction(st)
}

func (dfc *DisaggregatedFEController) runFEGracefulStep(ctx context.Context, cluster *dv1.DorisDisaggregatedCluster, est *appv1.StatefulSet, action *dv1.GracefulAction) (bool, error) {
	switch action.Phase {
	case dv1.GracefulPhaseDeletePod:
		return dfc.handleFEDeletePod(ctx, cluster, est, action)
	case dv1.GracefulPhaseWaitPodReady:
		return dfc.handleFEWaitPodReady(ctx, cluster, action)
	case dv1.GracefulPhaseWaitFEAlive:
		return dfc.handleFEWaitFEAlive(ctx, cluster, action)
	case dv1.GracefulPhaseDone:
		return true, nil
	default:
		return false, fmt.Errorf("unknown FE rollout phase %q", action.Phase)
	}
}

// handleFEDeletePod selects and deletes the next outdated FE. Followers are
// restarted before the current master so an election happens at most once per
// rollout.
func (dfc *DisaggregatedFEController) handleFEDeletePod(ctx context.Context, cluster *dv1.DorisDisaggregatedCluster, est *appv1.StatefulSet, action *dv1.GracefulAction) (bool, error) {
	if action.StartedAt.IsZero() {
		action.StartedAt = metav1.Now()
	}

	pods, err := dfc.outdatedFEPods(ctx, est, action.TargetRevision)
	if err != nil {
		return false, err
	}
	if len(pods) == 0 {
		action.Phase = dv1.GracefulPhaseDone
		return true, nil
	}

	// If a previous delete was interrupted before the state was persisted, adopt
	// the terminating Pod instead of deleting a second FE in the same rollout.
	for _, pod := range pods {
		if pod.DeletionTimestamp.IsZero() {
			continue
		}
		action.CurrentPod = pod.Name
		action.CurrentOrdinal = int32(podOrdinal(pod))
		action.InitialPodUID = string(pod.UID)
		action.ReplacementPodUID = ""
		action.StableBackendObservations = 0
		action.Phase = dv1.GracefulPhaseWaitPodReady
		resetFEDeadline(action, feRolloutPhaseTimeoutFactor*time.Duration(resource.DEFAULT_FE_TERMINATION_GRACE_PERIOD_SECONDS)*time.Second)
		action.LastMessage = fmt.Sprintf("FE pod %s is already terminating; waiting for it to be recreated", pod.Name)
		return false, nil
	}

	frontends, sqlErr := dfc.fetchFrontendOrdinals(ctx, cluster)
	pod, _, found := dfc.selectNextFEPod(pods, frontends, sqlErr, action, cluster)
	if !found {
		return false, nil
	}

	if sqlErr == nil {
		electionNumber := cluster.GetElectionNumber()
		if !feQuorumPreserved(frontends, int32(podOrdinal(pod)), electionNumber) {
			action.LastMessage = fmt.Sprintf("Waiting for quorum before restarting FE %s", pod.Name)
			if time.Since(action.StartedAt.Time) < feRolloutQuorumTimeout {
				dfc.K8srecorder.Eventf(cluster, string(sc.EventWarning), string(sc.FERolloutQuorumWait),
					"Waiting for enough alive electable FEs before restarting %s", pod.Name)
				return false, nil
			}
			dfc.K8srecorder.Eventf(cluster, string(sc.EventWarning), string(sc.FERolloutTimeout),
				"Continuing FE rollout for %s even though quorum could not be confirmed", pod.Name)
		}
	}

	action.CurrentPod = pod.Name
	action.CurrentOrdinal = int32(podOrdinal(pod))
	action.InitialPodUID = string(pod.UID)
	action.ReplacementPodUID = ""
	action.StableBackendObservations = 0
	action.Phase = dv1.GracefulPhaseWaitPodReady
	resetFEDeadline(action, feRolloutPhaseTimeoutFactor*time.Duration(resource.DEFAULT_FE_TERMINATION_GRACE_PERIOD_SECONDS)*time.Second)
	action.LastMessage = fmt.Sprintf("Deleted FE pod %s; waiting for it to become Ready again", pod.Name)

	if err := dfc.K8sclient.Delete(ctx, pod.DeepCopy()); err != nil && !apierrors.IsNotFound(err) {
		action.Phase = dv1.GracefulPhaseDeletePod
		action.CurrentPod = ""
		action.CurrentOrdinal = 0
		action.InitialPodUID = ""
		return false, fmt.Errorf("failed to delete FE pod %s: %w", pod.Name, err)
	}
	dfc.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.FERolloutPodDeleted),
		"Deleted FE pod %s during master-aware rollout", pod.Name)
	klog.Infof("disaggregatedFEController deleted FE pod %s ordinal=%d during master-aware rollout", pod.Name, action.CurrentOrdinal)
	return false, nil
}

func (dfc *DisaggregatedFEController) handleFEWaitPodReady(ctx context.Context, cluster *dv1.DorisDisaggregatedCluster, action *dv1.GracefulAction) (bool, error) {
	if action.DeadlineAt.IsZero() {
		resetFEDeadline(action, feRolloutPhaseTimeoutFactor*time.Duration(resource.DEFAULT_FE_TERMINATION_GRACE_PERIOD_SECONDS)*time.Second)
	}

	pod, err := dfc.getFEPod(ctx, cluster.Namespace, action.CurrentPod)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	ready := err == nil && pod.DeletionTimestamp.IsZero() &&
		string(pod.UID) != action.InitialPodUID &&
		pod.Labels[resource.POD_CONTROLLER_REVISION_HASH_KEY] == action.TargetRevision &&
		k8s.PodIsReady(&pod.Status)
	if !ready {
		action.LastMessage = fmt.Sprintf("Waiting for FE pod %s to be recreated with revision %s and become Ready", action.CurrentPod, action.TargetRevision)
		if feDeadlineExceeded(action) {
			dfc.K8srecorder.Eventf(cluster, string(sc.EventWarning), string(sc.FERolloutTimeout),
				"Timed out waiting for FE pod %s to become Ready; advancing rollout", action.CurrentPod)
			dfc.advanceFERollout(action)
		}
		return false, nil
	}

	action.ReplacementPodUID = string(pod.UID)
	action.Phase = dv1.GracefulPhaseWaitFEAlive
	resetFEDeadline(action, feRolloutPhaseTimeoutFactor*time.Duration(resource.DEFAULT_FE_TERMINATION_GRACE_PERIOD_SECONDS)*time.Second)
	action.LastMessage = fmt.Sprintf("FE pod %s is Ready; waiting for it to become alive in SHOW FRONTENDS", pod.Name)
	return false, nil
}

func (dfc *DisaggregatedFEController) handleFEWaitFEAlive(ctx context.Context, cluster *dv1.DorisDisaggregatedCluster, action *dv1.GracefulAction) (bool, error) {
	if action.DeadlineAt.IsZero() {
		resetFEDeadline(action, feRolloutPhaseTimeoutFactor*time.Duration(resource.DEFAULT_FE_TERMINATION_GRACE_PERIOD_SECONDS)*time.Second)
	}

	frontends, err := dfc.fetchFrontendOrdinals(ctx, cluster)
	if err == nil {
		fe := frontends[int(action.CurrentOrdinal)]
		alive, reason := feAliveAndCaughtUp(fe, frontends)
		if alive {
			action.StableBackendObservations++
			if action.StableBackendObservations >= feRolloutStableObservations {
				dfc.K8srecorder.Eventf(cluster, string(sc.EventNormal), string(sc.GracefulReplacementReady),
					"FE pod %s is alive in SHOW FRONTENDS", action.CurrentPod)
				dfc.advanceFERollout(action)
				return false, nil
			}
			action.LastMessage = fmt.Sprintf("FE pod %s is alive; confirming stability (%d/%d)", action.CurrentPod, action.StableBackendObservations, feRolloutStableObservations)
			return false, nil
		}
		action.StableBackendObservations = 0
		action.LastMessage = fmt.Sprintf("Waiting for FE pod %s to rejoin: %s", action.CurrentPod, reason)
	} else {
		action.StableBackendObservations = 0
		action.LastMessage = fmt.Sprintf("Waiting for SHOW FRONTENDS before checking FE pod %s: %v", action.CurrentPod, err)
	}

	if feDeadlineExceeded(action) {
		dfc.K8srecorder.Eventf(cluster, string(sc.EventWarning), string(sc.FERolloutTimeout),
			"Timed out waiting for FE pod %s to rejoin; advancing rollout", action.CurrentPod)
		dfc.advanceFERollout(action)
	}
	return false, nil
}

// advanceFERollout clears per-pod state and returns to pod selection.
func (dfc *DisaggregatedFEController) advanceFERollout(action *dv1.GracefulAction) {
	action.CurrentPod = ""
	action.CurrentOrdinal = 0
	action.InitialPodUID = ""
	action.ReplacementPodUID = ""
	action.StableBackendObservations = 0
	action.DeadlineAt = metav1.Time{}
	action.StartedAt = metav1.Now()
	action.LastMessage = ""
	action.Phase = dv1.GracefulPhaseDeletePod
}

// resetFEGracefulTarget starts pod selection again for a new StatefulSet
// revision without losing the fact that the rollout is still in progress.
func resetFEGracefulTarget(action *dv1.GracefulAction, revision string) {
	action.TargetRevision = revision
	action.CurrentPod = ""
	action.CurrentOrdinal = 0
	action.InitialPodUID = ""
	action.ReplacementPodUID = ""
	action.StableBackendObservations = 0
	action.DeadlineAt = metav1.Time{}
	action.StartedAt = metav1.Now()
	action.LastMessage = ""
	action.Phase = dv1.GracefulPhaseDeletePod
}

// outdatedFEPods lists Pods that are not on the target StatefulSet revision,
// highest ordinal first.
func (dfc *DisaggregatedFEController) outdatedFEPods(ctx context.Context, est *appv1.StatefulSet, targetRevision string) ([]corev1.Pod, error) {
	if est.Spec.Selector == nil {
		return nil, fmt.Errorf("statefulset %s/%s has no selector", est.Namespace, est.Name)
	}
	selector, err := metav1.LabelSelectorAsSelector(est.Spec.Selector)
	if err != nil {
		return nil, err
	}
	var podList corev1.PodList
	if err := dfc.K8sclient.List(ctx, &podList, client.InNamespace(est.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, err
	}
	var pods []corev1.Pod
	for _, pod := range podList.Items {
		if !strings.HasPrefix(pod.Name, est.Name+"-") {
			continue
		}
		if pod.Labels[resource.POD_CONTROLLER_REVISION_HASH_KEY] != targetRevision {
			pods = append(pods, pod)
		}
	}
	sort.Slice(pods, func(i, j int) bool {
		return extractOrdinal(pods[i].Name) > extractOrdinal(pods[j].Name)
	})
	return pods, nil
}

// selectNextFEPod orders outdated Pods: observers, followers, master, then
// Pods whose frontend row is unknown. When SHOW FRONTENDS is unavailable it
// waits briefly and then falls back to ordinal order.
func (dfc *DisaggregatedFEController) selectNextFEPod(
	pods []corev1.Pod,
	frontends map[int]*mysql.Frontend,
	sqlErr error,
	action *dv1.GracefulAction,
	cluster *dv1.DorisDisaggregatedCluster,
) (corev1.Pod, int32, bool) {
	if sqlErr != nil {
		if action.StartedAt.IsZero() {
			action.StartedAt = metav1.Now()
		}
		if time.Since(action.StartedAt.Time) < feRolloutSQLLookback {
			action.LastMessage = fmt.Sprintf("Waiting for SHOW FRONTENDS before selecting the next FE: %v", sqlErr)
			return corev1.Pod{}, 0, false
		}
		klog.Warningf("disaggregatedFEController FE rollout cannot query SHOW FRONTENDS after %s, falling back to ordinal order: %v", feRolloutSQLLookback, sqlErr)
		dfc.K8srecorder.Eventf(cluster, string(sc.EventWarning), string(sc.FERolloutTimeout),
			"SHOW FRONTENDS unavailable; restarting FE %s without master awareness", pods[0].Name)
		return pods[0], int32(podOrdinal(pods[0])), true
	}

	var observers, followers, masters, unknown []corev1.Pod
	for _, pod := range pods {
		fe := frontends[int(podOrdinal(pod))]
		switch {
		case fe != nil && fe.IsMaster:
			masters = append(masters, pod)
		case fe != nil && fe.Role == mysql.FE_OBSERVE_ROLE:
			observers = append(observers, pod)
		case fe != nil && fe.Role == mysql.FE_FOLLOWER_ROLE:
			followers = append(followers, pod)
		default:
			unknown = append(unknown, pod)
		}
	}

	ordered := make([]corev1.Pod, 0, len(pods))
	ordered = append(ordered, observers...)
	ordered = append(ordered, followers...)
	ordered = append(ordered, masters...)
	ordered = append(ordered, unknown...)
	if len(ordered) == 0 {
		return corev1.Pod{}, 0, false
	}
	return ordered[0], int32(podOrdinal(ordered[0])), true
}

// feQuorumPreserved reports whether the electable FEs that stay alive still
// form a majority after the candidate Pod is removed. Observers are not
// electable. The master may be reported as FOLLOWER or LEADER depending on the
// Doris version, so electability is defined by not being an observer.
func feQuorumPreserved(frontends map[int]*mysql.Frontend, candidateOrdinal, electionNumber int32) bool {
	// A single electable FE, or an already even-sized pair with no fault
	// tolerance, cannot keep a majority while one member is restarted.
	if electionNumber <= 2 || len(frontends) == 0 {
		return true
	}
	alive := int32(0)
	for ordinal, fe := range frontends {
		if fe == nil || fe.Role == mysql.FE_OBSERVE_ROLE || !fe.Alive {
			continue
		}
		if int32(ordinal) == candidateOrdinal {
			continue
		}
		alive++
	}
	return alive >= electionNumber/2+1
}

// feAliveAndCaughtUp checks membership, not only Pod readiness. The journal
// comparison is best effort and is skipped when either number cannot be parsed.
func feAliveAndCaughtUp(fe *mysql.Frontend, frontends map[int]*mysql.Frontend) (bool, string) {
	if fe == nil {
		return false, "frontend is not registered in SHOW FRONTENDS"
	}
	if !fe.Alive {
		return false, "alive=false"
	}
	if fe.IsMaster {
		return true, ""
	}

	masterJournal := int64(-1)
	for _, candidate := range frontends {
		if candidate == nil || !candidate.IsMaster {
			continue
		}
		if journal := parseReplayedJournal(candidate.ReplayedJournalId); journal >= 0 {
			if journal > masterJournal {
				masterJournal = journal
			}
		}
	}
	ownJournal := parseReplayedJournal(fe.ReplayedJournalId)
	if masterJournal < 0 || ownJournal < 0 {
		return true, ""
	}
	if ownJournal < masterJournal {
		return false, fmt.Sprintf("replayed journal %d is behind master journal %d", ownJournal, masterJournal)
	}
	return true, ""
}

func parseReplayedJournal(value string) int64 {
	journal, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return -1
	}
	return journal
}

func (dfc *DisaggregatedFEController) getFEPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := dfc.K8sclient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

func resetFEDeadline(action *dv1.GracefulAction, timeout time.Duration) {
	now := metav1.Now()
	action.StartedAt = now
	action.DeadlineAt = metav1.NewTime(now.Add(timeout))
}

func feDeadlineExceeded(action *dv1.GracefulAction) bool {
	return !action.DeadlineAt.IsZero() && time.Now().After(action.DeadlineAt.Time)
}

func podOrdinal(pod corev1.Pod) int {
	return extractOrdinal(pod.Name)
}

func extractOrdinal(podName string) int {
	index := strings.LastIndex(podName, "-")
	if index < 0 || index == len(podName)-1 {
		return -1
	}
	ordinal, err := strconv.Atoi(podName[index+1:])
	if err != nil {
		return -1
	}
	return ordinal
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

// hasFEGracefulAction reports whether a StatefulSet still carries rollout state.
func hasFEGracefulAction(sts *appv1.StatefulSet) bool {
	return feGracefulAnnotationValue(sts) != ""
}

func getFEGracefulAction(sts *appv1.StatefulSet) (*dv1.GracefulAction, error) {
	if sts.Annotations == nil {
		return nil, nil
	}
	raw := sts.Annotations[sc.GracefulActionAnnotation]
	if raw == "" {
		return nil, nil
	}
	var action dv1.GracefulAction
	if err := json.Unmarshal([]byte(raw), &action); err != nil {
		return nil, fmt.Errorf("failed to decode graceful action annotation on statefulset %s/%s: %w", sts.Namespace, sts.Name, err)
	}
	return &action, nil
}

func setFEGracefulAction(sts *appv1.StatefulSet, action *dv1.GracefulAction) {
	if sts.Annotations == nil {
		sts.Annotations = map[string]string{}
	}
	raw, err := json.Marshal(action)
	if err != nil {
		klog.Errorf("setFEGracefulAction: failed to marshal graceful action for statefulset %s/%s: %v", sts.Namespace, sts.Name, err)
		return
	}
	sts.Annotations[sc.GracefulActionAnnotation] = string(raw)
}

func clearFEGracefulAction(sts *appv1.StatefulSet) {
	if sts.Annotations == nil {
		return
	}
	delete(sts.Annotations, sc.GracefulActionAnnotation)
}

func feGracefulAnnotationValue(sts *appv1.StatefulSet) string {
	if sts == nil || sts.Annotations == nil {
		return ""
	}
	return sts.Annotations[sc.GracefulActionAnnotation]
}

// feGracefulStatefulSetControlEqual makes annotation updates part of the apply
// comparison. StatefulsetDeepEqualWithKey intentionally ignores annotations.
func feGracefulStatefulSetControlEqual(new, old *appv1.StatefulSet) bool {
	return feGracefulAnnotationValue(new) == feGracefulAnnotationValue(old)
}

func (dfc *DisaggregatedFEController) finalizeFEGracefulAction(ctx context.Context, st *appv1.StatefulSet) error {
	if err := dfc.removeFEGracefulAction(ctx, st); err != nil {
		return fmt.Errorf("failed to finalize FE rollout for statefulset %s/%s: %w", st.Namespace, st.Name, err)
	}
	return nil
}

// removeFEGracefulAction deletes the rollout annotation from the live
// StatefulSet. ApplyStatefulSet merges annotations from the desired object and
// therefore cannot delete a key on its own.
func (dfc *DisaggregatedFEController) removeFEGracefulAction(ctx context.Context, st *appv1.StatefulSet) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":null}}}`, sc.GracefulActionAnnotation))
	live := &appv1.StatefulSet{}
	live.Namespace = st.Namespace
	live.Name = st.Name
	if err := dfc.K8sclient.Patch(ctx, live, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return err
	}
	clearFEGracefulAction(st)
	return nil
}
