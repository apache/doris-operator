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
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	"github.com/selectdb/doris-operator/pkg/common/utils/metadata"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	defaultRollingUpdateStartPod int32 = 0
)

// NewStatefulSet construct statefulset.
func NewStatefulSet(dcr *v1.DorisCluster, componentType v1.ComponentType) appv1.StatefulSet {
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
	for _, vct := range bSpec.PersistentVolumes {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        vct.Name,
				Annotations: buildPVCAnnotations(vct),
			},
			Spec: vct.PersistentVolumeClaimSpec,
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
			Template:             NewPodTemplateSpec(dcr, componentType),
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

// StatefulSetDeepEqual judge two statefulset equal or not.
func StatefulSetDeepEqual(new *appv1.StatefulSet, old *appv1.StatefulSet, excludeReplicas bool) bool {
	equal := StatefulsetDeepEqualWithOmitKey(new, old, v1.ComponentResourceHash, false, excludeReplicas)
	if !equal {
		return clear_config_env_path_numbers_alwaysEquals(new, old, v1.ComponentResourceHash, excludeReplicas)

	}

	return true
}

// clear duplicate env for compatible 1.4.1, 1.5.1,1.6.1
// Note, Note,Note, please delete the logic code in 2024.12.31,
// used deepcopy for envs will assign to new and the operation will result new updated in unexpected.
func clear_config_env_path_numbers_alwaysEquals(new *appv1.StatefulSet, old *appv1.StatefulSet, annoKey string, excludeReplicas bool) bool {
	newAvoidEnvsSt := new.DeepCopy()
	var newEnvs []corev1.EnvVar
	//for find the main container position for assign envs to new equal.
	nMainContainerIndex := -1
	for i, c := range new.Spec.Template.Spec.Containers {
		if c.Name == string(v1.Component_FE) || c.Name == string(v1.Component_BE) || c.Name == string(v1.Component_CN) || c.Name == string(v1.Component_Broker) {
			newEnvs = c.Env
			nMainContainerIndex = i
			break
		}
	}

	var oldEnvs []corev1.EnvVar
	for _, c := range old.Spec.Template.Spec.Containers {
		if c.Name == string(v1.Component_FE) || c.Name == string(v1.Component_BE) || c.Name == string(v1.Component_CN) || c.Name == string(v1.Component_Broker) {
			oldEnvs = c.Env
			break
		}
	}

	for i := 0; i < len(newEnvs); i++ {
		if newEnvs[i].Name == config_env_name {
			newEnvs = append(newEnvs[:i], newEnvs[i+1:]...)
			i--
		}
	}

	poss := []int{}
	for i := 0; i < len(oldEnvs); i++ {
		if oldEnvs[i].Name == config_env_name {
			poss = append(poss, i)
		}
	}
	for i := 0; i < len(poss); i++ {
		if poss[i] >= len(newEnvs) {
			newEnvs = append(newEnvs, corev1.EnvVar{Name: config_env_name, Value: config_env_path})
		} else {
			index := poss[i]
			lastEnvs := append([]corev1.EnvVar{{Name: config_env_name, Value: config_env_path}}, newEnvs[index:]...)
			newEnvs = append(newEnvs[:index], lastEnvs...)
		}
	}
	
	if len(newEnvs) != len(oldEnvs) {
		return false
	} else {
		for i := 0; i < len(newEnvs); i++ {
			if newEnvs[i].Name != oldEnvs[i].Name {
				return false
			}
			if newEnvs[i].Value != "" && newEnvs[i].Value != oldEnvs[i].Value {
				return false
			}
		}
	}

	newAvoidEnvsSt.Spec.Template.Spec.Containers[nMainContainerIndex].Env = newEnvs
	if *new.Spec.Replicas == *old.Spec.Replicas {
		nAvoidOb := statefulSetHashObject(newAvoidEnvsSt, excludeReplicas)
		nAvoidHash := hash.HashObject(nAvoidOb)
		oldHash := old.Annotations[annoKey]
		return nAvoidHash == oldHash
	} else {
		*newAvoidEnvsSt.Spec.Replicas = *old.Spec.Replicas
		nAvoidOb := statefulSetHashObject(newAvoidEnvsSt, excludeReplicas)
		nAvoidHash := hash.HashObject(nAvoidOb)
		oldHash := old.Annotations[annoKey]
		if nAvoidHash == oldHash {
			new.Spec.Template.Spec.Containers[nMainContainerIndex].Env = newEnvs
		}
		return false
	}
}

func StatefulsetDeepEqualWithOmitKey(new, old *appv1.StatefulSet, annoKey string, omit bool, excludeReplicas bool) bool {
	if omit {
		newHso := statefulSetHashObject(new, excludeReplicas)
		newHashv := hash.HashObject(newHso)
		oldHso := statefulSetHashObject(old, excludeReplicas)
		oldHashv := hash.HashObject(oldHso)
		return new.Namespace == old.Namespace && newHashv == oldHashv
	}
	var newHashv, oldHashv string
	if annoKey == "" {
		annoKey = v1.ComponentResourceHash
	}

	if _, ok := new.Annotations[annoKey]; ok {
		newHashv = new.Annotations[annoKey]
	} else {
		newHso := statefulSetHashObject(new, excludeReplicas)
		newHashv = hash.HashObject(newHso)
	}

	if _, ok := old.Annotations[annoKey]; ok {
		oldHashv = old.Annotations[annoKey]
	} else {
		oldHso := statefulSetHashObject(old, excludeReplicas)
		oldHashv = hash.HashObject(oldHso)
	}

	anno := Annotations{}
	anno.AddAnnotation(new.Annotations)
	anno.Add(annoKey, newHashv)
	new.Annotations = anno

	klog.Info("the statefulset name "+new.Name+" new hash value ", newHashv, " old have value ", oldHashv)
	return newHashv == oldHashv &&
		new.Namespace == old.Namespace
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
