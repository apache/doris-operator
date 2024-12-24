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
	"fmt"
	"github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

var _ sc.DisaggregatedSubController = &DisaggregatedFEController{}

var (
	disaggregatedFEController = "disaggregatedFEController"
)

type DisaggregatedFEController struct {
	sc.DisaggregatedSubDefaultController
}

func New(mgr ctrl.Manager) *DisaggregatedFEController {
	return &DisaggregatedFEController{
		DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(disaggregatedFEController),
			ControllerName: disaggregatedFEController},
	}
}

func (dfc *DisaggregatedFEController) Sync(ctx context.Context, obj client.Object) error {
	ddc := obj.(*v1.DorisDisaggregatedCluster)
	//TODO: check ms status
	if !dfc.msAvailable(ddc) {
		dfc.K8srecorder.Event(ddc, string(sc.EventNormal), string(sc.WaitMetaServiceAvailable), "meta service have not ready.")
		return nil
	}

	dfc.CheckSecretMountPath(ddc, ddc.Spec.FeSpec.Secrets)
	dfc.CheckSecretExist(ctx, ddc, ddc.Spec.FeSpec.Secrets)

	if ddc.Spec.FeSpec.Replicas == nil {
		klog.Errorf("disaggregatedFEController sync disaggregatedDorisCluster namespace=%s,name=%s ,The number of disaggregated fe replicas is nil and has been corrected to the default value %d", ddc.Namespace, ddc.Name, v1.DefaultFeReplicaNumber)
		dfc.K8srecorder.Event(ddc, string(sc.EventNormal), string(sc.FESpecSetError), "The number of disaggregated fe replicas is nil and has been corrected to the default value 2")
		ddc.Spec.FeSpec.Replicas = &v1.DefaultFeReplicaNumber
	}

	electionNumber := ddc.GetElectionNumber()

	if *(ddc.Spec.FeSpec.Replicas) < electionNumber {
		dfc.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.FESpecSetError), "The number of disaggregated fe ElectionNumber is large than Replicas, Replicas has been corrected to the correct minimum value")
		klog.Errorf("disaggregatedFEController Sync disaggregatedDorisCluster namespace=%s,name=%s ,The number of disaggregated fe ElectionNumber(%d) is large than Replicas(%d), Replicas has been corrected to the correct minimum value", ddc.Namespace, ddc.Name, electionNumber, *(ddc.Spec.FeSpec.Replicas))
		ddc.Spec.FeSpec.Replicas = &electionNumber
	}

	confMap := dfc.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.FE_RESOLVEKEY, ddc.Spec.FeSpec.ConfigMaps)
	svcInternal := dfc.newInternalService(ddc, confMap)
	svc := dfc.newService(ddc, confMap)

	st := dfc.NewStatefulset(ddc, confMap)
	dfc.initialFEStatus(ddc)

	event, err := dfc.DefaultReconcileService(ctx, svcInternal)
	if err != nil {
		if event != nil {
			dfc.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
		}
		klog.Errorf("disaggregatedFEController reconcile internal service namespace %s name %s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return err
	}

	event, err = dfc.DefaultReconcileService(ctx, svc)
	if err != nil {
		if event != nil {
			dfc.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
		}
		klog.Errorf("disaggregatedFEController reconcile service namespace %s name %s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return err
	}

	event, err = dfc.reconcileStatefulset(ctx, st, ddc)
	if err != nil {
		if event != nil {
			dfc.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
		}
		klog.Errorf("disaggregatedFEController reconcile statefulset namespace %s name %s failed, err=%s", st.Namespace, st.Name, err.Error())
		return err
	}

	return nil
}

func (dfc *DisaggregatedFEController) msAvailable(ddc *v1.DorisDisaggregatedCluster) bool {
	endpoints := corev1.Endpoints{}
	if err := dfc.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.GetMSServiceName()}, &endpoints); err != nil {
		klog.Infof("DisaggregatedFEController Sync wait meta service name %s available occur failed %s\n", ddc.GetMSServiceName(), err.Error())
		return false
	}

	for _, sub := range endpoints.Subsets {
		if len(sub.Addresses) > 0 {
			return true
		}
	}
	return false
}

func (dfc *DisaggregatedFEController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	ddc := obj.(*v1.DorisDisaggregatedCluster)

	if err := dfc.recycleResources(ctx, ddc); err != nil {
		klog.Errorf("DisaggregatedFE ClearResources RecycleResources failed, namespace %s name %s, err:%s.", ddc.Namespace, ddc.Name, err.Error())
		return false, err
	}

	statefulsetName := ddc.GetFEStatefulsetName()
	serviceName := ddc.GetFEServiceName()
	serviceInternalName := ddc.GetFEInternalServiceName()

	if ddc.DeletionTimestamp.IsZero() {
		return true, nil
	}

	if err := k8s.DeleteService(ctx, dfc.K8sclient, ddc.Namespace, serviceName); err != nil {
		klog.Errorf("disaggregatedFEController delete service namespace %s name %s failed, err=%s", ddc.Namespace, serviceName, err.Error())
		dfc.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.FEServiceDeleteFailed), err.Error())
		return false, err
	}

	if err := k8s.DeleteService(ctx, dfc.K8sclient, ddc.Namespace, serviceInternalName); err != nil {
		klog.Errorf("disaggregatedFEController delete internal service namespace %s name %s failed, err=%s", ddc.Namespace, serviceInternalName, err.Error())
		dfc.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.FEServiceDeleteFailed), err.Error())
		return false, err
	}

	if err := k8s.DeleteStatefulset(ctx, dfc.K8sclient, ddc.Namespace, statefulsetName); err != nil {
		klog.Errorf("disaggregatedFEController delete statefulset namespace %s name %s failed, err=%s", ddc.Namespace, statefulsetName, err.Error())
		dfc.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.FEStatefulsetDeleteFailed), err.Error())
		return false, err
	}

	return true, nil
}

func (dfc *DisaggregatedFEController) GetControllerName() string {
	return disaggregatedFEController
}

// podIsFollower if fe pod name has tail: '-n', n is less than electionNumber is follower
func (dfc *DisaggregatedFEController) podIsFollower(podName, stfName string, electionNumber int) bool {
	if !strings.HasPrefix(podName, stfName+"-") {
		return false
	}
	suffix := podName[len(stfName)+1:]
	num, err := strconv.Atoi(suffix)
	if err != nil {
		return false
	}
	return num < electionNumber
}

func (dfc *DisaggregatedFEController) UpdateComponentStatus(obj client.Object) error {
	var masterAliveReplicas int32
	var availableReplicas int32
	var creatingReplicas int32
	var failedReplicas int32

	ddc := obj.(*v1.DorisDisaggregatedCluster)

	stfName := ddc.GetFEStatefulsetName()

	// FEStatus
	feSpec := ddc.Spec.FeSpec
	electionNumber := ddc.GetElectionNumber()
	selector := dfc.newFEPodsSelector(ddc.Name)
	var podList corev1.PodList
	if err := dfc.K8sclient.List(context.Background(), &podList, client.InNamespace(ddc.Namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for _, pod := range podList.Items {

		if ready := k8s.PodIsReady(&pod.Status); ready {
			if dfc.podIsFollower(pod.Name, stfName, int(electionNumber)) {
				masterAliveReplicas++
			}
			availableReplicas++
		} else if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			creatingReplicas++
		} else {
			failedReplicas++
		}
	}

	// at least one fe PodIsReady FEStatus.AvailableStatu is Available,
	// ClusterHealth.FeAvailable is true,
	// for ClusterHealth.Health is yellow
	if masterAliveReplicas > 0 {
		ddc.Status.FEStatus.AvailableStatus = v1.Available
		ddc.Status.ClusterHealth.FeAvailable = true
	}
	// all fe pods  are Ready, FEStatus.Phase is Ready，
	// for ClusterHealth.Health is green
	if masterAliveReplicas == electionNumber && availableReplicas == *(feSpec.Replicas) {
		ddc.Status.FEStatus.Phase = v1.Ready
	}

	return nil
}

// initial fe status before sync resources. status changing with sync steps, and generate the last status by classify pods.
func (dfc *DisaggregatedFEController) initialFEStatus(ddc *v1.DorisDisaggregatedCluster) {
	if ddc.Status.FEStatus.Phase == v1.Reconciling || ddc.Status.FEStatus.Phase == v1.ScaleDownFailed || ddc.Status.FEStatus.Phase == v1.Scaling {
		return
	}
	feStatus := v1.FEStatus{
		Phase:     v1.Reconciling,
		ClusterId: fmt.Sprintf("%d", ddc.GetInstanceHashId()),
	}
	ddc.Status.FEStatus = feStatus
}

func (dfc *DisaggregatedFEController) reconcileStatefulset(ctx context.Context, st *appv1.StatefulSet, cluster *v1.DorisDisaggregatedCluster) (*sc.Event, error) {
	var est appv1.StatefulSet
	if err := dfc.K8sclient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, dfc.K8sclient, st); err != nil {
			klog.Errorf("disaggregatedFEController reconcileStatefulset create statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.FECreateResourceFailed, Message: err.Error()}, err
		}

		return nil, nil
	} else if err != nil {
		klog.Errorf("disaggregatedFEController reconcileStatefulset get statefulset failed, namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return nil, err
	}

	var replicas int32
	if cluster.Spec.FeSpec.Replicas != nil {
		replicas = *cluster.Spec.FeSpec.Replicas
	}
	electionNumber := cluster.GetElectionNumber()
	if replicas < electionNumber {
		dfc.K8srecorder.Event(cluster, string(sc.EventWarning), string(sc.FESpecSetError), "The number of disaggregated fe ElectionNumber is large than Replicas, Replicas has been corrected to the correct minimum value")
		klog.Errorf("disaggregatedFEController reconcileStatefulset disaggregatedDorisCluster namespace=%s,name=%s ,The number of disaggregated fe ElectionNumber(%d) is large than Replicas(%d)", cluster.Namespace, cluster.Name, electionNumber, *(cluster.Spec.FeSpec.Replicas))
		cluster.Spec.FeSpec.Replicas = &electionNumber
		st.Spec.Replicas = &electionNumber
	}

	// fe scale check and set FEStatus phase
	willRemovedAmount := replicas - *(est.Spec.Replicas)

	//  if fe scale, drop fe node by http
	if willRemovedAmount < 0 || cluster.Status.FEStatus.Phase == v1.ScaleDownFailed {
		if err := dfc.dropFEBySQLClient(ctx, dfc.K8sclient, cluster); err != nil {
			cluster.Status.FEStatus.Phase = v1.ScaleDownFailed
			klog.Errorf("ScaleDownFE failed, err:%s ", err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.FEHTTPFailed, Message: err.Error()},
				err
		}
		cluster.Status.FEStatus.Phase = v1.Scaling
	}

	// apply fe StatefulSet
	if err := k8s.ApplyStatefulSet(ctx, dfc.K8sclient, st, func(st, est *appv1.StatefulSet) bool {
		return resource.StatefulsetDeepEqualWithKey(st, est, v1.DisaggregatedSpecHashValueAnnotation, false)
	}); err != nil {
		klog.Errorf("disaggregatedFEController reconcileStatefulset apply statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.FEApplyResourceFailed, Message: err.Error()}, err
	}
	return nil, nil
}

// RecycleResources pvc resource for fe recycle
func (dfc *DisaggregatedFEController) recycleResources(ctx context.Context, ddc *v1.DorisDisaggregatedCluster) error {
	if ddc.Spec.FeSpec.PersistentVolume != nil {
		return dfc.listAndDeletePersistentVolumeClaim(ctx, ddc)
	}
	return nil
}

// dropFEBySQLClient only delete the fe nodes whose pod number is greater than the expected number (cluster.Spec.FeSpec.Replicas) by calling the drop_node interface
func (dfc *DisaggregatedFEController) dropFEBySQLClient(ctx context.Context, k8sclient client.Client, cluster *v1.DorisDisaggregatedCluster) error {
	// get adminuserName and pwd
	secret, _ := k8s.GetSecret(ctx, k8sclient, cluster.Namespace, cluster.Spec.AuthSecret)
	adminUserName, password := resource.GetDorisLoginInformation(secret)

	// get host and port
	// When the operator and dcr are deployed in different namespace, it will be inaccessible, so need to add the dcr svc namespace
	host := cluster.GetFEVIPAddresss()
	confMap := dfc.GetConfigValuesFromConfigMaps(cluster.Namespace, resource.FE_RESOLVEKEY, cluster.Spec.FeSpec.ConfigMaps)
	queryPort := resource.GetPort(confMap, resource.QUERY_PORT)

	// connect to doris sql to get master node
	// It may not be the master, or even the node that needs to be deleted, causing the deletion SQL to fail.
	dbConf := mysql.DBConfig{
		User:     adminUserName,
		Password: password,
		Host:     host,
		Port:     strconv.FormatInt(int64(queryPort), 10),
		Database: "mysql",
	}
	masterDBClient, err := mysql.NewDorisMasterSqlDB(dbConf)
	if err != nil {
		klog.Errorf("NewDorisMasterSqlDB failed, get fe node connection err:%s", err.Error())
		return err
	}
	defer masterDBClient.Close()

	allObserves, err := masterDBClient.GetObservers()
	if err != nil {
		klog.Errorf("dropFEFromSQLClient failed, GetObservers err:%s", err.Error())
		return err
	}

	// means: needRemovedAmount = allobservers - (replicas - election)
	electionNumber := cluster.GetElectionNumber()
	needRemovedAmount := int32(len(allObserves)) - *(cluster.Spec.FeSpec.Replicas) + electionNumber
	if needRemovedAmount <= 0 {
		klog.Errorf("dropFEFromSQLClient failed, Observers number(%d) is not larger than scale number(%d) ", len(allObserves), *(cluster.Spec.FeSpec.Replicas)-electionNumber)
		return nil
	}

	// get will delete Observes
	var frontendMap map[int]*mysql.Frontend // frontendMap key is fe pod index ,value is frontend
	stsName := cluster.GetFEStatefulsetName()

	if resource.GetStartMode(confMap) == resource.START_MODEL_FQDN { // use host
		frontendMap, err = mysql.BuildSeqNumberToFrontendMap(allObserves, nil, stsName)
		if err != nil {
			klog.Errorf("dropFEFromSQLClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	} else { // use ip
		podMap := make(map[string]string) // key is pod ip, value is pod name
		pods, err := k8s.GetPods(ctx, k8sclient, cluster.Namespace, dfc.getFEPodLabels(cluster))
		if err != nil {
			klog.Errorf("dropFEFromSQLClient failed, GetPods err:%s", err)
			return nil
		}
		for _, item := range pods.Items {
			if strings.HasPrefix(item.GetName(), stsName) {
				podMap[item.Status.PodIP] = item.GetName()
			}
		}
		frontendMap, err = mysql.BuildSeqNumberToFrontendMap(allObserves, podMap, stsName)
		if err != nil {
			klog.Errorf("dropFEFromSQLClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	}
	observes := mysql.FindNeedDeletedObservers(frontendMap, needRemovedAmount)
	// drop node and return
	return masterDBClient.DropObserver(observes)
}
