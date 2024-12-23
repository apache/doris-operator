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
	"github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/metadata"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

func (d *DisaggregatedSubDefaultController) GetConfigValuesFromConfigMaps(namespace string, resolveKey string, cms []v1.ConfigMap) map[string]interface{} {
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
func (d *DisaggregatedSubDefaultController) NewDefaultService(ddc *v1.DorisDisaggregatedCluster) *corev1.Service {
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

func (d *DisaggregatedSubDefaultController) NewDefaultStatefulset(ddc *v1.DorisDisaggregatedCluster) *appv1.StatefulSet {
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

func (d *DisaggregatedSubDefaultController) BuildDefaultConfigMapVolumesVolumeMounts(cms []v1.ConfigMap) ([]corev1.Volume, []corev1.VolumeMount) {
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
	if err := k8s.ApplyService(ctx, d.K8sclient, svc, func(nsvc, osvc *corev1.Service) bool {
		return resource.ServiceDeepEqualWithAnnoKey(nsvc, osvc, v1.DisaggregatedSpecHashValueAnnotation)
	}); err != nil {
		klog.Errorf("disaggregatedSubDefaultController reconcileService apply service namespace=%s name=%s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return &Event{Type: EventWarning, Reason: ServiceApplyedFailed, Message: err.Error()}, err
	}

	return nil, nil
}

// generate map for mountpath:secret
func (d *DisaggregatedSubDefaultController) CheckSecretMountPath(ddc *v1.DorisDisaggregatedCluster, secrets []v1.Secret) {
	var mountsMap = make(map[string]v1.Secret)
	for _, secret := range secrets {
		path := secret.MountPath
		if s, exist := mountsMap[path]; exist {
			klog.Errorf("CheckSecretMountPath error: the mountPath %s is repeated between secret: %s and secret: %s.", path, secret.SecretName, s.SecretName)
			d.K8srecorder.Event(ddc, string(EventWarning), string(SecretPathRepeated), fmt.Sprintf("the mountPath %s is repeated between secret: %s and secret: %s.", path, secret.SecretName, s.SecretName))
		}
		mountsMap[path] = secret
	}
}

// CheckSecretExist, check the secret exist or not in specify namespace.
func (d *DisaggregatedSubDefaultController) CheckSecretExist(ctx context.Context, ddc *v1.DorisDisaggregatedCluster, secrets []v1.Secret) {
	errMessage := ""
	for _, secret := range secrets {
		var s corev1.Secret
		if getErr := d.K8sclient.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: secret.SecretName}, &s); getErr != nil {
			errMessage = errMessage + fmt.Sprintf("(name: %s, namespace: %s, err: %s), ", secret.SecretName, ddc.Namespace, getErr.Error())
		}
	}
	if errMessage != "" {
		klog.Errorf("CheckSecretExist error: %s.", errMessage)
		d.K8srecorder.Event(ddc, string(EventWarning), string(SecretNotExist), fmt.Sprintf("CheckSecretExist error: %s.", errMessage))
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
