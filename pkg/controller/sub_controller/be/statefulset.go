package be

import (
	v1 "github.com/selectdb/doris-operator/api/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
)

func (be *Controller) buildBEStatefulSet(dcr *v1.DorisCluster) appv1.StatefulSet {
	st := resource.NewStatefulSet(dcr, v1.Component_BE)
	st.Spec.Template = be.buildBEPodTemplateSpec(dcr)
	return st
}
