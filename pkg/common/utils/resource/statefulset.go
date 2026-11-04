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

package resource

import (
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/hash"
	"github.com/apache/doris-operator/pkg/common/utils/metadata"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	defaultRollingUpdateStartPod int32 = 0
)

// NewStatefulSet construct statefulset.
func NewStatefulSet(dcr *v1.DorisCluster, config map[string]interface{}, componentType v1.ComponentType) appv1.StatefulSet {
	bSpec := getBaseSpecFromCluster(dcr, componentType)
	orf := metav1.OwnerReference{
		APIVersion: dcr.APIVersion,
		Kind:       dcr.Kind,
		Name:       dcr.Name,
		UID:        dcr.UID,
	}

	selector := metav1.LabelSelector{
		MatchLabels: v1.GenerateStatefulSetSelector(dcr, componentType),
	}

	var volumeClaimTemplates []corev1.PersistentVolumeClaim

	_, _, sharedPaths := BuildSharedVolumesAndVolumeMounts(dcr.Spec.SharedPersistentVolumeClaims, componentType)
	dorisPersistentVolumes, _ := GenerateEveryoneMountPathDorisPersistentVolume(bSpec, sharedPaths, config, componentType)
	for _, dorisPersistentVolume := range dorisPersistentVolumes {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        dorisPersistentVolume.Name,
				Annotations: buildPVCAnnotations(dorisPersistentVolume),
			},
			Spec: dorisPersistentVolume.PersistentVolumeClaimSpec,
		}

		volumeClaimTemplates = append(volumeClaimTemplates, pvc)
	}

	st := appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       dcr.Namespace,
			Name:            v1.GenerateComponentStatefulSetName(dcr, componentType),
			Labels:          v1.GenerateStatefulSetLabels(dcr, componentType),
			OwnerReferences: []metav1.OwnerReference{orf},
		},

		Spec: appv1.StatefulSetSpec{
			Replicas:             bSpec.Replicas,
			Selector:             &selector,
			Template:             NewPodTemplateSpec(dcr, config, componentType),
			VolumeClaimTemplates: volumeClaimTemplates,
			ServiceName:          v1.GenerateInternalCommunicateServiceName(dcr, componentType),
			RevisionHistoryLimit: metadata.GetInt32Pointer(5),
			UpdateStrategy: appv1.StatefulSetUpdateStrategy{
				Type: appv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appv1.RollingUpdateStatefulSetStrategy{
					Partition: metadata.GetInt32Pointer(defaultRollingUpdateStartPod),
				},
			},

			PodManagementPolicy: appv1.ParallelPodManagement,
		},
	}

	return st
}

// StatefulSetDeepEqual compares live managed fields and preserves harmless
// duplicate CONFIGMAP_MOUNT_PATH entries left by older native operators.
func StatefulSetDeepEqual(new, old *appv1.StatefulSet, excludeReplicas bool) bool {
	if StatefulsetDeepEqualWithKey(new, old, v1.ComponentResourceHash, excludeReplicas) {
		return true
	}
	return clear_config_env_path_numbers_alwaysEquals(new, old, v1.ComponentResourceHash, excludeReplicas)
}

func clear_config_env_path_numbers_alwaysEquals(new, old *appv1.StatefulSet, annoKey string, excludeReplicas bool) bool {
	candidate := new.DeepCopy()
	for i := range candidate.Spec.Template.Spec.Containers {
		container := &candidate.Spec.Template.Spec.Containers[i]
		if container.Name != string(v1.Component_FE) && container.Name != string(v1.Component_BE) && container.Name != string(v1.Component_CN) && container.Name != string(v1.Component_Broker) {
			continue
		}
		var previous *corev1.Container
		for j := range old.Spec.Template.Spec.Containers {
			if old.Spec.Template.Spec.Containers[j].Name == container.Name {
				previous = &old.Spec.Template.Spec.Containers[j]
				break
			}
		}
		if previous == nil {
			return false
		}
		var expected []corev1.EnvVar
		for _, env := range container.Env {
			if env.Name == config_env_name {
				if env.Value != config_env_path || env.ValueFrom != nil {
					return false
				}
			} else {
				expected = append(expected, env)
			}
		}
		previousCount, desiredCount := 0, 0
		for _, env := range previous.Env {
			if env.Name == config_env_name {
				previousCount++
			}
		}
		for _, env := range container.Env {
			if env.Name == config_env_name {
				desiredCount++
			}
		}
		// This exception is only for surplus duplicates from older versions.
		// Never suppress adding a newly requested configuration environment.
		if previousCount < 2 || previousCount <= desiredCount {
			return false
		}
		var compatible []corev1.EnvVar
		next := 0
		for _, env := range previous.Env {
			if env.Name == config_env_name {
				if env.Value != config_env_path || env.ValueFrom != nil {
					return false
				}
				compatible = append(compatible, env)
			} else {
				if next >= len(expected) {
					return false
				}
				compatible = append(compatible, expected[next])
				next++
			}
		}
		if next != len(expected) {
			return false
		}
		container.Env = compatible
		// Only preserve the legacy layout when every other managed template field
		// matches. Replica changes must not trigger a rolling update of these pods.
		if !StatefulsetDeepEqualWithKey(candidate, old, annoKey, true) {
			return false
		}
		new.Spec.Template.Spec.Containers[i].Env = compatible
		return StatefulsetDeepEqualWithKey(new, old, annoKey, excludeReplicas)
	}
	return false
}

func StatefulsetDeepEqualWithKey(new, old *appv1.StatefulSet, annoKey string, excludeReplicas bool) bool {
	if annoKey == "" {
		annoKey = v1.ComponentResourceHash
	}
	annotations := NewAnnotations(Annotations(new.Annotations))
	annotations[annoKey] = hash.HashObject(statefulSetHashObject(new, excludeReplicas))
	new.Annotations = annotations
	if new.Name != old.Name || new.Namespace != old.Namespace {
		return false
	}
	if !excludeReplicas && ptr.Deref(new.Spec.Replicas, int32(1)) != ptr.Deref(old.Spec.Replicas, int32(1)) {
		return false
	}
	if new.Spec.ServiceName != old.Spec.ServiceName || !equality.Semantic.DeepEqual(new.Spec.Selector, old.Spec.Selector) {
		return false
	}
	for key, value := range new.Labels {
		if actual, ok := old.Labels[key]; !ok || actual != value {
			return false
		}
	}
	if !statefulSetStrategyEqual(new.Spec.UpdateStrategy, old.Spec.UpdateStrategy) {
		return false
	}
	return equality.Semantic.DeepEqual(normalizedPodTemplate(new.Spec.Template, new.Spec.Template), normalizedPodTemplate(old.Spec.Template, new.Spec.Template)) &&
		equality.Semantic.DeepEqual(normalizedClaimTemplates(new.Spec.VolumeClaimTemplates), normalizedClaimTemplates(old.Spec.VolumeClaimTemplates))
}

// hashStatefulsetObject contains the info for hash comparison.
type hashStatefulsetObject struct {
	name                 string
	namespace            string
	labels               map[string]string
	selector             metav1.LabelSelector
	podTemplate          corev1.PodTemplateSpec
	serviceName          string
	volumeClaimTemplates []corev1.PersistentVolumeClaim
	replicas             int32
}

// StatefulsetHashObject construct the hash spec for deep equals to exist statefulset.
func statefulSetHashObject(st *appv1.StatefulSet, excludeReplica bool) hashStatefulsetObject {
	//set -1 for the initial is zero.
	replicas := int32(-1)
	if !excludeReplica {
		if st.Spec.Replicas != nil {
			replicas = *st.Spec.Replicas
		}
	}
	selector := metav1.LabelSelector{}
	if st.Spec.Selector != nil {
		selector = *st.Spec.Selector
	}

	return hashStatefulsetObject{
		name:                 st.Name,
		namespace:            st.Namespace,
		labels:               st.Labels,
		selector:             selector,
		podTemplate:          st.Spec.Template,
		serviceName:          st.Spec.ServiceName,
		volumeClaimTemplates: st.Spec.VolumeClaimTemplates,
		replicas:             replicas,
	}
}

// MergeStatefulSets merge exist statefulset and new statefulset.
func MergeStatefulSets(new *appv1.StatefulSet, old appv1.StatefulSet) {
	MergeMetadata(&new.ObjectMeta, old.ObjectMeta)
}
