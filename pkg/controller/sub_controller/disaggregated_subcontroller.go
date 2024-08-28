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
	"bytes"
	"context"
	"fmt"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/metadata"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DisaggregatedSubController interface {
	//Sync reconcile for sub controller. bool represent the component have updated.
	Sync(ctx context.Context, obj client.Object) error
	//clear all resource about sub-component.
	ClearResources(ctx context.Context, obj client.Object) (bool, error)

	//return the controller name, beController, feController,cnController for log.
	GetControllerName() string

	//UpdateStatus update the component status on src.
	UpdateComponentStatus(obj client.Object) error
}

type DisaggregatedSubDefaultController struct {
	K8sclient      client.Client
	K8srecorder    record.EventRecorder
	ControllerName string
}

// Deprecated:
func (d *DisaggregatedSubDefaultController) GetMSConfig(ctx context.Context, cms []mv1.ConfigMap, namespace string, componentType mv1.ComponentType) (map[string]interface{}, error) {
	if len(cms) == 0 {
		return make(map[string]interface{}), nil
	}
	configMaps, err := k8s.GetDisaggregatedConfigMaps(ctx, d.K8sclient, namespace, cms)
	if err != nil {
		klog.Errorf("DisaggregatedSubDefaultController GetConfig get configmap failed, namespace: %s,err: %s \n", namespace, err.Error())
	}
	res, resolveErr := resource.ResolveDMSConfigMaps(configMaps, componentType)
	return res, utils.MergeError(err, resolveErr)
}

func (d *DisaggregatedSubDefaultController) GetConfigValuesFromConfigMaps(namespace string, resolveKey string, cms []dv1.ConfigMap) map[string]interface{} {
	if len(cms) == 0 {
		return nil
	}

	for _, cm := range cms {
		kcm, err := k8s.GetConfigMap(context.Background(), d.K8sclient, namespace, cm.Name)
		if err != nil {
			klog.Errorf("disaggregatedFEController getConfigValuesFromConfigMaps namespace=%s, name=%s, failed, err=%s", namespace, cm.Name, err.Error())
			continue
		}

		if _, ok := kcm.Data[resolveKey]; !ok {
			continue
		}

		v := kcm.Data[resolveKey]
		viper.SetConfigType("properties")
		viper.ReadConfig(bytes.NewBuffer([]byte(v)))
		return viper.AllSettings()
	}

	return nil
}

// for config default values.
func (d *DisaggregatedSubDefaultController) NewDefaultService(ddc *dv1.DorisDisaggregatedCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ddc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: ddc.APIVersion,
					Kind:       ddc.Kind,
					Name:       ddc.Name,
					UID:        ddc.UID,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			SessionAffinity: corev1.ServiceAffinityClientIP,
		},
	}
}

func (d *DisaggregatedSubDefaultController) NewDefaultStatefulset(ddc *dv1.DorisDisaggregatedCluster) *appv1.StatefulSet {
	return &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       ddc.Namespace,
			OwnerReferences: []metav1.OwnerReference{resource.GetOwnerReference(ddc)},
		},
		Spec: appv1.StatefulSetSpec{
			PodManagementPolicy:  appv1.ParallelPodManagement,
			RevisionHistoryLimit: metadata.GetInt32Pointer(5),
			UpdateStrategy: appv1.StatefulSetUpdateStrategy{
				Type: appv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appv1.RollingUpdateStatefulSetStrategy{
					Partition: metadata.GetInt32Pointer(0),
				},
			},
		},
	}
}

func (d *DisaggregatedSubDefaultController) BuildDefaultConfigMapVolumesVolumeMounts(cms []dv1.ConfigMap) ([]corev1.Volume, []corev1.VolumeMount) {
	var vs []corev1.Volume
	var vms []corev1.VolumeMount
	for _, cm := range cms {
		v := corev1.Volume{
			Name: cm.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm.Name,
					},
				},
			},
		}

		vs = append(vs, v)
		vm := corev1.VolumeMount{
			Name:      cm.Name,
			MountPath: cm.MountPath,
		}
		if vm.MountPath == "" {
			vm.MountPath = resource.ConfigEnvPath
		}
		vms = append(vms, vm)
	}
	return vs, vms
}

func (d *DisaggregatedSubDefaultController) ConstructDefaultAffinity(matchKey, value string, ddcAffinity *corev1.Affinity) *corev1.Affinity {
	affinity := d.newDefaultAffinity(matchKey, value)

	if ddcAffinity == nil {
		return affinity
	}

	ddcPodAntiAffinity := ddcAffinity.PodAntiAffinity
	if ddcPodAntiAffinity != nil {
		affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = ddcPodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, ddcPodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
	}

	affinity.NodeAffinity = ddcAffinity.NodeAffinity
	affinity.PodAffinity = ddcAffinity.PodAffinity

	return affinity
}

func (d *DisaggregatedSubDefaultController) newDefaultAffinity(matchKey, value string) *corev1.Affinity {
	if matchKey == "" || value == "" {
		return nil
	}

	podAffinityTerm := corev1.WeightedPodAffinityTerm{
		Weight: 20,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: matchKey, Operator: metav1.LabelSelectorOpIn, Values: []string{value}},
				},
			},
			TopologyKey: resource.NODE_TOPOLOGYKEY,
		},
	}
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{podAffinityTerm},
		},
	}
}

// the common logic to apply service, will used by fe,be,ms.
func (d *DisaggregatedSubDefaultController) DefaultReconcileService(ctx context.Context, svc *corev1.Service) (*Event, error) {
	var esvc corev1.Service
	if err := d.K8sclient.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, &esvc); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, d.K8sclient, svc); err != nil {
			klog.Errorf("disaggregatedSubDefaultController reconcileService create service namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
			return &Event{Type: EventWarning, Reason: ServiceApplyedFailed, Message: err.Error()}, err
		}
	} else if err != nil {
		klog.Errorf("disaggregatedSubDefaultController reconcileService get service failed, namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return nil, err
	}

	if err := k8s.ApplyService(ctx, d.K8sclient, svc, func(nsvc, osvc *corev1.Service) bool {
		return resource.ServiceDeepEqualWithAnnoKey(nsvc, osvc, dv1.DisaggregatedSpecHashValueAnnotation)
	}); err != nil {
		klog.Errorf("disaggregatedSubDefaultController reconcileService apply service namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return &Event{Type: EventWarning, Reason: ServiceApplyedFailed, Message: err.Error()}, err
	}

	return nil, nil
}

// generate map for mountpath:configmap
// Deprecated:
func (d *DisaggregatedSubDefaultController) CheckMSConfigMountPath(dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) {
	bspec, _ := resource.GetDMSBaseSpecFromCluster(dms, componentType)
	cms := bspec.ConfigMaps

	var mountsMap = make(map[string]mv1.ConfigMap)
	for _, cm := range cms {
		path := cm.MountPath
		if m, exist := mountsMap[path]; exist {
			klog.Errorf("CheckMSConfigMountPath error: the mountPath %s is repeated between configmap: %s and configmap: %s.", path, cm.Name, m.Name)
			d.K8srecorder.Event(dms, string(EventWarning), string(ConfigMapPathRepeated), fmt.Sprintf("the mountPath %s is repeated between configmap: %s and configmap: %s.", path, cm.Name, m.Name))
		}
		mountsMap[path] = cm
	}
}

// RestrictConditionsEqual adds two StatefulSet,
// It is used to control the conditions for comparing.
// nst StatefulSet - a new StatefulSet
// est StatefulSet - an old StatefulSet
func (d *DisaggregatedSubDefaultController) RestrictConditionsEqual(nst *appv1.StatefulSet, est *appv1.StatefulSet) {
	//shield persistent volume update when the pvcProvider=Operator
	//in webhook should intercept the volume spec updated when use statefulset pvc.
	// TODO: updates to statefulset spec for fields other than 'replicas', 'template', 'updateStrategy', 'persistentVolumeClaimRetentionPolicy' and 'minReadySeconds' are forbidden
	nst.Spec.VolumeClaimTemplates = est.Spec.VolumeClaimTemplates
}

// Deprecated:
// ClearMSCommonResources clear common resources all component have, as statefulset, service.
// response `bool` represents all resource have deleted, if not and delete resource failed return false for next reconcile retry.
func (d *DisaggregatedSubDefaultController) ClearMSCommonResources(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) (bool, error) {
	//if the doris is not have cn.
	stName := mv1.GenerateComponentStatefulSetName(dms, componentType)
	//externalServiceName := mv1.GenerateExternalServiceName(dms, componentType)
	internalServiceName := mv1.GenerateCommunicateServiceName(dms, componentType)
	if err := k8s.DeleteStatefulset(ctx, d.K8sclient, dms.Namespace, stName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("DisaggregatedSubDefaultController ClearCommonResources delete statefulset failed, namespace=%s,name=%s, error=%s.", dms.Namespace, stName, err.Error())
		return false, err
	}

	if err := k8s.DeleteService(ctx, d.K8sclient, dms.Namespace, internalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("DisaggregatedSubDefaultController ClearCommonResources delete search service, namespace=%s,name=%s,error=%s.", dms.Namespace, internalServiceName, err.Error())
		return false, err
	}

	return true, nil
}

func (d *DisaggregatedSubDefaultController) ClassifyPodsByStatus(namespace string, status *mv1.BaseStatus, labels map[string]string, replicas int32) error {
	var podList corev1.PodList
	if err := d.K8sclient.List(context.Background(), &podList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return err
	}

	var creatings, readys, faileds []string
	podmap := make(map[string]corev1.Pod)
	//get all pod status that controlled by st.
	for _, pod := range podList.Items {
		podmap[pod.Name] = pod
		if ready := k8s.PodIsReady(&pod.Status); ready {
			readys = append(readys, pod.Name)
		} else if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			creatings = append(creatings, pod.Name)
		} else {
			faileds = append(faileds, pod.Name)
		}
	}

	if len(readys) == int(replicas) {
		status.Phase = mv1.Ready
	} else if len(faileds) != 0 {
		status.Phase = mv1.Failed
	} else if len(creatings) != 0 {
		status.Phase = mv1.Creating
	}

	status.AvailableStatus = mv1.UnAvailable
	if status.Phase == mv1.Ready {
		status.AvailableStatus = mv1.Available
	}
	return nil
}
