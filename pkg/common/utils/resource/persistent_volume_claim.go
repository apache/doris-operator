package resource

import (
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	pvc_finalizer          = "selectdb.doris.com/pvc-finalizer"
	pvc_manager_annotation = "selectdb.doris.com/pvc-manager"
)

func BuildPVCName(stsName, ordinal, volumeName string) string {
	pvcName := stsName + "-" + ordinal
	if volumeName != "" {
		pvcName = volumeName + "-" + pvcName
	}
	return pvcName
}

func BuildPVC(volume dorisv1.PersistentVolume, labels map[string]string, namespace, stsName, ordinal string) corev1.PersistentVolumeClaim {
	annotations := buildPVCAnnotations(volume)

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        BuildPVCName(stsName, ordinal, volume.Name),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
			Finalizers:  []string{pvc_finalizer},
		},
		Spec: volume.PersistentVolumeClaimSpec,
	}
	return pvc
}

// finalAnnotations is a combination of user annotations and operator default annotations
func buildPVCAnnotations(volume dorisv1.PersistentVolume) Annotations {
	annotations := Annotations{}
	if volume.PVCProvisioner == dorisv1.PVCProvisionerOperator {
		annotations.Add(pvc_manager_annotation, "operator")
		annotations.Add(dorisv1.ComponentResourceHash, hash.HashObject(volume.PersistentVolumeClaimSpec))
	}

	if volume.Annotations != nil && len(volume.Annotations) > 0 {
		annotations.AddAnnotation(volume.Annotations)
	}
	return annotations
}

func BuildDMSPVC(volume mv1.PersistentVolume, labels map[string]string, namespace, stsName, ordinal string) corev1.PersistentVolumeClaim {
	annotations := buildDMSAnnotationsForPVC(volume)

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        BuildPVCName(stsName, ordinal, volume.Name),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
			Finalizers:  []string{pvc_finalizer},
		},
		Spec: volume.PersistentVolumeClaimSpec,
	}
	return pvc
}

// finalAnnotations is a combination of user annotations and operator default annotations
func buildDMSAnnotationsForPVC(volume mv1.PersistentVolume) Annotations {
	annotations := Annotations{}
	if volume.PVCProvisioner == mv1.PVCProvisionerOperator {
		annotations.Add(pvc_manager_annotation, "operator")
		annotations.Add(mv1.ComponentResourceHash, hash.HashObject(volume.PersistentVolumeClaimSpec))
	}

	if volume.Annotations != nil && len(volume.Annotations) > 0 {
		annotations.AddAnnotation(volume.Annotations)
	}
	return annotations
}
