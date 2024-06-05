package fe

import (
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
)

func (fc *Controller) buildFEStatefulSet(dcr *v1.DorisCluster) appv1.StatefulSet {
	st := resource.NewStatefulSet(dcr, v1.Component_FE)
	st.Spec.Template = fc.buildFEPodTemplateSpec(dcr)
	return st
}
