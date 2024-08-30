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

package v1

import (
	"fmt"
	"strings"
)

/*
please use get function to replace new function.
*/

func newCCStatefulsetName(ddcName /*dorisDisaggregatedCluster Name*/, ccName /*computeCluster's name*/ string) string {
	//ccName use "_", but name in kubernetes object use "-"
	stName := ddcName + "-" + ccName
	stName = strings.ReplaceAll(stName, "_", "-")
	return stName
}

// RE:[a-zA-Z][0-9a-zA-Z_]+
func newCCId(namespace, stsName string) string {
	return strings.ReplaceAll(namespace+"_"+stsName, "-", "_")
}

func newCCCloudUniqueIdPre(instanceId string) string {
	return fmt.Sprintf("1:%s", instanceId)
}

func (ddc *DorisDisaggregatedCluster) GetCCStatefulsetName(cc *ComputeCluster) string {
	ccStsName := ""

	newStsName := newCCStatefulsetName(ddc.Name, cc.Name)
	stsNameExtist := false

	for _, ccs := range ddc.Status.ComputeClusterStatuses {
		if ccs.ClusterId == cc.ClusterId {
			ccStsName = ccs.StatefulsetName
		}
		if ccs.StatefulsetName == newStsName {
			stsNameExtist = true
		}
	}
	//check stsName is existed. Prevent stsname conflicts when modifying ccName
	if ccStsName == "" && !stsNameExtist {
		return newStsName
	}

	return ccStsName
}

func (ddc *DorisDisaggregatedCluster) GetInstanceId() string {
	if ddc.Status.InstanceId != "" {
		return ddc.Status.InstanceId
	}

	// need config in vaultConfigMap.
	return ""
}

func (ddc *DorisDisaggregatedCluster) GetCCId(cc *ComputeCluster) string {
	if cc == nil || ddc == nil {
		return ""
	}

	if cc.ClusterId == "" {
		stsName := ddc.GetCCStatefulsetName(cc)
		id := newCCId(ddc.Namespace, stsName)
		//check clusterId is exist.
		for _, ccs := range ddc.Status.ComputeClusterStatuses {
			if ccs.ClusterId == id {
				return ""
			}
		}
		cc.ClusterId = id
		return cc.ClusterId
	}

	for _, ccs := range ddc.Status.ComputeClusterStatuses {
		if cc.ClusterId == ccs.ClusterId {
			return cc.ClusterId
		}
	}
	return ""
}

func (ddc *DorisDisaggregatedCluster) GetCCCloudUniqueIdPre() string {
	return newCCCloudUniqueIdPre(ddc.GetInstanceId())
}

func (ddc *DorisDisaggregatedCluster) GetFEStatefulsetName() string {
	return ddc.Name + "-" + "fe"
}

func (ddc *DorisDisaggregatedCluster) GetMSStatefulsetName() string {
	return ddc.Name + "-" + "ms"
}

func (ddc *DorisDisaggregatedCluster) GetCCServiceName(cc *ComputeCluster) string {
	return ddc.GetCCStatefulsetName(cc)
}

func (ddc *DorisDisaggregatedCluster) GetFEServiceName() string {
	return ddc.Name + "-" + "fe"
}

func (ddc *DorisDisaggregatedCluster) GetMSServiceName() string {
	return ddc.Name + "-" + "ms"
}
