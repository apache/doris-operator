package resource

import (
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	pvc_finalizer          = "selectdb.doris.com/pvc-finalizer"
	pvc_manager_annotation = "selectdb.doris.com/pvc-manager"
)

func BuildPVC(volume dorisv1.PersistentVolume, labels map[string]string, namespace, stsName, ordinal string) corev1.PersistentVolumeClaim {
	pvcName := stsName + "-" + ordinal
	if volume.Name != "" {
		pvcName = volume.Name + "-" + pvcName
	}

	finalAnnotations := buildPVCAnnotations(volume)

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: finalAnnotations,
			Finalizers:  []string{pvc_finalizer},
		},
		Spec: volume.PersistentVolumeClaimSpec,
	}
	return pvc
}

// finalAnnotations is a combination of user annotations and operator default annotations
func buildPVCAnnotations(volume dorisv1.PersistentVolume) Annotations {
	finalAnnotations := Annotations{
		pvc_manager_annotation:        "operator",
		dorisv1.ComponentResourceHash: hash.HashObject(volume.PersistentVolumeClaimSpec),
	}
	if volume.Annotations != nil && len(volume.Annotations) > 0 {
		finalAnnotations.AddAnnotation(volume.Annotations)
	}
	return finalAnnotations
}
