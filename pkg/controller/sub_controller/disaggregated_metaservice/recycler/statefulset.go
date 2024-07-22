package recycler

import (
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
)

func (rc *RecyclerController) buildRCStatefulSet(dms *mv1.DorisDisaggregatedMetaService) appv1.StatefulSet {
	st := resource.NewDMSStatefulSet(dms, mv1.Component_RC)
	st.Spec.Template = rc.buildMSPodTemplateSpec(dms)
	return st
}
