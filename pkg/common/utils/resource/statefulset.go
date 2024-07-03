package resource

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
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

// use computeGroup build base statefulset.
func NewStatefulSetWithComputeGroup(cg *dv1.ComputeGroup) *appv1.StatefulSet {
	st := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: appv1.StatefulSetSpec{
			Replicas: cg.Replicas,
		},
	}
	return st
}

// StatefulSetDeepEqual judge two statefulset equal or not.
func StatefulSetDeepEqual(new *appv1.StatefulSet, old *appv1.StatefulSet, excludeReplicas bool) bool {
	return StatefulsetDeepEqualWithAnnoKey(new, old, v1.ComponentResourceHash, excludeReplicas)
}

func StatefulsetDeepEqualWithAnnoKey(new, old *appv1.StatefulSet, annoKey string, excludeReplicas bool) bool {
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
