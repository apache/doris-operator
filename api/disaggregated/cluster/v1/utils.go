package v1

const (
	DorisDisaggregatedClusterName string = "app.doris.disaggregated.cluster"

	//OwnerReference list ownerReferences this object
	DorisDisaggregatedOwnerReference string = "app.doris.disaggregated.ownerreference/name"

	DorisDisaggregatedComputeGroupClusterId string = "app.doris.disaggregated.cg-clusterid"

	DorisDisaggregatedComputeGroupCloudUniqueId string = "app.doris.disaggregated.cg-clouduniqueid"

	DorisDisaggregatedPodType string = "app.doris.disaggregated.type"

	DisaggregatedSpecHashValueAnnotation string = "doris.disaggregated.cluster/hash"
)

type DisaggregatedComponentType string

var (
	DisaggregatedFE DisaggregatedComponentType = "FE"
	DisaggregatedBE DisaggregatedComponentType = "BE"
)
