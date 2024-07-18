package sub_controller

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
)

func GetDisaggregatedCommand(componentType dv1.DisaggregatedComponentType) (commands []string, args []string) {
	switch componentType {
	case dv1.DisaggregatedFE:
		return []string{"/opt/apache-doris/fe_disaggregated_entrypoint.sh"}, []string{}
	case dv1.DisaggregatedBE:
		return []string{"/opt/apache-doris/be_disaggregated_entrypoint.sh"}, []string{}
	default:
		return nil, nil
	}
}

// get the script path of prestop, this will be called before stop container.
func GetDisaggregatedPreStopScript(componentType dv1.DisaggregatedComponentType) string {
	switch componentType {
	case dv1.DisaggregatedFE:
		return "/opt/apache-doris/fe_disaggregated_prestop.sh"
	case dv1.DisaggregatedBE:
		return "/opt/apache-doris/be_disaggregated_prestop.sh"
	default:
		return ""
	}
}
