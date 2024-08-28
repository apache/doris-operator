// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

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
	case dv1.DisaggregatedMS:
		return []string{"/opt/apache-doris/ms_disaggregated_entrypoint.sh"}, []string{"meta-service"}
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
	case dv1.DisaggregatedMS:
		return "/opt/apache-doris/ms_disaggregated_prestop.sh"

	default:
		return ""
	}
}
