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
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"
)

/*
please use get function to replace new function.
*/

func newCGStatefulsetName(ddcName /*dorisDisaggregatedCluster Name*/, cgName /*computeGroup's name*/ string) string {
	//ccName use "_", but name in kubernetes object use "-"
	stName := ddcName + "-" + cgName
	stName = strings.ReplaceAll(stName, "_", "-")
	return stName
}

func (ddc *DorisDisaggregatedCluster) GetCGStatefulsetName(cg *ComputeGroup) string {
	return newCGStatefulsetName(ddc.Name, cg.UniqueId)
}

func (ddc *DorisDisaggregatedCluster) GetInstanceId() string {
	instanceId := ddc.Namespace + "-" + ddc.Name
	// need config in vaultConfigMap.
	return instanceId
}

func (ddc *DorisDisaggregatedCluster) GetInstanceHashId() int64 {
	instanceId := ddc.Namespace + "-" + ddc.Name

	hasher := sha256.New()
	hasher.Write([]byte(instanceId))
	hashBytes := hasher.Sum(nil)
	hashInt := new(big.Int).SetBytes(hashBytes)

	rangeStart := big.NewInt(1000000000)
	rangeEnd := big.NewInt(2000000000)
	rangeSize := new(big.Int).Sub(rangeEnd, rangeStart)

	hashMod := new(big.Int).Mod(hashInt, rangeSize)
	res := new(big.Int).Add(hashMod, rangeStart)
	return res.Int64()
}

func (ddc *DorisDisaggregatedCluster) GetCGId(cg *ComputeGroup) string {
	if cg == nil || ddc == nil {
		return ""
	}
	//for _, ccs := range ddc.Status.ComputeGroupStatuses {
	//	if cg.Name == ccs.ComputeClusterName || cg.ClusterId == ccs.ClusterId {
	//		return cg.ClusterId
	//	}
	//}
	//
	//stsName := ddc.GetCGStatefulsetName(cg)
	////update cg' clusterId for auto assemble, if not config.
	//if cg.ClusterId == "" {
	//	cg.ClusterId = newCCId(ddc.Namespace, stsName)
	//}
	//
	//return cg.ClusterId
	stsName := ddc.GetCGStatefulsetName(cg)
	clusterId := strings.ReplaceAll(stsName, "-", "_")
	return clusterId
}

func (ddc *DorisDisaggregatedCluster) GetCGCloudUniqueIdPre() string {
	return newCGCloudUniqueIdPre(ddc.GetInstanceId())
}

func (ddc *DorisDisaggregatedCluster) GetFEStatefulsetName() string {
	return ddc.Name + "-" + "fe"
}

func (ddc *DorisDisaggregatedCluster) GetMSStatefulsetName() string {
	return ddc.Name + "-" + "ms"
}

func (ddc *DorisDisaggregatedCluster) GetCGServiceName(cg *ComputeGroup) string {
	svcName := ddc.Name + "-" + cg.UniqueId
	svcName = strings.ReplaceAll(svcName, "_", "-")
	return svcName
}

func (ddc *DorisDisaggregatedCluster) GetFEServiceName() string {
	return ddc.Name + "-" + "fe"
}

func (ddc *DorisDisaggregatedCluster) GetMSServiceName() string {
	return ddc.Name + "-" + "ms"
}
