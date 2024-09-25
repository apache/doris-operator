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
	"fmt"
	"github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/disaggregated_ms/ms_http"
	"github.com/apache/doris-operator/pkg/common/utils/disaggregated_ms/ms_meta"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
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

const (
	ms_http_token_key = "http_token"
	instance_conf_key = "instance.conf"
	ms_conf_name      = "doris_cloud.conf"
)

type DisaggregatedFEController struct {
	sc.DisaggregatedSubDefaultController
	//record instance metadata for checking instance need create or update.
	instanceMeta map[string] /*instanceId*/ interface{}
}

func New(mgr ctrl.Manager) *DisaggregatedFEController {
	im := make(map[string]interface{})
	return &DisaggregatedFEController{
		DisaggregatedSubDefaultController: sc.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(disaggregatedFEController),
			ControllerName: disaggregatedFEController},
		instanceMeta: im,
	}
}

func (dfc *DisaggregatedFEController) Sync(ctx context.Context, obj client.Object) error {
	ddc := obj.(*v1.DorisDisaggregatedCluster)
	//TODO: check ms status
	if !dfc.msAvailable(ddc) {
		dfc.K8srecorder.Event(ddc, string(sc.EventNormal), string(sc.WaitMetaServiceAvailable), "meta service have not ready.")
		return nil
	}

	//get instance config, validating config, display some instance info in DorisDisaggregatedCluster, apply instance info into ms.
	if _, err := func() (ctrl.Result, error) {
		instanceConf, err := dfc.getInstanceConfig(ctx, ddc)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := dfc.validateInstanceInfo(instanceConf); err != nil {
			return ctrl.Result{}, err
		}
		//display InstanceInfo in DorisDisaggregatedCluster
		dfc.displayInstanceInfo(instanceConf, ddc)

		//TODO: wait interface fixed. realize update ak,sk.
		event, err := dfc.ApplyInstanceMeta(ddc.Status.MetaServiceStatus.MetaServiceEndpoint, ddc.Status.MetaServiceStatus.MsToken, instanceConf)
		if event != nil {
			dfc.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
		}
		return ctrl.Result{}, err
	}(); err != nil {
		return err
	}

	if ddc.Spec.FeSpec.Replicas == nil || *(ddc.Spec.FeSpec.Replicas) < DefaultFeReplicaNumber {
		klog.Errorf("disaggregatedFEController sync disaggregatedDorisCluster namespace=%s,name=%s ,The number of disaggregated fe replicas is illegal and has been corrected to the default value %d", ddc.Namespace, ddc.Name, DefaultFeReplicaNumber)
		dfc.K8srecorder.Event(ddc, string(sc.EventNormal), string(sc.FESpecSetError), "The number of disaggregated fe replicas is illegal and has been corrected to the default value 2")
		ddc.Spec.FeSpec.Replicas = &DefaultFeReplicaNumber
	}

	confMap := dfc.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.FE_RESOLVEKEY, ddc.Spec.FeSpec.ConfigMaps)
	svc := dfc.newService(ddc, confMap)

	st := dfc.NewStatefulset(ddc, confMap)
	dfc.initialFEStatus(ddc)

	event, err := dfc.DefaultReconcileService(ctx, svc)
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
		klog.Infof("DisaggregatedFEController Sync wait fe service name %s available occur failed %s\n", ddc.GetMSServiceName(), err.Error())
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

	statefulsetName := ddc.GetFEStatefulsetName()
	serviceName := ddc.GetFEServiceName()

	if err := dfc.recycleResources(ctx, ddc); err != nil {
		klog.Errorf("DisaggregatedFE ClearResources RecycleResources failed, namespace %s name %s, err:%s.", ddc.Namespace, ddc.Name, err.Error())
		return false, err
	}

	if ddc.DeletionTimestamp.IsZero() {
		return true, nil
	}

	if err := k8s.DeleteService(ctx, dfc.K8sclient, ddc.Namespace, serviceName); err != nil {
		klog.Errorf("disaggregatedFEController delete service namespace %s name %s failed, err=%s", ddc.Namespace, serviceName, err.Error())
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

	ddc := obj.(*v1.DorisDisaggregatedCluster)

	stfName := ddc.GetFEStatefulsetName()

	// FEStatus
	feSpec := ddc.Spec.FeSpec
	selector := dfc.newFEPodsSelector(ddc.Name)
	var podList corev1.PodList
	if err := dfc.K8sclient.List(context.Background(), &podList, client.InNamespace(ddc.Namespace), client.MatchingLabels(selector)); err != nil {
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
		ddc.Status.FEStatus.AvailableStatus = v1.Available
		ddc.Status.ClusterHealth.FeAvailable = true
	}
	// all fe pods  are Ready, FEStatus.Phase is Ready，
	// for ClusterHealth.Health is green
	if masterAliveReplicas == DefaultElectionNumber && availableReplicas == *(feSpec.Replicas) {
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
		ClusterId: ms_http.FeClusterId,
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

	// fe scale check and set FEStatus phase
	scaleNumber := *(cluster.Spec.FeSpec.Replicas) - *(est.Spec.Replicas)

	// apply fe StatefulSet
	if err := k8s.ApplyStatefulSet(ctx, dfc.K8sclient, st, func(st, est *appv1.StatefulSet) bool {
		return resource.StatefulsetDeepEqualWithOmitKey(st, est, v1.DisaggregatedSpecHashValueAnnotation, true, false)
	}); err != nil {
		klog.Errorf("disaggregatedFEController reconcileStatefulset apply statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.FEApplyResourceFailed, Message: err.Error()}, err
	}

	//  if fe scale, drop fe node by http
	if scaleNumber < 0 || cluster.Status.FEStatus.Phase == v1.ScaleDownFailed {
		if err := dfc.dropFEFromHttpClient(cluster); err != nil {
			cluster.Status.FEStatus.Phase = v1.ScaleDownFailed
			klog.Errorf("ScaleDownFE failed, err:%s ", err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.FEHTTPFailed, Message: err.Error()},
				err
		}
		cluster.Status.FEStatus.Phase = v1.Scaling
	}
	//dropped

	return nil, nil
}

// dropFEFromHttpClient only delete the fe nodes whose pod number is greater than the expected number (cluster.Spec.FeSpec.Replicas) by calling the drop_node interface
func (dfc *DisaggregatedFEController) dropFEFromHttpClient(cluster *v1.DorisDisaggregatedCluster) error {
	//TODO: cancle for new sql interface debug
	/*feReplica := cluster.Spec.FeSpec.Replicas

	unionId := "1:" + cluster.GetInstanceId() + ":" + cluster.GetFEStatefulsetName() + "-0"
	feCluster, err := ms_http.GetFECluster(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, cluster.GetInstanceId(), unionId)
	if err != nil {
		klog.Errorf("dropFEFromHttpClient GetFECluster failed, err:%s ", err.Error())
		return err
	}

	var dropNodes []*ms_http.NodeInfo
	for _, node := range feCluster {
		splitCloudUniqueIDArr := strings.Split(node.CloudUniqueID, "-")
		podNum, err := strconv.Atoi(splitCloudUniqueIDArr[len(splitCloudUniqueIDArr)-1])
		if err != nil {
			klog.Errorf("splitCloudUniqueIDArr can not split CloudUniqueID : %s,err:%s", node.CloudUniqueID, err.Error())
			return err
		}
		if podNum >= int(*feReplica) {
			dropNodes = append(dropNodes, node)
		}

	}
	if len(dropNodes) == 0 {
		return nil
	}
	specifyCluster, err := ms_http.DropFENodes(cluster.Status.MetaServiceStatus.MetaServiceEndpoint, cluster.Status.MetaServiceStatus.MsToken, cluster.GetInstanceId(), dropNodes)
	if err != nil {
		klog.Errorf("dropFEFromHttpClient DropFENodes failed, err:%s ", err.Error())
		return err
	}

	if specifyCluster.Code != ms_http.SuccessCode {
		jsonData, _ := json.Marshal(specifyCluster)
		klog.Errorf("dropFEFromHttpClient DropFENodes response failed , response: %s", jsonData)
		return err
	}
	*/
	return nil
}

// RecycleResources pvc resource for fe recycle
func (dfc *DisaggregatedFEController) recycleResources(ctx context.Context, ddc *v1.DorisDisaggregatedCluster) error {
	if ddc.Spec.FeSpec.PersistentVolume != nil {
		return dfc.listAndDeletePersistentVolumeClaim(ctx, ddc)
	}
	return nil
}

func (dfc *DisaggregatedFEController) createObjectInfo(endpoint, token string, instance map[string]interface{}) (*sc.Event, error) {
	str, _ := json.Marshal(instance)
	mr, err := ms_http.CreateInstance(endpoint, token, str)
	if err != nil {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.MSInteractError, Message: err.Error()}, errors.New("createObjectInfo failed, err " + err.Error())
	}
	if mr.Code != ms_http.SuccessCode && mr.Code != ms_http.ALREADY_EXIST {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.ObjectConfigError, Message: mr.Msg}, errors.New("createObjectInfo " + mr.Code + mr.Msg)
	}

	return &sc.Event{Type: sc.EventNormal, Reason: sc.InstanceMetaCreated}, nil
}

func (dfc *DisaggregatedFEController) validateInstanceInfo(instanceConf map[string]interface{}) error {
	idv := instanceConf[ms_meta.Instance_id]
	if idv == nil {
		return errors.New("not config instance id")
	}
	id, ok := idv.(string)
	if !ok || id == "" {
		return errors.New("not config instance id")
	}
	return dfc.validateVaultInfo(instanceConf)
}

func (dfc *DisaggregatedFEController) validateVaultInfo(instanceConf map[string]interface{}) error {
	vi := instanceConf[ms_meta.Vault]
	if vi == nil {
		return errors.New("have not vault config")
	}

	vault, ok := vi.(map[string]interface{})
	if !ok {
		return errors.New("vault not json format")
	}

	if obj, ok := vault[ms_meta.Obj_info]; ok {
		objMap, ok := obj.(map[string]interface{})
		if !ok {
			return errors.New("obj_info not json format")
		}

		return dfc.validateS3(objMap)
	}

	if i, ok := vault[ms_meta.Key_hdfs_info]; ok {
		hdfsMap, ok := i.(map[string]interface{})
		if !ok {
			return errors.New("hdfs not json format")
		}
		return dfc.validateHDFS(hdfsMap)
	}

	return errors.New("s3 and hdfs all empty")
}

func (dfc *DisaggregatedFEController) validateHDFS(m map[string]interface{}) error {
	if err := dfc.validateString(m, ms_meta.Key_hdfs_info_prefix); err != nil {
		return err
	}

	if err := dfc.validateString(m, ms_meta.Key_hdfs_info_build_conf); err != nil {
		return err
	}
	bv := m[ms_meta.Key_hdfs_info_build_conf]
	bm, ok := bv.(map[string]interface{})
	if !ok {
		return errors.New("hdfs build_conf not json format")
	}

	if err := dfc.validateString(bm, ms_meta.Key_hdfs_info_build_conf_fs_name); err != nil {
		return err
	}
	if err := dfc.validateString(bm, ms_meta.Key_hdfs_info_build_conf_user); err != nil {
		return err
	}
	return nil
}

func (dc *DisaggregatedFEController) validateS3(m map[string]interface{}) error {
	cks := []string{ms_meta.Obj_info_ak, ms_meta.Obj_info_sk, ms_meta.Obj_info_bucket, ms_meta.Obj_info_prefix, ms_meta.Obj_info_prefix}
	msg := ""
	for _, ck := range cks {
		if err := dc.validateString(m, ck); err != nil {
			msg += err.Error() + ";"
		}
	}

	if msg == "" {
		return nil
	}

	return errors.New(msg)
}

func (dfc *DisaggregatedFEController) validateString(m map[string]interface{}, key string) error {
	v := m[key]
	if v == nil {
		return errors.New("not config")
	}
	str, ok := v.(string)
	if !ok || str == "" {
		return errors.New("not string or empty")
	}
	return nil
}

func (dfc *DisaggregatedFEController) CreateOrUpdateObjectMeta(endpoint, token string, instance map[string]interface{}) (*sc.Event, error) {
	idv := instance[ms_meta.Instance_id]
	instanceId := idv.(string)
	if !dfc.haveCreated(instanceId) {
		return dfc.createObjectInfo(endpoint, token, instance)
	}

	// if not match in memory, should compare with ms.
	if !dfc.isModified(instance) {
		return nil, nil
	}

	return nil, nil
}

func (dfc *DisaggregatedFEController) displayInstanceInfo(instanceConf map[string]interface{}, ddc *v1.DorisDisaggregatedCluster) {
	instanceId := (instanceConf[ms_meta.Instance_id]).(string)
	ddc.Status.InstanceId = instanceId
}

func (dfc *DisaggregatedFEController) getInstanceConfig(ctx context.Context, ddc *v1.DorisDisaggregatedCluster) (map[string]interface{}, error) {
	if ddc.Spec.InstanceConfigMap == "" {
		dfc.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.ObjectInfoInvalid), "vaultConfigmap should config a configMap that have object info.")
		return nil, errors.New("vaultConfigmap for object info should be specified")
	}

	cmName := ddc.Spec.InstanceConfigMap
	var cm corev1.ConfigMap
	if err := dfc.K8sclient.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: cmName}, &cm); err != nil {
		dfc.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.ObjectInfoInvalid), fmt.Sprintf("name %s configmap get failed, err=%s", cmName, err.Error()))
		return nil, err
	}

	if _, ok := cm.Data[instance_conf_key]; !ok {
		dfc.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.ObjectInfoInvalid), fmt.Sprintf("%s configmap data have not config key %s for object info.", cmName, instance_conf_key))
		return nil, errors.New(fmt.Sprintf("%s configmap data have not config key %s for object info.", cmName, instance_conf_key))
	}

	v := cm.Data[instance_conf_key]
	instance := map[string]interface{}{}
	err := json.Unmarshal([]byte(v), &instance)
	if err != nil {
		dfc.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.ObjectInfoInvalid), fmt.Sprintf("json unmarshal error=%s", err.Error()))
		return nil, err
	}

	return instance, nil
}

func (dfc *DisaggregatedFEController) ApplyInstanceMeta(endpoint, token string, instanceConf map[string]interface{}) (*sc.Event, error) {

	instanceId := (instanceConf[ms_meta.Instance_id]).(string)
	event, err := dfc.CreateOrUpdateObjectMeta(endpoint, token, instanceConf)
	if err != nil {
		return event, err
	}

	// store instance info for next update ak, sk etc...
	dfc.instanceMeta[instanceId] = instanceConf
	return nil, nil
}

func (dfc *DisaggregatedFEController) isModified(instance map[string]interface{}) bool {
	//TODO: the kernel interface not fixed, now not provided update function.
	return false
}

func (dfc *DisaggregatedFEController) haveCreated(instanceId string) bool {
	_, ok := dfc.instanceMeta[instanceId]
	//TODO: get from ms check
	return ok
}
