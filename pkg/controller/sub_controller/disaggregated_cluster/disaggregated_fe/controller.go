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
	"bytes"
	"context"
	"encoding/json"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_http"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

var _ sub_controller.DisaggregatedSubController = &DisaggregatedFEController{}

var (
	disaggregatedFEController = "disaggregatedFEController"
)

type DisaggregatedFEController struct {
	k8sClient      client.Client
	k8sRecorder    record.EventRecorder
	controllerName string
}

func New(mgr ctrl.Manager) *DisaggregatedFEController {
	return &DisaggregatedFEController{
		k8sClient:      mgr.GetClient(),
		k8sRecorder:    mgr.GetEventRecorderFor(disaggregatedFEController),
		controllerName: disaggregatedFEController,
	}
}

func (dfc *DisaggregatedFEController) Sync(ctx context.Context, obj client.Object) error {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)

	if *(ddc.Spec.FeSpec.Replicas) < DefaultFeReplicaNumber {
		klog.Errorf("disaggregatedFEController sync disaggregatedDorisCluster namespace=%s,name=%s ,The number of disaggregated fe replicas is illegal and has been corrected to the default value %d", ddc.Namespace, ddc.Name, DefaultFeReplicaNumber)
		dfc.k8sRecorder.Event(ddc, string(sub_controller.EventNormal), string(sub_controller.FESpecSetError), "The number of disaggregated fe replicas is illegal and has been corrected to the default value 2")
		ddc.Spec.FeSpec.Replicas = &DefaultFeReplicaNumber
	}

	confMap := dfc.getConfigValuesFromConfigMaps(ddc.Namespace, ddc.Spec.FeSpec.ConfigMaps)
	svc := dfc.newService(ddc, confMap)

	st := dfc.NewStatefulset(ddc, confMap)
	dfc.initialFEStatus(ddc)

	event, err := dfc.reconcileService(ctx, svc)
	if err != nil {
		if event != nil {
			dfc.k8sRecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
		}
		klog.Errorf("disaggregatedFEController reconcile service namespace %s name %s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return err
	}

	event, err = dfc.reconcileStatefulset(ctx, st, ddc)
	if err != nil {
		if event != nil {
			dfc.k8sRecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
		}
		klog.Errorf("disaggregatedFEController reconcile statefulset namespace %s name %s failed, err=%s", st.Namespace, st.Name, err.Error())
		return err
	}

	return nil
}

func (dfc *DisaggregatedFEController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)

	statefulsetName := ddc.GetFEStatefulsetName()
	serviceName := ddc.GetFEServiceName()

	if err := dfc.RecycleResources(ctx, ddc); err != nil {
		klog.Errorf("DisaggregatedFE ClearResources RecycleResources failed, namespace %s name %s, err:%s.", ddc.Namespace, ddc.Name, err.Error())
		return false, err
	}

	if ddc.DeletionTimestamp.IsZero() {
		return true, nil
	}

	if err := k8s.DeleteService(ctx, dfc.k8sClient, ddc.Namespace, serviceName); err != nil {
		klog.Errorf("disaggregatedFEController delete service namespace %s name %s failed, err=%s", ddc.Namespace, serviceName, err.Error())
		dfc.k8sRecorder.Event(ddc, string(sub_controller.EventWarning), string(sub_controller.FEServiceDeleteFailed), err.Error())
		return false, err
	}

	if err := k8s.DeleteStatefulset(ctx, dfc.k8sClient, ddc.Namespace, statefulsetName); err != nil {
		klog.Errorf("disaggregatedFEController delete statefulset namespace %s name %s failed, err=%s", ddc.Namespace, statefulsetName, err.Error())
		dfc.k8sRecorder.Event(ddc, string(sub_controller.EventWarning), string(sub_controller.FEStatefulsetDeleteFailed), err.Error())
		return false, err
	}

	return true, nil
}

func (dfc *DisaggregatedFEController) GetControllerName() string {
	return disaggregatedFEController
}

// podIsMaster if fe pod name has tail: '-0', is master
func (dfc *DisaggregatedFEController) podIsMaster(podName, stfName string) bool {
	if !strings.HasPrefix(podName, stfName+"-") {
		return false
	}
	suffix := podName[len(stfName)+1:]
	num, err := strconv.Atoi(suffix)
	if err != nil {
		return false
	}
	return num == 0
}

func (dfc *DisaggregatedFEController) UpdateComponentStatus(obj client.Object) error {
	var masterAliveReplicas int32
	var availableReplicas int32
	var creatingReplicas int32
	var failedReplicas int32

	ddc := obj.(*dv1.DorisDisaggregatedCluster)

	stfName := ddc.GetFEStatefulsetName()

	// FEStatus
	feSpec := ddc.Spec.FeSpec
	selector := dfc.newFEPodsSelector(ddc.Name)
	var podList corev1.PodList
	if err := dfc.k8sClient.List(context.Background(), &podList, client.InNamespace(ddc.Namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for _, pod := range podList.Items {

		if ready := k8s.PodIsReady(&pod.Status); ready {
			if dfc.podIsMaster(pod.Name, stfName) {
				masterAliveReplicas = 1
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
		ddc.Status.FEStatus.AvailableStatus = dv1.Available
		ddc.Status.ClusterHealth.FeAvailable = true
	}
	// all fe pods  are Ready, FEStatus.Phase is Ready，
	// for ClusterHealth.Health is green
	if masterAliveReplicas == DefaultElectionNumber && availableReplicas == *(feSpec.Replicas) {
		ddc.Status.FEStatus.Phase = dv1.Ready
	}

	return nil
}

// get compute start config from all configmaps that config in CR, resolve config for config ports in pod or service, etc.
func (dfc *DisaggregatedFEController) getConfigValuesFromConfigMaps(namespace string, cms []dv1.ConfigMap) map[string]interface{} {
	if len(cms) == 0 {
		return nil
	}

	for _, cm := range cms {
		kcm, err := k8s.GetConfigMap(context.Background(), dfc.k8sClient, namespace, cm.Name)
		if err != nil {
			klog.Errorf("disaggregatedFEController getConfigValuesFromConfigMaps namespace=%s, name=%s, failed, err=%s", namespace, cm.Name, err.Error())
			continue
		}

		if _, ok := kcm.Data[resource.FE_RESOLVEKEY]; !ok {
			continue
		}

		v := kcm.Data[resource.FE_RESOLVEKEY]
		viper.SetConfigType("properties")
		viper.ReadConfig(bytes.NewBuffer([]byte(v)))
		return viper.AllSettings()
	}

	return nil
}

// initial fe status before sync resources. status changing with sync steps, and generate the last status by classify pods.
func (dfc *DisaggregatedFEController) initialFEStatus(ddc *dv1.DorisDisaggregatedCluster) {
	if ddc.Status.FEStatus.Phase == dv1.Reconciling {
		return
	}
	feStatus := dv1.FEStatus{
		Phase:     dv1.Reconciling,
		ClusterId: ms_http.FeClusterId,
	}
	ddc.Status.FEStatus = feStatus
}

func (dfc *DisaggregatedFEController) reconcileService(ctx context.Context, svc *corev1.Service) (*sub_controller.Event, error) {
	var esvc corev1.Service
	if err := dfc.k8sClient.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, &esvc); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, dfc.k8sClient, svc); err != nil {
			klog.Errorf("disaggregatedFEController reconcileService create service namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
			return &sub_controller.Event{Type: sub_controller.EventWarning, Reason: sub_controller.FECreateResourceFailed, Message: err.Error()}, err
		}
	} else if err != nil {
		klog.Errorf("disaggregatedFEController reconcileService get service failed, namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return nil, err
	}

	if err := k8s.ApplyService(ctx, dfc.k8sClient, svc, func(nsvc, osvc *corev1.Service) bool {
		return resource.ServiceDeepEqualWithAnnoKey(nsvc, osvc, dv1.DisaggregatedSpecHashValueAnnotation)
	}); err != nil {
		klog.Errorf("disaggregatedFEController reconcileService apply service namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return &sub_controller.Event{Type: sub_controller.EventWarning, Reason: sub_controller.FECreateResourceFailed, Message: err.Error()}, err
	}

	return nil, nil
}

func (dfc *DisaggregatedFEController) reconcileStatefulset(ctx context.Context, st *appv1.StatefulSet, cluster *dv1.DorisDisaggregatedCluster) (*sub_controller.Event, error) {
	var est appv1.StatefulSet
	if err := dfc.k8sClient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, dfc.k8sClient, st); err != nil {
			klog.Errorf("disaggregatedFEController reconcileStatefulset create statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
			return &sub_controller.Event{Type: sub_controller.EventWarning, Reason: sub_controller.FECreateResourceFailed, Message: err.Error()}, err
		}

		return nil, nil
	} else if err != nil {
		klog.Errorf("disaggregatedFEController reconcileStatefulset get statefulset failed, namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return nil, err
	}

	// fe scale check and set FEStatus phase
	if cluster.Spec.FeSpec.Replicas == nil {
		cluster.Spec.FeSpec.Replicas = resource.GetInt32Pointer(0)
	}
	scaleNumber := *(cluster.Spec.FeSpec.Replicas) - *(est.Spec.Replicas)
	if scaleNumber != 0 { // set fe Phase as Reconciling
		cluster.Status.FEStatus.Phase = dv1.Reconciling
		// In Reconcile, it is possible that the status cannot be updated in time,
		// resulting in an error in the status judgment based on the last status,
		// so the status will be forced to modify here
		if err := k8s.SetClusterPhase(ctx, dfc.k8sClient, cluster.Name, cluster.Namespace, dv1.Reconciling, dv1.DisaggregatedFE, nil); err != nil {
			klog.Errorf("reconcileStatefulset SetClusterPhase 'Reconciling' failed err:%s ", err.Error())
			return &sub_controller.Event{Type: sub_controller.EventWarning, Reason: sub_controller.FEStatusUpdateFailed, Message: err.Error()}, err
		}
	}

	// apply fe StatefulSet
	if err := k8s.ApplyStatefulSet(ctx, dfc.k8sClient, st, func(st, est *appv1.StatefulSet) bool {
		return resource.StatefulsetDeepEqualWithOmitKey(st, est, dv1.DisaggregatedSpecHashValueAnnotation, true, false)
	}); err != nil {
		klog.Errorf("disaggregatedFEController reconcileStatefulset apply statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sub_controller.Event{Type: sub_controller.EventWarning, Reason: sub_controller.FEApplyResourceFailed, Message: err.Error()}, err
	}

	//  if fe scale, drop fe node by http
	if scaleNumber < 0 || cluster.Status.FEStatus.Phase == dv1.Reconciling {
		if err := dfc.dropFEFromHttpClient(cluster); err != nil {
			klog.Errorf("reconcileStatefulset dropFEFromHttpClient failed, err:%s ", err.Error())
			return &sub_controller.Event{Type: sub_controller.EventWarning, Reason: sub_controller.FEHTTPFailed, Message: err.Error()},
				err
		}
	}

	return nil, nil
}

// dropFEFromHttpClient only delete the fe nodes whose pod number is greater than the expected number (cluster.Spec.FeSpec.Replicas) by calling the drop_node interface
func (dfc *DisaggregatedFEController) dropFEFromHttpClient(cluster *dv1.DorisDisaggregatedCluster) error {
	feReplica := cluster.Spec.FeSpec.Replicas

	unionId := "1:" + cluster.GetInstanceId() + cluster.GetFEStatefulsetName() + "-0"

	feCluster, err := ms_http.GetFECluster(cluster.Status.MsEndpoint, cluster.Status.MsToken, cluster.GetInstanceId(), unionId)
	if err != nil {
		klog.Errorf("dropFEFromHttpClient GetFECluster failed, err:%s ", err.Error())
		return err
	}

	var dropNodes []*ms_http.NodeInfo
	for _, node := range feCluster {
		splitCloudUniqueIDArr := strings.Split(node.CloudUniqueID, "-")
		podNum, err := strconv.Atoi(splitCloudUniqueIDArr[len(splitCloudUniqueIDArr)-1])
		if err != nil {
			klog.Errorf("dropFEFromHttpClient splitCloudUniqueIDArr can not split CloudUniqueID : %s,err:%s", node.CloudUniqueID, err.Error())
			return err
		}
		if podNum >= int(*feReplica) {
			dropNodes = append(dropNodes, node)
		}

	}
	if len(dropNodes) == 0 {
		return nil
	}
	specifyCluster, err := ms_http.DropFENodes(cluster.Status.MsEndpoint, cluster.Status.MsToken, cluster.GetInstanceId(), dropNodes)
	if err != nil {
		klog.Errorf("dropFEFromHttpClient DropFENodes failed, err:%s ", err.Error())
		return err
	}

	if specifyCluster.Code != ms_http.SuccessCode {
		jsonData, _ := json.Marshal(specifyCluster)
		klog.Errorf("dropFEFromHttpClient DropFENodes response failed , response: %s", jsonData)
		return err
	}

	return nil
}

// RecycleResources pvc resource for fe recycle
func (dfc *DisaggregatedFEController) RecycleResources(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) error {
	if ddc.Spec.FeSpec.PersistentVolume != nil {
		return dfc.listAndDeletePersistentVolumeClaim(ctx, ddc)
	}
	return nil
}
