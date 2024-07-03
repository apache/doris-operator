package disaggregated_fe

import dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"

func (dfc *DisaggregatedFEController) newFEPodsSelector(ddc *dv1.DorisDisaggregatedCluster) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName:    ddc.Name,
		dv1.DorisDisaggregatedPodType:        "fe",
		dv1.DorisDisaggregatedOwnerReference: ddc.Name,
	}
}
