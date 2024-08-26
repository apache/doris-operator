package resource

import (
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"testing"
)

func Test_BuildPVCAnnotations(t *testing.T) {
	test := dorisv1.PersistentVolume{
		Name:      "test",
		MountPath: "/etc/doris",
		Annotations: NewAnnotations(Annotations{
			"test": "test",
		}),
		PVCProvisioner: "Operator",
	}

	anno := buildPVCAnnotations(test)
	if _, ok := anno[pvc_manager_annotation]; !ok {
		t.Errorf("buildPVCAnnotations failed, not \"pvc_manager_annotation\" annotation.")
	}
}
