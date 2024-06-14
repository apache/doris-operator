package v1

var (
	FDBNameSuffix = "-foundationdb"
	NameLabelKey  = "disaggregated.metaservice.doris.com/name"
)

// build foundationdbCluster's label for classify pods.
func (ddm *DorisDisaggregatedMetaService) GenerateFDBLabels() map[string]string {
	if ddm.Labels == nil {
		return map[string]string{
			NameLabelKey: ddm.Name,
		}
	}

	labels := make(map[string]string)
	labels[NameLabelKey] = ddm.Name
	for k, v := range ddm.Labels {
		labels[k] = v
	}

	return labels
}

func (ddm *DorisDisaggregatedMetaService) GenerateFDBClusterName() string {
	return ddm.Name + FDBNameSuffix
}
