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

package sub_controller

import (
	"context"
	"fmt"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	utils "github.com/apache/doris-operator/pkg/common/utils"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	"github.com/apache/doris-operator/pkg/common/utils/set"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"
)

type SubController interface {
	//Sync reconcile for sub controller. bool represent the component have updated.
	Sync(ctx context.Context, cluster *dorisv1.DorisCluster) error
	//clear all resource about sub-component.
	ClearResources(ctx context.Context, cluster *dorisv1.DorisCluster) (bool, error)

	//return the controller name, beController, feController,cnController for log.
	GetControllerName() string

	//UpdateStatus update the component status on src.
	UpdateComponentStatus(cluster *dorisv1.DorisCluster) error
}

// SubDefaultController define common function for all component about doris.
type SubDefaultController struct {
	K8sclient   client.Client
	K8srecorder record.EventRecorder
}

func (d *SubDefaultController) CheckRestartTimeAndInject(dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) bool {
	var baseSpec *dorisv1.BaseSpec
	var restartedAt string
	var restartAnnotationsKey string
	switch componentType {
	case dorisv1.Component_FE:
		baseSpec = &dcr.Spec.FeSpec.BaseSpec
		restartedAt = dcr.Annotations[dorisv1.FERestartAt]
		restartAnnotationsKey = dorisv1.FERestartAt
	case dorisv1.Component_BE:
		baseSpec = &dcr.Spec.BeSpec.BaseSpec
		restartedAt = dcr.Annotations[dorisv1.BERestartAt]
		restartAnnotationsKey = dorisv1.BERestartAt
	default:
		klog.Errorf("CheckRestartTimeAndInject dorisClusterName %s, namespace %s componentType %s not supported.", dcr.Name, dcr.Namespace, componentType)
	}

	if restartedAt == "" {
		return false
	}

	// run shell: date +"%Y-%m-%dT%H:%M:%S%:z"
	// "2024-11-21T11:08:56+08:00"
	parseTime, err := time.Parse(time.RFC3339, restartedAt)
	if err != nil {
		checkErr := fmt.Errorf("CheckRestartTimeAndInject error: time format is incorrect. dorisClusterName: %s, namespace: %s, componentType %s, wrong parse 'restartedAt': %s , error: %s", dcr.Name, dcr.Namespace, componentType, restartedAt, err.Error())
		klog.Error(checkErr.Error())
		d.K8srecorder.Event(dcr, string(EventWarning), string(RestartTimeInvalid), checkErr.Error())
		return false
	}

	effectiveStartTime := time.Now().Add(-10 * time.Minute)

	if effectiveStartTime.After(parseTime) {
		klog.Errorf("CheckRestartTimeAndInject The time has expired, dorisClusterName: %s, namespace: %s, componentType %s, wrong parse 'restartedAt': %s : The time has expired, if you want to restart doris, please set a future time", dcr.Name, dcr.Namespace, componentType, restartedAt)
		d.K8srecorder.Event(dcr, string(EventWarning), string(RestartTimeInvalid), fmt.Sprintf("the %s restart time is not effective. the 'restartedAt' %s can't be earlier than 10 minutes before the current time", componentType, restartedAt))
		return false
	}

	// check passed, set annotations to doriscluster baseSpec
	if baseSpec.Annotations == nil {
		baseSpec.Annotations = make(map[string]string)
	}
	baseSpec.Annotations[restartAnnotationsKey] = restartedAt
	return true
}

// UpdateStatus update the component status on src.
func (d *SubDefaultController) UpdateStatus(namespace string, status *dorisv1.ComponentStatus, labels map[string]string, replicas int32, componentType dorisv1.ComponentType) error {
	return d.ClassifyPodsByStatus(namespace, status, labels, replicas, componentType)
}

func (d *SubDefaultController) ClassifyPodsByStatus(namespace string, status *dorisv1.ComponentStatus, labels map[string]string, replicas int32, componentType dorisv1.ComponentType) error {
	var podList corev1.PodList
	if err := d.K8sclient.List(context.Background(), &podList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return err
	}

	var creatings, readys, faileds []string
	podmap := make(map[string]corev1.Pod)

	if len(podList.Items) == 0 {
		return nil
	}

	restartAnnotationsKey := dorisv1.GetRestartAnnotationKey(componentType)
	firstRestartAnnotation := podList.Items[0].Annotations[restartAnnotationsKey]

	//get all pod status that controlled by st.
	stsRollingRestartAnnotationsSameCheck := true
	for _, pod := range podList.Items {
		if pod.Annotations[restartAnnotationsKey] != firstRestartAnnotation {
			stsRollingRestartAnnotationsSameCheck = false
		}
		podmap[pod.Name] = pod
		if ready := k8s.PodIsReady(&pod.Status); ready {
			readys = append(readys, pod.Name)
		} else if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			creatings = append(creatings, pod.Name)
		} else {
			faileds = append(faileds, pod.Name)
		}
	}

	if len(readys) == int(replicas) && stsRollingRestartAnnotationsSameCheck {
		status.ComponentCondition.Phase = dorisv1.Available
	} else if len(faileds) != 0 {
		status.ComponentCondition.Phase = dorisv1.HaveMemberFailed
		status.ComponentCondition.Reason = podmap[faileds[0]].Status.Reason
		status.ComponentCondition.Message = podmap[faileds[0]].Status.Message
	} else if len(creatings) != 0 {
		status.ComponentCondition.Reason = podmap[creatings[0]].Status.Reason
		status.ComponentCondition.Message = podmap[creatings[0]].Status.Message
	}

	status.RunningMembers = readys
	status.FailedMembers = faileds
	status.CreatingMembers = creatings
	return nil
}

func (d *SubDefaultController) GetConfig(ctx context.Context, configMapInfo *dorisv1.ConfigMapInfo, namespace string, componentType dorisv1.ComponentType) (map[string]interface{}, error) {
	config, err := k8s.GetConfig(ctx, d.K8sclient, configMapInfo, namespace, componentType)
	if err != nil {
		klog.Errorf("SubDefaultController GetConfig get configmap failed, namespace: %s,err: %s \n", namespace, err.Error())
	}
	return config, nil
}

// generate map for mountpath:configmap
func (d *SubDefaultController) CheckConfigMountPath(dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) {
	var configMapInfo dorisv1.ConfigMapInfo
	switch componentType {
	case dorisv1.Component_FE:
		configMapInfo = dcr.Spec.FeSpec.ConfigMapInfo
	case dorisv1.Component_BE:
		configMapInfo = dcr.Spec.BeSpec.ConfigMapInfo
	case dorisv1.Component_CN:
		configMapInfo = dcr.Spec.CnSpec.ConfigMapInfo
	case dorisv1.Component_Broker:
		configMapInfo = dcr.Spec.BrokerSpec.ConfigMapInfo
	default:
		klog.Infof("the componentType %s is not supported.", componentType)
	}
	cms := resource.GetMountConfigMapInfo(configMapInfo)
	var mountsMap = make(map[string]dorisv1.MountConfigMapInfo)
	for _, cm := range cms {
		path := cm.MountPath
		if m, exist := mountsMap[path]; exist {
			klog.Errorf("CheckConfigMountPath error: the mountPath %s is repeated between configmap: %s and configmap: %s.", path, cm.ConfigMapName, m.ConfigMapName)
			d.K8srecorder.Event(dcr, string(EventWarning), string(ConfigMapPathRepeated), fmt.Sprintf("the mountPath %s is repeated between configmap: %s and configmap: %s.", path, cm.ConfigMapName, m.ConfigMapName))
		}
		mountsMap[path] = cm
	}
}

// generate map for mountpath:secret
func (d *SubDefaultController) CheckSecretMountPath(dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) {
	var secrets []dorisv1.Secret
	switch componentType {
	case dorisv1.Component_FE:
		secrets = dcr.Spec.FeSpec.Secrets
	case dorisv1.Component_BE:
		secrets = dcr.Spec.BeSpec.Secrets
	case dorisv1.Component_CN:
		secrets = dcr.Spec.CnSpec.Secrets
	case dorisv1.Component_Broker:
		secrets = dcr.Spec.BrokerSpec.Secrets
	default:
		klog.Infof("the componentType %s is not supported.", componentType)
	}
	var mountsMap = make(map[string]dorisv1.Secret)
	for _, secret := range secrets {
		path := secret.MountPath
		if s, exist := mountsMap[path]; exist {
			klog.Errorf("CheckSecretMountPath error: the mountPath %s is repeated between secret: %s and secret: %s.", path, secret.SecretName, s.SecretName)
			d.K8srecorder.Event(dcr, string(EventWarning), string(SecretPathRepeated), fmt.Sprintf("the mountPath %s is repeated between secret: %s and secret: %s.", path, secret.SecretName, s.SecretName))
		}
		mountsMap[path] = secret
	}
}

// CheckSecretExist, check the secret exist or not in specify namespace.
func (d *SubDefaultController) CheckSecretExist(ctx context.Context, dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) {
	var secrets []dorisv1.Secret
	switch componentType {
	case dorisv1.Component_FE:
		secrets = dcr.Spec.FeSpec.Secrets
	case dorisv1.Component_BE:
		secrets = dcr.Spec.BeSpec.Secrets
	case dorisv1.Component_CN:
		secrets = dcr.Spec.CnSpec.Secrets
	case dorisv1.Component_Broker:
		secrets = dcr.Spec.BrokerSpec.Secrets
	default:
		klog.Infof("the componentType %s is not supported.", componentType)
	}
	errMessage := ""
	for _, secret := range secrets {
		var s corev1.Secret
		if getErr := d.K8sclient.Get(ctx, types.NamespacedName{Namespace: dcr.Namespace, Name: secret.SecretName}, &s); getErr != nil {
			errMessage = errMessage + fmt.Sprintf("(name: %s, namespace: %s, err: %s), ", secret.SecretName, dcr.Namespace, getErr.Error())
		}
	}
	if errMessage != "" {
		klog.Errorf("CheckSecretExist error: %s.", errMessage)
		d.K8srecorder.Event(dcr, string(EventWarning), string(SecretNotExist), fmt.Sprintf("CheckSecretExist error: %s.", errMessage))
	}
}

// ClearCommonResources clear common resources all component have, as statefulset, service.
// response `bool` represents all resource have deleted, if not and delete resource failed return false for next reconcile retry.
func (d *SubDefaultController) ClearCommonResources(ctx context.Context, dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) (bool, error) {
	//if the doris is not have cn.
	stName := dorisv1.GenerateComponentStatefulSetName(dcr, componentType)
	externalServiceName := dorisv1.GenerateExternalServiceName(dcr, componentType)
	internalServiceName := dorisv1.GenerateInternalCommunicateServiceName(dcr, componentType)
	if err := k8s.DeleteStatefulset(ctx, d.K8sclient, dcr.Namespace, stName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("SubDefaultController ClearResources delete statefulset failed, namespace=%s,name=%s, error=%s.", dcr.Namespace, stName, err.Error())
		return false, err
	}

	if err := k8s.DeleteService(ctx, d.K8sclient, dcr.Namespace, internalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("SubDefaultController ClearResources delete search service, namespace=%s,name=%s,error=%s.", dcr.Namespace, internalServiceName, err.Error())
		return false, err
	}
	if err := k8s.DeleteService(ctx, d.K8sclient, dcr.Namespace, externalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("SubDefaultController ClearResources delete external service, namespace=%s, name=%s,error=%s.", dcr.Namespace, externalServiceName, err.Error())
		return false, err
	}

	return true, nil
}

func (d *SubDefaultController) FeAvailable(dcr *dorisv1.DorisCluster) bool {
	addr, _ := dorisv1.GetConfigFEAddrForAccess(dcr, dorisv1.Component_BE)
	if addr != "" {
		return true
	}

	//if fe deploy in k8s, should wait fe available
	//1. wait for fe ok.
	endpoints := corev1.Endpoints{}
	if err := d.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: dcr.Namespace, Name: dorisv1.GenerateExternalServiceName(dcr, dorisv1.Component_FE)}, &endpoints); err != nil {
		klog.Infof("SubDefaultController Sync wait fe service name %s available occur failed %s\n", dorisv1.GenerateExternalServiceName(dcr, dorisv1.Component_FE), err.Error())
		return false
	}

	for _, sub := range endpoints.Subsets {
		if len(sub.Addresses) > 0 {
			return true
		}
	}
	return false
}

// RestrictConditionsEqual adds two StatefulSet,
// It is used to control the conditions for comparing.
// nst StatefulSet - a new StatefulSet
// est StatefulSet - an old StatefulSet
func (d *SubDefaultController) RestrictConditionsEqual(nst *appv1.StatefulSet, est *appv1.StatefulSet) {
	//shield persistent volume update when the pvcProvider=Operator
	//in webhook should intercept the volume spec updated when use statefulset pvc.
	// TODO: updates to statefulset spec for fields other than 'replicas', 'template', 'updateStrategy', 'persistentVolumeClaimRetentionPolicy' and 'minReadySeconds' are forbidden
	nst.Spec.VolumeClaimTemplates = est.Spec.VolumeClaimTemplates
}

// PrepareReconcileResources prepare resource for reconcile
// response: bool, if true presents resource have ready for reconciling, if false presents resource is preparing.
func (d *SubDefaultController) PrepareReconcileResources(ctx context.Context, dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) bool {
	switch componentType {
	case dorisv1.Component_FE:
		return d.prepareFEReconcileResources(ctx, dcr)
	case dorisv1.Component_BE:
		return d.prepareBEReconcileResources(ctx, dcr)
	case dorisv1.Component_CN:
		return d.prepareCNReconcileResources(ctx, dcr)
	default:
		klog.Infof("prepareReconcileResource not support type= %s", componentType)
		return true
	}
}

// prepareFEReconcileResources prepare resource for fe reconcile
// response: bool, if true presents resource have ready for fe reconciling, if false presents resource is preparing.
func (d *SubDefaultController) prepareFEReconcileResources(ctx context.Context, dcr *dorisv1.DorisCluster) bool {
	if len(dcr.Spec.FeSpec.PersistentVolumes) != 0 {
		return d.preparePersistentVolumeClaim(ctx, dcr, dorisv1.Component_FE)
	}

	return true
}

// prepareBEReconcileResources prepare resource for be reconcile
// response: bool, if true presents resource have ready for be reconciling, if false presents resource is preparing.
func (d *SubDefaultController) prepareBEReconcileResources(ctx context.Context, dcr *dorisv1.DorisCluster) bool {
	if len(dcr.Spec.BeSpec.PersistentVolumes) != 0 {
		return d.preparePersistentVolumeClaim(ctx, dcr, dorisv1.Component_BE)
	}

	return true
}

// prepareCNReconcileResources prepare resource for cn reconcile
// response: bool, if true presents resource have ready for cn reconciling, if false presents resource is preparing.
func (d *SubDefaultController) prepareCNReconcileResources(ctx context.Context, dcr *dorisv1.DorisCluster) bool {
	if len(dcr.Spec.CnSpec.PersistentVolumes) != 0 {
		return d.preparePersistentVolumeClaim(ctx, dcr, dorisv1.Component_CN)
	}

	return true
}

// 1. list pvcs, create or update,
// 1.1 labels use statefulset selector.
// 2. classify pvcs by dorisv1.PersistentVolume.name
// 2.1 travel pvcs, use key="-^"+volume.name, value=pvc put into map. starting with "-^" as the k8s resource name not allowed start with it.
func (d *SubDefaultController) preparePersistentVolumeClaim(ctx context.Context, dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) bool {
	var volumes []dorisv1.PersistentVolume
	var replicas int32
	switch componentType {
	case dorisv1.Component_FE:
		volumes = dcr.Spec.FeSpec.PersistentVolumes
		replicas = *dcr.Spec.FeSpec.Replicas
	case dorisv1.Component_BE:
		volumes = dcr.Spec.BeSpec.PersistentVolumes
		replicas = *dcr.Spec.BeSpec.Replicas
	case dorisv1.Component_CN:
		volumes = dcr.Spec.CnSpec.PersistentVolumes
		replicas = *dcr.Spec.CnSpec.Replicas
	default:
	}

	pvcList := corev1.PersistentVolumeClaimList{}
	selector := dorisv1.GenerateStatefulSetSelector(dcr, componentType)
	stsName := dorisv1.GenerateComponentStatefulSetName(dcr, componentType)
	if err := d.K8sclient.List(ctx, &pvcList, client.InNamespace(dcr.Namespace), client.MatchingLabels(selector)); err != nil {
		d.K8srecorder.Event(dcr, string(EventWarning), PVCListFailed, string("list component "+componentType+" failed!"))
		return false
	}
	//classify pvc by volume.Name, pvc.name generate by volume.Name + statefulset.Name + ordinal
	pvcMap := make(map[string][]corev1.PersistentVolumeClaim)

	for _, pvc := range pvcList.Items {
		//start with unique string for classify pvc, avoid empty string match all pvc.Name
		key := "-^"
		for _, volume := range volumes {
			if volume.Name != "" && strings.HasPrefix(pvc.Name, volume.Name) {
				key = key + volume.Name
				break
			}
		}

		if _, ok := pvcMap[key]; !ok {
			pvcMap[key] = []corev1.PersistentVolumeClaim{}
		}
		pvcMap[key] = append(pvcMap[key], pvc)
	}

	//presents the pvc have all created or updated to new version.
	prepared := true
	for _, volume := range volumes {
		// if provider not `operator` should not manage pvc.
		if volume.PVCProvisioner != dorisv1.PVCProvisionerOperator {
			continue
		}

		if !d.patchPVCs(ctx, dcr, selector, pvcMap["-^"+volume.Name], stsName, volume, replicas) {
			prepared = false
		}
	}

	return prepared
}

func (d *SubDefaultController) patchPVCs(ctx context.Context, dcr *dorisv1.DorisCluster, selector map[string]string,
	pvcs []corev1.PersistentVolumeClaim, stsName string, volume dorisv1.PersistentVolume, replicas int32) bool {
	//patch already exist in k8s .
	prepared := true
	for _, pvc := range pvcs {
		oldCapacity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		newCapacity := volume.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage]
		if !oldCapacity.Equal(newCapacity) {
			// if pvc need update, the resource have not prepared, return false.
			prepared = false
			eventType := EventNormal
			reason := PVCUpdate
			message := pvc.Name + " update successfully!"
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = newCapacity
			if err := d.K8sclient.Patch(ctx, &pvc, client.Merge); err != nil {
				klog.Errorf("SubDefaultController namespace %s name %s patch pvc %s failed, %s", dcr.Namespace, dcr.Name, pvc.Name, err.Error())
				eventType = EventWarning
				reason = PVCUpdateFailed
				message = pvc.Name + " update failed, " + err.Error()
			}

			d.K8srecorder.Event(dcr, string(eventType), reason, message)
		}
	}

	// if need add new pvc, the resource prepared not finished, return false.
	if len(pvcs) < int(replicas) {
		prepared = false
		d.K8srecorder.Event(dcr, string(EventNormal), PVCCreate, fmt.Sprintf("create PVC ordinal %d - %d", len(pvcs), replicas))
	}

	baseOrdinal := len(pvcs)
	for ; baseOrdinal < int(replicas); baseOrdinal++ {
		pvc := resource.BuildPVC(volume, selector, dcr.Namespace, stsName, strconv.Itoa(baseOrdinal))
		if err := d.K8sclient.Create(ctx, &pvc); err != nil && !apierrors.IsAlreadyExists(err) {
			d.K8srecorder.Event(dcr, string(EventWarning), PVCCreateFailed, err.Error())
			klog.Errorf("SubDefaultController namespace %s name %s create pvc %s failed, %s.", dcr.Namespace, dcr.Name, pvc.Name, err.Error())
		}
	}

	return prepared
}

// RecycleResources pvc resource for recycle
func (d *SubDefaultController) RecycleResources(ctx context.Context, dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) error {
	switch componentType {
	case dorisv1.Component_FE:
		return d.recycleFEResources(ctx, dcr)
	default:
		klog.Infof("RecycleResources not support type=%s", componentType)
		return nil
	}
}

// recycleFEResources pvc resource for fe recycle
func (d *SubDefaultController) recycleFEResources(ctx context.Context, dcr *dorisv1.DorisCluster) error {
	if len(dcr.Spec.FeSpec.PersistentVolumes) != 0 {
		return d.listAndDeletePersistentVolumeClaim(ctx, dcr, dorisv1.Component_FE)
	}
	return nil
}

// listAndDeletePersistentVolumeClaim:
// 1. list pvcs by statefulset selector labels .
// 2. get pvcs by dorisv1.PersistentVolume.name
// 2.1 travel pvcs, use key="-^"+volume.name, value=pvc put into map. starting with "-^" as the k8s resource name not allowed start with it.
// 3. delete pvc
func (d *SubDefaultController) listAndDeletePersistentVolumeClaim(ctx context.Context, dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) error {
	var volumes []dorisv1.PersistentVolume
	var replicas int32
	switch componentType {
	case dorisv1.Component_FE:
		volumes = dcr.Spec.FeSpec.PersistentVolumes
		replicas = *dcr.Spec.FeSpec.Replicas
	case dorisv1.Component_BE:
		volumes = dcr.Spec.BeSpec.PersistentVolumes
		replicas = *dcr.Spec.BeSpec.Replicas
	case dorisv1.Component_CN:
		volumes = dcr.Spec.CnSpec.PersistentVolumes
		replicas = *dcr.Spec.CnSpec.Replicas
	default:
	}

	pvcList := corev1.PersistentVolumeClaimList{}
	selector := dorisv1.GenerateStatefulSetSelector(dcr, componentType)
	stsName := dorisv1.GenerateComponentStatefulSetName(dcr, componentType)
	if err := d.K8sclient.List(ctx, &pvcList, client.InNamespace(dcr.Namespace), client.MatchingLabels(selector)); err != nil {
		d.K8srecorder.Event(dcr, string(EventWarning), PVCListFailed, string("list component "+componentType+" failed!"))
		return err
	}
	//classify pvc by volume.Name, pvc.name generate by volume.Name + statefulset.Name + ordinal
	pvcMap := make(map[string][]corev1.PersistentVolumeClaim)

	for _, pvc := range pvcList.Items {
		//start with unique string for classify pvc, avoid empty string match all pvc.Name
		key := "-^"
		for _, volume := range volumes {
			if volume.Name != "" && strings.HasPrefix(pvc.Name, volume.Name) {
				key = key + volume.Name
				break
			}
		}

		if _, ok := pvcMap[key]; !ok {
			pvcMap[key] = []corev1.PersistentVolumeClaim{}
		}
		pvcMap[key] = append(pvcMap[key], pvc)
	}

	var mergeError error
	for _, volume := range volumes {
		// Clean up the existing PVC that is larger than expected
		claims := pvcMap["-^"+volume.Name]
		if len(claims) <= int(replicas) {
			continue
		}
		if err := d.deletePVCs(ctx, dcr, selector, len(claims), stsName, volume.Name, replicas); err != nil {
			mergeError = utils.MergeError(mergeError, err)
		}
	}
	return mergeError
}

// deletePVCs will Loop to remove excess pvc
func (d *SubDefaultController) deletePVCs(ctx context.Context, dcr *dorisv1.DorisCluster, selector map[string]string,
	pvcSize int, stsName, volumeName string, replicas int32) error {
	maxOrdinal := pvcSize

	var mergeError error
	for ; maxOrdinal > int(replicas); maxOrdinal-- {
		pvcName := resource.BuildPVCName(stsName, strconv.Itoa(maxOrdinal-1), volumeName)
		if err := k8s.DeletePVC(ctx, d.K8sclient, dcr.Namespace, pvcName, selector); err != nil {
			d.K8srecorder.Event(dcr, string(EventWarning), PVCDeleteFailed, err.Error())
			klog.Errorf("SubController namespace %s name %s delete pvc %s failed, %s.", dcr.Namespace, dcr.Name, pvcName, err.Error())
			mergeError = utils.MergeError(mergeError, err)
		}
	}
	return mergeError
}

func (d *SubDefaultController) InitStatus(dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) {
	switch componentType {
	case dorisv1.Component_FE:
		d.initFEStatus(dcr)
	case dorisv1.Component_BE:
		d.initBEStatus(dcr)
	default:
		klog.Infof("InitStatus not support type= %s", componentType)
	}
}

func (d *SubDefaultController) initFEStatus(cluster *dorisv1.DorisCluster) {
	initPhase := dorisv1.Initializing
	// When in the Change phase, the state should inherit the last state instead of using the default state. Prevent incorrect Initializing of the change state
	if cluster.Status.FEStatus != nil && dorisv1.IsReconcilingStatusPhase(cluster.Status.FEStatus) {
		initPhase = cluster.Status.FEStatus.ComponentCondition.Phase
	}

	status := &dorisv1.ComponentStatus{
		ComponentCondition: dorisv1.ComponentCondition{
			SubResourceName:    dorisv1.GenerateComponentStatefulSetName(cluster, dorisv1.Component_FE),
			Phase:              initPhase,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}
	status.AccessService = dorisv1.GenerateExternalServiceName(cluster, dorisv1.Component_FE)
	cluster.Status.FEStatus = status
}

func (d *SubDefaultController) initBEStatus(cluster *dorisv1.DorisCluster) {
	initPhase := dorisv1.Initializing
	// When in the Change phase, the state should inherit the last state instead of using the default state. Prevent incorrect Initializing of the change state
	if cluster.Status.BEStatus != nil && dorisv1.IsReconcilingStatusPhase(cluster.Status.BEStatus) {
		initPhase = cluster.Status.BEStatus.ComponentCondition.Phase
	}

	status := &dorisv1.ComponentStatus{
		ComponentCondition: dorisv1.ComponentCondition{
			SubResourceName:    dorisv1.GenerateComponentStatefulSetName(cluster, dorisv1.Component_BE),
			Phase:              initPhase,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}
	status.AccessService = dorisv1.GenerateExternalServiceName(cluster, dorisv1.Component_BE)
	cluster.Status.BEStatus = status
}

// BuildCoreConfigmapStatusHash
// resolve configmap for doris core configuration file (fe.conf/be.conf),
// After parsing the configuration file, it is converted into a configured map,
// And return the map's hash
func (d *SubDefaultController) BuildCoreConfigmapStatusHash(ctx context.Context, dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) string {
	names := resource.GetDorisCoreConfigMapNames(dcr)
	cmName := names[componentType]
	if cmName != "" {
		cm, err := k8s.GetConfigMap(ctx, d.K8sclient, dcr.Namespace, cmName)
		if err != nil {
			d.K8srecorder.Event(dcr, string(EventWarning), string(ConfigMapGetFailed), "BuildCoreConfigmapStatusHash configmap "+"namespace "+dcr.Namespace+" name "+cmName+" find failed "+err.Error())
			return ""
		}

		confMap, err := resource.ResolveConfigMaps([]*corev1.ConfigMap{cm}, componentType)

		if err != nil {
			d.K8srecorder.Event(dcr, string(EventWarning), string(ConfigMapGetFailed), "BuildCoreConfigmapStatusHash configmap "+"namespace "+dcr.Namespace+" name "+cmName+" find failed "+err.Error())
			return ""
		}

		return set.Map2Hash(confMap)
	}

	return ""
}

// CompareConfigmapAndTriggerRestart
// 1. Compared by configmap Resolve file to map`s hash
// 2. Add restart trigger DCR
func (d *SubDefaultController) CompareConfigmapAndTriggerRestart(dcr *dorisv1.DorisCluster, oldStatus dorisv1.ComponentStatus, componentType dorisv1.ComponentType) {
	oldCmHash := oldStatus.CoreConfigMapHashValue
	if oldCmHash == "" {
		// oldCmHash is "" means the following situations:
		// * First deployment: no restart is required, just skip it.
		// * Not the first deployment, configmap was not configured: add configmap for doris, then statusfulset schedules automatic rolling restart, and this method does not need to be triggered
		// * Not the first deployment, configmap is also configured: the operator upgrade operation is done, and 'CoreConfigMapHashValue' was not available before. It also needs to be skipped, no restart is required, CoreConfigMapHashValue will be modified in the subsequent 'UpdateComponentStatus' method.
		return
	}

	newCmHash := d.BuildCoreConfigmapStatusHash(context.Background(), dcr, componentType)
	if newCmHash == "" {
		// dcr has no configmap for doris core config
		return
	}

	if oldCmHash == newCmHash {
		// not change configmap
		return
	}

	// configmap changed , restart sts
	if oldStatus.ComponentCondition.Phase == dorisv1.Available {
		klog.Infof("CompareConfigmapAndTriggerRestart TriggerRestart %s for CRD %s , namespace: %s", componentType, dcr.Namespace, dcr.Namespace)
		dcr.Annotations[dorisv1.GetRestartAnnotationKey(componentType)] = time.Now().Format(time.RFC3339)
		status := dcr.GetComponentStatus(componentType)
		status.ComponentCondition.Phase = dorisv1.Restarting
	}
}
