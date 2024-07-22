package ms

import (
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
)

func (msc *Controller) buildMSStatefulSet(dms *mv1.DorisDisaggregatedMetaService, config map[string]interface{}) appv1.StatefulSet {
	st := resource.NewDMSStatefulSet(dms, mv1.Component_MS)
	st.Spec.Template = msc.buildMSPodTemplateSpec(dms)
	return st
}
