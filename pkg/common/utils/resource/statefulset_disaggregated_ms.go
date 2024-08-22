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
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	"github.com/selectdb/doris-operator/pkg/common/utils/metadata"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	defaultRollingUpdateStartDMSPod int32 = 0
	defaultLogPrefixName                  = "log"

	defaultDMSImagePullPolicy corev1.PullPolicy = corev1.PullIfNotPresent
)

// NewDMSStatefulSet construct statefulset for metaservice and recycler.
func NewDMSStatefulSet(dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) appv1.StatefulSet {
	bSpec, replicas := GetDMSBaseSpecFromCluster(dms, componentType)

	orf := metav1.OwnerReference{
		APIVersion: dms.APIVersion,
		Kind:       dms.Kind,
		Name:       dms.Name,
		UID:        dms.UID,
	}

	selector := metav1.LabelSelector{
		MatchLabels: mv1.GenerateStatefulSetSelector(dms, componentType),
	}

	var volumeClaimTemplates []corev1.PersistentVolumeClaim
	cpv := bSpec.PersistentVolume
	if cpv != nil {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        defaultLogPrefixName,
				Annotations: NewAnnotations(),
			},
			Spec: cpv.PersistentVolumeClaimSpec,
		}
		volumeClaimTemplates = append(volumeClaimTemplates, pvc)
	}

	st := appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       dms.Namespace,
			Name:            mv1.GenerateComponentStatefulSetName(dms, componentType),
			Labels:          mv1.GenerateStatefulSetLabels(dms, componentType),
			OwnerReferences: []metav1.OwnerReference{orf},
		},

		Spec: appv1.StatefulSetSpec{
			Replicas:             replicas,
			Selector:             &selector,
			Template:             NewDMSPodTemplateSpec(dms, componentType),
			VolumeClaimTemplates: volumeClaimTemplates,
			ServiceName:          mv1.GenerateCommunicateServiceName(dms, componentType),
			RevisionHistoryLimit: metadata.GetInt32Pointer(5),
			UpdateStrategy: appv1.StatefulSetUpdateStrategy{
				Type: appv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appv1.RollingUpdateStatefulSetStrategy{
					Partition: metadata.GetInt32Pointer(defaultRollingUpdateStartDMSPod),
				},
			},
			// ParallelPodManagement will create and delete pods as soon as the stateful set replica count is changed, and will not wait for pods to be ready or complete
			PodManagementPolicy: appv1.ParallelPodManagement,
		},
	}

	return st
}

// StatefulSetDeepEqual judge two statefulset equal or not.
func DMSStatefulSetDeepEqual(new *appv1.StatefulSet, old *appv1.StatefulSet, excludeReplicas bool) bool {
	var newHashv, oldHashv string

	if _, ok := new.Annotations[mv1.ComponentResourceHash]; ok {
		newHashv = new.Annotations[mv1.ComponentResourceHash]
	} else {
		newHso := statefulSetHashObject(new, excludeReplicas)
		newHashv = hash.HashObject(newHso)
	}

	if _, ok := old.Annotations[mv1.ComponentResourceHash]; ok {
		oldHashv = old.Annotations[mv1.ComponentResourceHash]
	} else {
		oldHso := statefulSetHashObject(old, excludeReplicas)
		oldHashv = hash.HashObject(oldHso)
	}

	anno := Annotations{}
	anno.AddAnnotation(new.Annotations)
	anno.Add(mv1.ComponentResourceHash, newHashv)
	new.Annotations = anno

	klog.Info("the statefulset name "+new.Name+" new hash value ", newHashv, " old have value ", oldHashv)
	return newHashv == oldHashv &&
		new.Namespace == old.Namespace
}
