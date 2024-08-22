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

package k8s

import (
	"context"
	"errors"
	"fmt"
	"github.com/FoundationDB/fdb-kubernetes-operator/api/v1beta2"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/autoscaling/v1"
	v2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// judge two services equal or not in some fields. develoer can custom the function.
type ServiceEqual func(svc1 *corev1.Service, svc2 *corev1.Service) bool

// judge two statefulset equal or not in some fields. develoer can custom the function.
type StatefulSetEqual func(st1 *appv1.StatefulSet, st2 *appv1.StatefulSet) bool

func ApplyService(ctx context.Context, k8sclient client.Client, svc *corev1.Service, equal ServiceEqual) error {
	// As stated in the RetryOnConflict's documentation, the returned error shouldn't be wrapped.
	var esvc corev1.Service
	err := k8sclient.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &esvc)
	if err != nil && apierrors.IsNotFound(err) {
		return CreateClientObject(ctx, k8sclient, svc)
	} else if err != nil {
		return err
	}

	if equal(svc, &esvc) {
		klog.Info("CreateOrUpdateService service Name, Ports, Selector, ServiceType, Labels have not change ", "namespace ", svc.Namespace, " name ", svc.Name)
		return nil
	}

	//resolve the bug: metadata.resourceversion invalid value '' must be specified for an update
	svc.ResourceVersion = esvc.ResourceVersion
	return PatchClientObject(ctx, k8sclient, svc)
}

// ApplyStatefulSet when the object is not exist, create object. if exist and statefulset have been updated, patch the statefulset.
func ApplyStatefulSet(ctx context.Context, k8sclient client.Client, st *appv1.StatefulSet, equal StatefulSetEqual) error {
	var est appv1.StatefulSet
	err := k8sclient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est)
	if err != nil && apierrors.IsNotFound(err) {
		return CreateClientObject(ctx, k8sclient, st)
	} else if err != nil {
		return err
	}

	//if have restart annotation we should exclude it impacts on hash.
	if equal(st, &est) {
		klog.Infof("ApplyStatefulSet Sync exist statefulset name=%s, namespace=%s, equals to new statefulset.", est.Name, est.Namespace)
		return nil
	}

	st.ResourceVersion = est.ResourceVersion
	return PatchClientObject(ctx, k8sclient, st)
}

func CreateClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.Info("Creating resource service ", "namespace ", object.GetNamespace(), " name ", object.GetName(), " kind ", object.GetObjectKind().GroupVersionKind().Kind)
	if err := k8sclient.Create(ctx, object); err != nil {
		return err
	}
	return nil
}

func UpdateClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.Info("Updating resource service ", "namespace ", object.GetNamespace(), " name ", object.GetName(), " kind ", object.GetObjectKind())
	if err := k8sclient.Update(ctx, object); err != nil {
		return err
	}
	return nil
}

func CreateOrUpdateClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.V(4).Infof("create or update resource namespace=%s,name=%s,kind=%s.", object.GetNamespace(), object.GetName(), object.GetObjectKind())
	if err := k8sclient.Update(ctx, object); apierrors.IsNotFound(err) {
		return k8sclient.Create(ctx, object)
	} else if err != nil {
		return err
	}

	return nil
}

// PatchClientObject patch object when the object exist. if not return error.
func PatchClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.V(4).Infof("patch resource namespace=%s,name=%s,kind=%s.", object.GetNamespace(), object.GetName(), object.GetObjectKind())
	if err := k8sclient.Patch(ctx, object, client.Merge); err != nil {
		return err
	}

	return nil
}

// PatchOrCreate patch object if not exist create object.
func PatchOrCreate(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.V(4).Infof("patch or create resource namespace=%s,name=%s,kind=%s.", object.GetNamespace(), object.GetName(), object.GetObjectKind())
	if err := k8sclient.Patch(ctx, object, client.Merge); apierrors.IsNotFound(err) {
		return k8sclient.Create(ctx, object)
	} else if err != nil {
		return err
	}

	return nil
}

func DeleteClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	if err := k8sclient.Delete(ctx, object); err != nil {
		return err
	}
	return nil
}

// DeleteStatefulset delete statefulset.
func DeleteStatefulset(ctx context.Context, k8sclient client.Client, namespace, name string) error {
	var st appv1.StatefulSet
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &st); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return k8sclient.Delete(ctx, &st)
}

// DeleteService delete service.
func DeleteService(ctx context.Context, k8sclient client.Client, namespace, name string) error {
	var svc corev1.Service
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &svc); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return k8sclient.Delete(ctx, &svc)
}

// DeleteAutoscaler as version type delete response autoscaler.
func DeleteAutoscaler(ctx context.Context, k8sclient client.Client, namespace, name string, autoscalerVersion dorisv1.AutoScalerVersion) error {
	var autoscaler client.Object
	switch autoscalerVersion {
	case dorisv1.AutoScalerV1:
		autoscaler = &v1.HorizontalPodAutoscaler{}
	case dorisv1.AutoSclaerV2:
		autoscaler = &v2.HorizontalPodAutoscaler{}

	default:
		return errors.New(fmt.Sprintf("the autoscaler type %s is not supported.", autoscalerVersion))
	}

	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, autoscaler); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return k8sclient.Delete(ctx, autoscaler)
}

func PodIsReady(status *corev1.PodStatus) bool {
	if status.ContainerStatuses == nil {
		return false
	}

	for _, cs := range status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
	}

	return true
}

// get the secret by namespace and name.
func GetSecret(ctx context.Context, k8sclient client.Client, namespace, name string) (*corev1.Secret, error) {
	var secret corev1.Secret
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func CreateSecret(ctx context.Context, k8sclient client.Client, secret *corev1.Secret) error {
	return k8sclient.Create(ctx, secret)
}

func UpdateSecret(ctx context.Context, k8sclient client.Client, secret *corev1.Secret) error {
	if err := k8sclient.Update(ctx, secret); err != nil {
		return err
	}
	return nil
}

// GetConfigMap get the configmap name=name, namespace=namespace.
func GetConfigMap(ctx context.Context, k8scient client.Client, namespace, name string) (*corev1.ConfigMap, error) {
	var configMap corev1.ConfigMap
	if err := k8scient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &configMap); err != nil {
		return nil, err
	}

	return &configMap, nil
}

// GetConfigMaps get the configmap by the array of MountConfigMapInfo and namespace.
func GetConfigMaps(ctx context.Context, k8scient client.Client, namespace string, cms []dorisv1.MountConfigMapInfo) ([]*corev1.ConfigMap, error) {
	var configMaps []*corev1.ConfigMap
	errMessage := ""
	for _, cm := range cms {
		var configMap corev1.ConfigMap
		if getErr := k8scient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cm.ConfigMapName}, &configMap); getErr != nil {
			errMessage = errMessage + fmt.Sprintf("(name: %s, namespace: %s, err: %s), ", cm.ConfigMapName, namespace, getErr.Error())
		}
		configMaps = append(configMaps, &configMap)
	}
	if errMessage != "" {
		return configMaps, errors.New("Failed to get configmap: " + errMessage)
	}
	return configMaps, nil
}

// get the Service by namespace and name.
func GetService(ctx context.Context, k8sclient client.Client, namespace, name string) (*corev1.Service, error) {
	var svc corev1.Service
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &svc); err != nil {
		return nil, err
	}
	return &svc, nil
}

func GetPods(ctx context.Context, k8sclient client.Client, targetDCR dorisv1.DorisCluster, componentType dorisv1.ComponentType) (corev1.PodList, error) {
	pods := corev1.PodList{}

	err := k8sclient.List(
		ctx,
		&pods,
		client.InNamespace(targetDCR.Namespace),
		client.MatchingLabels(dorisv1.GetPodLabels(&targetDCR, componentType)),
	)
	if err != nil {
		return pods, err
	}

	return pods, nil
}

// GetConfig get conf from configmap by componentType , if not use configmap get an empty map.
func GetConfig(ctx context.Context, k8sclient client.Client, configMapInfo *dorisv1.ConfigMapInfo, namespace string, componentType dorisv1.ComponentType) (map[string]interface{}, error) {
	cms := resource.GetMountConfigMapInfo(*configMapInfo)
	if len(cms) == 0 {
		return make(map[string]interface{}), nil
	}

	configMaps, err := GetConfigMaps(ctx, k8sclient, namespace, cms)
	if err != nil {
		klog.Errorf("GetConfig get configmap failed, namespace: %s,err: %s \n", namespace, err.Error())
	}
	res, resolveErr := resource.ResolveConfigMaps(configMaps, componentType)
	return res, utils.MergeError(err, resolveErr)
}

// SetDorisClusterPhase set DorisCluster Phase status,
// Perform a check before setting, and do not change if the status is the same as the last time
func SetDorisClusterPhase(
	ctx context.Context,
	k8sclient client.Client,
	dcrName, namespace string,
	phase dorisv1.ComponentPhase,
	componentType dorisv1.ComponentType,
) error {
	var edcr dorisv1.DorisCluster
	if err := k8sclient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: dcrName}, &edcr); err != nil {
		return err
	}
	isStatusEqual := false
	switch componentType {
	case dorisv1.Component_FE:
		isStatusEqual = (edcr.Status.FEStatus.ComponentCondition.Phase == phase)
		edcr.Status.FEStatus.ComponentCondition.Phase = phase
	case dorisv1.Component_BE:
		isStatusEqual = (edcr.Status.BEStatus.ComponentCondition.Phase == phase)
		edcr.Status.BEStatus.ComponentCondition.Phase = phase
	case dorisv1.Component_CN:
		isStatusEqual = (edcr.Status.CnStatus.ComponentCondition.Phase == phase)
		edcr.Status.CnStatus.ComponentCondition.Phase = phase
	case dorisv1.Component_Broker:
		isStatusEqual = (edcr.Status.BrokerStatus.ComponentCondition.Phase == phase)
		edcr.Status.BrokerStatus.ComponentCondition.Phase = phase
	default:
		klog.Infof("SetDorisClusterPhase not support type=", componentType)
		return nil
	}
	if isStatusEqual {
		klog.Infof("UpdateDorisClusterPhase will not change cluster %s Phase, it is already %s ,DCR name: %s, namespace: %s,", componentType, phase, dcrName, namespace)
		return nil
	}
	return k8sclient.Status().Update(ctx, &edcr)
}

func SetClusterPhase(
	ctx context.Context,
	k8sclient client.Client,
	ddcName, namespace string,
	phase dv1.Phase,
	componentType dv1.DisaggregatedComponentType,
	ccStsNames []string,
) error {
	var edcr dv1.DorisDisaggregatedCluster
	if err := k8sclient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ddcName}, &edcr); err != nil {
		return err
	}
	isStatusEqual := true
	switch componentType {
	case dv1.DisaggregatedFE:
		isStatusEqual = edcr.Status.FEStatus.Phase == phase
		edcr.Status.FEStatus.Phase = phase
	case dv1.DisaggregatedBE:
		for i, ccs := range edcr.Status.ComputeClusterStatuses {
			name := ccs.StatefulsetName
			for _, ccStsName := range ccStsNames {
				if ccStsName == name {
					if ccs.Phase != phase {
						isStatusEqual = false
					}
					edcr.Status.ComputeClusterStatuses[i].Phase = phase
				}
			}
		}
	default:
		klog.Infof("SetClusterPhase not support type=%s", componentType)
		return nil
	}
	if isStatusEqual {
		klog.Infof("UpdateDDCPhase will not change cluster %s Phase, it is already %s ,DDC name: %s, namespace: %s,", componentType, phase, ddcName, namespace)
		return nil
	}
	return k8sclient.Status().Update(ctx, &edcr)
}

func GetDisaggregatedConfigMaps(ctx context.Context, k8scient client.Client, namespace string, cms []mv1.ConfigMap) ([]*corev1.ConfigMap, error) {
	var configMaps []*corev1.ConfigMap
	errMessage := ""
	for _, cm := range cms {
		var configMap corev1.ConfigMap
		if getErr := k8scient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cm.Name}, &configMap); getErr != nil {
			errMessage = errMessage + fmt.Sprintf("(name: %s, namespace: %s, err: %s), ", cm.Name, namespace, getErr.Error())
		}
		configMaps = append(configMaps, &configMap)
	}
	if errMessage != "" {
		return configMaps, errors.New("Failed to get configmap: " + errMessage)
	}
	return configMaps, nil
}

// ApplyFoundationDBCluster apply FoundationDBCluster to apiserver.
func ApplyFoundationDBCluster(ctx context.Context, k8sclient client.Client, fdb *v1beta2.FoundationDBCluster) error {
	var efdb v1beta2.FoundationDBCluster
	if err := k8sclient.Get(ctx, types.NamespacedName{
		Name:      fdb.Name,
		Namespace: fdb.Namespace,
	}, &efdb); apierrors.IsNotFound(err) {
		return k8sclient.Create(ctx, fdb)
	}

	fdb.ResourceVersion = efdb.ResourceVersion
	return k8sclient.Patch(ctx, fdb, client.Merge)
}

func DeleteFoundationDBCluster(ctx context.Context, k8sclient client.Client, namespace, name string) error {
	fdb, err := GetFoundationDBCluster(ctx, k8sclient, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return k8sclient.Delete(ctx, fdb)
}

func GetFoundationDBCluster(ctx context.Context, k8sclient client.Client, namespace, name string) (*v1beta2.FoundationDBCluster, error) {
	var fdb v1beta2.FoundationDBCluster
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &fdb); err != nil {
		return nil, err
	}
	return &fdb, nil
}

// DeletePVC clean up existing pvc by pvc name, namespace and labels
func DeletePVC(ctx context.Context, k8sclient client.Client, namespace, pvcName string, labels map[string]string) error {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels:    labels,
		},
	}
	err := k8sclient.Delete(ctx, &pvc)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
