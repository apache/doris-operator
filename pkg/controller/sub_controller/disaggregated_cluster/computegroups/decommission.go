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
	"fmt"
	"strconv"
	"strings"
	"time"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	ms "github.com/apache/doris-operator/pkg/common/utils/metaservice"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const decommissionActionAnnotation = "doris.disaggregated.cluster/decommission-action"

const (
	decommissionPrepared   = "Prepared"
	decommissionSubmitted  = "Submitted"
	decommissionWaitFESync = "WaitFESync"
	decommissionDropping   = "Dropping"
)

// The task is persisted before any SQL is sent. A Submitted task is deliberately
// not resubmitted on restart or transport failure: SQL may already have committed.
type decommissionAction struct {
	ComputeGroupID string               `json:"computeGroupID"`
	FromReplicas   int32                `json:"fromReplicas"`
	Replicas       int32                `json:"replicas"`
	CloudUniqueID  string               `json:"cloudUniqueID"`
	InstanceID     string               `json:"instanceID"`
	Targets        []decommissionTarget `json:"targets"`
}

type decommissionTarget struct {
	Ordinal       int32        `json:"ordinal"`
	Host          string       `json:"host,omitempty"`
	HeartbeatPort int          `json:"heartbeatPort,omitempty"`
	BackendID     string       `json:"backendID,omitempty"`
	CloudUniqueID string       `json:"cloudUniqueID,omitempty"`
	Phase         string       `json:"phase"`
	CompletedAt   *metav1.Time `json:"completedAt,omitempty"`
	AbsentSince   *metav1.Time `json:"absentSince,omitempty"`
}

type decommissionSnapshot struct {
	Nodes    map[int32]ms.Node
	Backends map[int32]*mysql.Backend
	// One observation per registered FE, including observers. No FE may retain a
	// target when Kubernetes removes its pod.
	FEViews  []map[int32]*mysql.Backend
	SyncWait time.Duration
}

type decommissionSQL interface {
	DecommissionBE([]*mysql.Backend) error
	DropBE([]*mysql.Backend) error
}

// reconcileDecommissionScaleDown owns replica changes while decommission is
// enabled. Metadata-only patches leave the running pod template untouched.
func (dcgs *DisaggregatedComputeGroupsController) reconcileDecommissionScaleDown(ctx context.Context, st, est *appv1.StatefulSet, cluster *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) (bool, error) {
	raw := est.Annotations[decommissionActionAnnotation]
	if raw == "" && (!cluster.Spec.EnableDecommission || *st.Spec.Replicas >= *est.Spec.Replicas) {
		return false, nil
	}
	var cgStatus *dv1.ComputeGroupStatus
	for i := range cluster.Status.ComputeGroupStatuses {
		if cluster.Status.ComputeGroupStatuses[i].UniqueId == cg.UniqueId {
			cgStatus = &cluster.Status.ComputeGroupStatuses[i]
			break
		}
	}
	if cgStatus == nil {
		return true, errors.New("compute group status is missing during decommission")
	}
	cgStatus.Phase = dv1.Decommissioning
	if !cluster.Spec.EnableDecommission {
		return true, errors.New("restore enableDecommission to finish the active decommission task")
	}
	if *st.Spec.Replicas < 0 || cgStatus.ComputeGroupId == "" {
		return true, errors.New("decommission requires a valid compute group ID and nonnegative replicas")
	}
	var action decommissionAction
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &action); err != nil {
			return true, errors.New("invalid decommission action annotation")
		}
		if err := action.validate(*st.Spec.Replicas, *est.Spec.Replicas, cgStatus.ComputeGroupId); err != nil {
			return true, err
		}
	}
	sqlClient, err := dcgs.getMasterSqlClient(ctx, cluster)
	if err != nil {
		return true, err
	}
	defer sqlClient.Close()
	backends, err := sqlClient.GetBackendsByComputeGroupId(cgStatus.ComputeGroupId)
	if err != nil {
		return true, err
	}
	if raw == "" {
		action = decommissionAction{ComputeGroupID: cgStatus.ComputeGroupId, FromReplicas: *est.Spec.Replicas, Replicas: *st.Spec.Replicas}
		for ordinal := action.FromReplicas - 1; ordinal >= action.Replicas; ordinal-- {
			action.Targets = append(action.Targets, decommissionTarget{Ordinal: ordinal, Phase: decommissionPrepared})
		}
		feConf := dcgs.GetConfigValuesFromConfigMaps(cluster.Namespace, resource.FE_RESOLVEKEY, cluster.Spec.FeSpec.ConfigMaps)
		if configured, ok := feConf["cloud_unique_id"]; ok {
			action.CloudUniqueID = fmt.Sprint(configured)
		}
		for _, backend := range backends {
			var tag struct {
				CloudUniqueID string `json:"cloud_unique_id"`
			}
			if err := json.Unmarshal([]byte(backend.Tag), &tag); err != nil {
				return true, err
			}
			if tag.CloudUniqueID != "" {
				action.CloudUniqueID = tag.CloudUniqueID
				break
			}
		}
		if action.CloudUniqueID == "" {
			// A group can already be empty before a scale-down is requested. DDC
			// images use this degraded ID format for the operator's CLUSTER_ID.
			action.CloudUniqueID = fmt.Sprintf("1:%d:operator", cluster.GetInstanceHashId())
		}
	}
	snapshot, err := dcgs.decommissionSnapshot(ctx, cluster, cg, &action, sqlClient, backends)
	if err != nil {
		return true, err
	}
	// Manual SQL on retained nodes does not expand the operator's deletion range.
	// Surface the discrepancy without cancelling or dropping those nodes.
	retiringRetained := map[int32]bool{}
	for ordinal, node := range snapshot.Nodes {
		if ordinal < action.Replicas && (node.Status == ms.Decommissioning || node.Status == ms.Decommissioned) {
			retiringRetained[ordinal] = true
		}
	}
	for _, view := range snapshot.FEViews {
		for ordinal, backend := range view {
			if ordinal < action.Replicas && backend.SystemDecommissioned {
				retiringRetained[ordinal] = true
			}
		}
	}
	for ordinal := range retiringRetained {
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventWarning), "RetainedBackendDecommissioning", "Compute group %s backend ordinal %d is under decommission outside the requested scale-down range", cg.UniqueId, ordinal)
	}
	save := func() error { return dcgs.saveDecommissionAction(ctx, est, &action) }
	if raw == "" {
		// Observe and persist the identities of this CR's targets before SQL.
		for i := range action.Targets {
			if err := bindDecommissionTarget(&action.Targets[i], snapshot); err != nil {
				return true, err
			}
		}
		return true, save()
	}
	done, err := advanceDecommission(&action, snapshot, time.Now(), sqlClient, save)
	if err != nil {
		dcgs.K8srecorder.Eventf(cluster, string(sc.EventWarning), string(sc.CGSqlExecFailed), "Decommission of compute group %s is blocked: %s", cg.UniqueId, err)
		return true, err
	}
	if !done {
		return true, nil
	}
	if err := dcgs.finishDecommissionAction(ctx, est, action.Replicas); err != nil {
		return true, err
	}
	cgStatus.Phase = dv1.Scaling
	return true, nil
}

// Remove the task and lower replicas atomically, after every MS and FE view
// confirms absence. A restart cannot lose progress before replicas are changed.
func (dcgs *DisaggregatedComputeGroupsController) finishDecommissionAction(ctx context.Context, est *appv1.StatefulSet, replicas int32) error {
	before := est.DeepCopy()
	delete(est.Annotations, decommissionActionAnnotation)
	est.Spec.Replicas = &replicas
	return dcgs.K8sclient.Patch(ctx, est, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}

func (a *decommissionAction) validate(replicas, current int32, cgid string) error {
	if a.ComputeGroupID == "" || a.ComputeGroupID != cgid || a.CloudUniqueID == "" || a.InstanceID == "" || a.Replicas < 0 || a.FromReplicas <= a.Replicas || len(a.Targets) != int(a.FromReplicas-a.Replicas) {
		return errors.New("invalid decommission task identity or replica range")
	}
	if replicas != a.Replicas || current != a.FromReplicas {
		return errors.New("replica target changed during decommission; restore the original target to finish the active task")
	}
	for i, target := range a.Targets {
		if target.Ordinal != a.FromReplicas-1-int32(i) {
			return errors.New("invalid decommission target ordinal")
		}
		switch target.Phase {
		case decommissionPrepared, decommissionSubmitted, decommissionWaitFESync, decommissionDropping:
		default:
			return errors.New("invalid decommission target phase")
		}
	}
	return nil
}

func (dcgs *DisaggregatedComputeGroupsController) saveDecommissionAction(ctx context.Context, est *appv1.StatefulSet, action *decommissionAction) error {
	value, err := json.Marshal(action)
	if err != nil {
		return err
	}
	before := est.DeepCopy()
	if est.Annotations == nil {
		est.Annotations = map[string]string{}
	}
	est.Annotations[decommissionActionAnnotation] = string(value)
	return dcgs.K8sclient.Patch(ctx, est, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}

func (dcgs *DisaggregatedComputeGroupsController) decommissionSnapshot(ctx context.Context, cluster *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup, action *decommissionAction, sqlClient *mysql.DB, backends []*mysql.Backend) (*decommissionSnapshot, error) {
	conf := dcgs.GetConfigValuesFromConfigMaps(cluster.Namespace, resource.MS_RESOLVEKEY, cluster.Spec.MetaService.ConfigMaps)
	endpoint := fmt.Sprintf("%s.%s:%d", cluster.GetMSServiceName(), cluster.Namespace, resource.GetPort(conf, resource.BRPC_LISTEN_PORT))
	token := resource.DefaultMsToken
	if value, ok := conf[resource.DefaultMsTokenKey]; ok {
		token = fmt.Sprint(value)
	}
	msClient := ms.NewClient(endpoint, token)
	if action.InstanceID == "" {
		instanceID, err := msClient.GetInstanceID(ctx, action.CloudUniqueID)
		if err != nil {
			return nil, err
		}
		action.InstanceID = instanceID
		action.CloudUniqueID = "1:" + instanceID + ":operator"
	}
	remote, err := msClient.GetCluster(ctx, action.InstanceID, action.CloudUniqueID, action.ComputeGroupID)
	if err != nil {
		return nil, err
	}
	podOrdinals := map[string]int32{}
	var pods corev1.PodList
	if err := dcgs.K8sclient.List(ctx, &pods, client.InNamespace(cluster.Namespace), client.MatchingLabels(dcgs.newCGPodsSelector(cluster.Name, cg.UniqueId))); err != nil {
		return nil, err
	}
	stsName := cluster.GetCGStatefulsetName(cg)
	for _, pod := range pods.Items {
		ordinal, err := decommissionOrdinal(stsName, pod.Name, nil)
		if err != nil {
			return nil, err
		}
		if pod.Status.PodIP != "" {
			podOrdinals[pod.Status.PodIP] = ordinal
		}
	}
	snapshot := &decommissionSnapshot{Nodes: map[int32]ms.Node{}, Backends: map[int32]*mysql.Backend{}}
	for _, node := range remote.Nodes {
		host := node.Host
		if host == "" {
			host = node.IP
		}
		ordinal, err := decommissionOrdinal(stsName, host, podOrdinals)
		if err != nil {
			return nil, err
		}
		if node.CloudUniqueID == "" || node.HeartbeatPort <= 0 {
			return nil, errors.New("MS node identity or heartbeat port is missing")
		}
		if _, exists := snapshot.Nodes[ordinal]; exists {
			return nil, errors.New("multiple MS nodes map to the same pod ordinal")
		}
		snapshot.Nodes[ordinal] = node
	}
	snapshot.Backends, err = indexDecommissionBackends(stsName, podOrdinals, backends)
	if err != nil {
		return nil, err
	}
	// Read every registered FE directly: a query through the Service or master
	// alone cannot prove that followers and observers have synchronized removal.
	frontends, err := sqlClient.ShowFrontends()
	if err != nil {
		return nil, err
	}
	if len(frontends) == 0 {
		return nil, errors.New("no FE observations are available")
	}
	feConf := dcgs.GetConfigValuesFromConfigMaps(cluster.Namespace, resource.FE_RESOLVEKEY, cluster.Spec.FeSpec.ConfigMaps)
	interval := 10 * time.Second
	if value, ok := feConf["cloud_cluster_check_interval_second"]; ok {
		seconds, err := strconv.Atoi(fmt.Sprint(value))
		if err != nil || seconds <= 0 || seconds > 86400 {
			return nil, errors.New("invalid cloud_cluster_check_interval_second")
		}
		if time.Duration(seconds)*time.Second > interval {
			interval = time.Duration(seconds) * time.Second
		}
	}
	// Allow two configured synchronization periods, and still verify FE state.
	snapshot.SyncWait = 2 * interval
	user, password := dcgs.GetManagementAdminUserAndPWD(ctx, cluster)
	tlsConfig, secretName := dcgs.DisaggregatedSubDefaultController.FindSecretTLSConfig(feConf, cluster)
	secret, err := k8s.GetSecret(ctx, dcgs.K8sclient, cluster.Namespace, secretName)
	if tlsConfig != nil && err != nil {
		return nil, err
	}
	for _, fe := range frontends {
		if !fe.Alive {
			return nil, fmt.Errorf("waiting for FE %s to become alive and synchronize", fe.Host)
		}
		view := snapshot.Backends
		feDB := sqlClient
		if !fe.IsMaster {
			db, err := mysql.NewDorisSqlDB(mysql.DBConfig{User: user, Password: password, Host: fe.Host, Port: strconv.Itoa(fe.QueryPort), Database: "mysql", Timeout: 10 * time.Second}, tlsConfig, secret)
			if err != nil {
				return nil, fmt.Errorf("cannot observe FE %s: %w", fe.Host, err)
			}
			feDB = db
			nodes, queryErr := db.GetBackendsByComputeGroupId(action.ComputeGroupID)
			if queryErr != nil {
				db.Close()
				return nil, queryErr
			}
			view, err = indexDecommissionBackends(stsName, podOrdinals, nodes)
			if err != nil {
				db.Close()
				return nil, err
			}
		}
		actualInterval, configErr := frontendSyncInterval(feDB)
		if !fe.IsMaster {
			feDB.Close()
		}
		if configErr != nil {
			return nil, configErr
		}
		if 2*actualInterval > snapshot.SyncWait {
			snapshot.SyncWait = 2 * actualInterval
		}
		snapshot.FEViews = append(snapshot.FEViews, view)
	}
	return snapshot, nil
}

// Early 3.x keeps the interval in FeConstants and returns no matching row.
// Later versions expose it through FE config, including changes made via SQL.
func frontendSyncInterval(db *mysql.DB) (time.Duration, error) {
	var rows []struct {
		Value string `db:"Value"`
	}
	if err := db.USelect(&rows, "ADMIN SHOW FRONTEND CONFIG LIKE 'cloud_cluster_check_interval_second'"); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 10 * time.Second, nil
	}
	seconds, err := strconv.Atoi(rows[0].Value)
	if err != nil || seconds <= 0 || seconds > 86400 {
		return 0, errors.New("invalid FE synchronization interval")
	}
	return time.Duration(seconds) * time.Second, nil
}

func decommissionOrdinal(stsName, host string, podIPs map[string]int32) (int32, error) {
	if ordinal, ok := podIPs[host]; ok {
		return ordinal, nil
	}
	name := strings.Split(host, ".")[0]
	prefix := stsName + "-"
	if !strings.HasPrefix(name, prefix) {
		return 0, fmt.Errorf("cannot map backend host %q to StatefulSet %s", host, stsName)
	}
	ordinal, err := strconv.ParseInt(strings.TrimPrefix(name, prefix), 10, 32)
	if err != nil || ordinal < 0 {
		return 0, fmt.Errorf("invalid backend ordinal in host %q", host)
	}
	return int32(ordinal), nil
}

func indexDecommissionBackends(stsName string, podIPs map[string]int32, backends []*mysql.Backend) (map[int32]*mysql.Backend, error) {
	indexed := map[int32]*mysql.Backend{}
	for _, backend := range backends {
		ordinal, err := decommissionOrdinal(stsName, backend.Host, podIPs)
		if err != nil {
			return nil, err
		}
		if _, exists := indexed[ordinal]; exists {
			return nil, errors.New("multiple FE backends map to the same pod ordinal")
		}
		indexed[ordinal] = backend
	}
	return indexed, nil
}

func bindDecommissionTarget(target *decommissionTarget, snapshot *decommissionSnapshot) error {
	if node, ok := snapshot.Nodes[target.Ordinal]; ok {
		if target.CloudUniqueID != "" && target.CloudUniqueID != node.CloudUniqueID {
			return errors.New("MS target identity changed during decommission")
		}
		target.CloudUniqueID = node.CloudUniqueID
	}
	if backend := snapshot.Backends[target.Ordinal]; backend != nil {
		if backend.BackendID == "" || backend.HeartbeatPort <= 0 {
			return errors.New("FE target identity or heartbeat port is missing")
		}
		var tag struct {
			CloudUniqueID string `json:"cloud_unique_id"`
		}
		if backend.Tag != "" {
			if err := json.Unmarshal([]byte(backend.Tag), &tag); err != nil {
				return errors.New("invalid target backend tag")
			}
			if tag.CloudUniqueID != "" && target.CloudUniqueID != "" && tag.CloudUniqueID != target.CloudUniqueID {
				return errors.New("MS and FE target identities disagree")
			}
		}
		if target.BackendID != "" && target.BackendID != backend.BackendID {
			return errors.New("FE target identity changed during decommission")
		}
		if target.Host != "" && (target.Host != backend.Host || target.HeartbeatPort != backend.HeartbeatPort) {
			return errors.New("target endpoint changed during decommission")
		}
		target.BackendID, target.Host, target.HeartbeatPort = backend.BackendID, backend.Host, backend.HeartbeatPort
	}
	for _, view := range snapshot.FEViews {
		if backend := view[target.Ordinal]; backend != nil && target.BackendID != "" && backend.BackendID != target.BackendID {
			return errors.New("FE views disagree about the target backend identity")
		}
	}
	return nil
}

func advanceDecommission(action *decommissionAction, snapshot *decommissionSnapshot, now time.Time, sqlClient decommissionSQL, save func() error) (bool, error) {
	if len(snapshot.FEViews) == 0 || snapshot.SyncWait < 10*time.Second {
		return false, errors.New("FE observations and a synchronization interval are required before decommission")
	}
	allAbsent := true
	for i := range action.Targets {
		target := &action.Targets[i]
		if err := bindDecommissionTarget(target, snapshot); err != nil {
			return false, err
		}
		node, inMS := snapshot.Nodes[target.Ordinal]
		backend := snapshot.Backends[target.Ordinal]
		inFE, feSynchronized := false, true
		for _, view := range snapshot.FEViews {
			if be := view[target.Ordinal]; be != nil {
				inFE = true
				if !be.SystemDecommissioned {
					feSynchronized = false
				}
			}
		}
		if !inMS && !inFE && backend == nil {
			// Some MS versions implement state changes using a remove/add pair.
			// A single missing observation must not be mistaken for a durable DROP.
			if target.AbsentSince == nil {
				observedAt := metav1.NewTime(now.Truncate(time.Second).Add(time.Second))
				target.AbsentSince = &observedAt
				if err := save(); err != nil {
					return false, err
				}
				allAbsent = false
			} else if now.Before(target.AbsentSince.Add(snapshot.SyncWait)) {
				allAbsent = false
			}
			continue
		}
		allAbsent = false
		if target.AbsentSince != nil {
			target.AbsentSince = nil
			if err := save(); err != nil {
				return false, err
			}
		}
		if !inMS {
			// MS may already have removed this node while FE still exposes it.
			// Wait for every FE; do not issue SQL against a stale FE-only entry.
			continue
		}
		switch node.Status {
		case ms.Running:
			if target.CompletedAt != nil || target.Phase == decommissionDropping {
				return false, errors.New("MS target returned to RUNNING after completion; refusing to drop it")
			}
			// In 3.x and early 4.0, manual SQL marks FE first and may leave MS RUNNING.
			manuallyStarted := backend != nil && backend.SystemDecommissioned
			for _, view := range snapshot.FEViews {
				if be := view[target.Ordinal]; be != nil && be.SystemDecommissioned {
					manuallyStarted = true
				}
			}
			if manuallyStarted || target.Phase == decommissionSubmitted {
				if target.Phase != decommissionSubmitted {
					target.Phase = decommissionSubmitted
					if err := save(); err != nil {
						return false, err
					}
				}
				continue
			}
			if backend == nil {
				continue
			} // Wait for registration to reach the master.
			target.Phase = decommissionSubmitted
			if err := save(); err != nil {
				return false, err
			}
			if err := sqlClient.DecommissionBE([]*mysql.Backend{backend}); err != nil {
				return false, fmt.Errorf("decommission submission result is uncertain; inspect backend %s before retrying: %w", backend.Host, err)
			}
		case ms.Decommissioning:
			if target.Phase == decommissionDropping {
				return false, errors.New("MS target regressed after DROP was requested")
			}
			if target.Phase != decommissionSubmitted || target.CompletedAt != nil {
				target.Phase = decommissionSubmitted
				target.CompletedAt = nil
				if err := save(); err != nil {
					return false, err
				}
			}
		case ms.Decommissioned:
			if target.CompletedAt == nil {
				completedAt := metav1.NewTime(now.Truncate(time.Second).Add(time.Second))
				target.CompletedAt = &completedAt
				target.Phase = decommissionWaitFESync
				if err := save(); err != nil {
					return false, err
				}
				continue
			}
			if now.Before(target.CompletedAt.Add(snapshot.SyncWait)) || !feSynchronized {
				continue
			}
			if backend == nil {
				continue
			} // MS still contains the node; no safe SQL endpoint yet.
			if target.Phase != decommissionDropping {
				target.Phase = decommissionDropping
				if err := save(); err != nil {
					return false, err
				}
			}
			// Retrying DROP is safe: the snapshot above still proves this exact node is
			// completed. Failures block replica changes and are verified on the next poll.
			if err := sqlClient.DropBE([]*mysql.Backend{backend}); err != nil {
				return false, fmt.Errorf("drop decommissioned backend %s failed: %w", backend.Host, err)
			}
		default:
			return false, fmt.Errorf("unknown MS state %q for ordinal %d", node.Status, target.Ordinal)
		}
	}
	return allAbsent, nil
}
