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

package controller

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/api/doris/v1"
	"reflect"
	"sort"
)

func disAggregatedInconsistentStatus(ests *dv1.DorisDisaggregatedClusterStatus, ddc *dv1.DorisDisaggregatedCluster) bool {
	return reflect.DeepEqual(ests, ddc.Status)
}

func inconsistentStatus(status *v1.DorisClusterStatus, dcr *v1.DorisCluster) bool {
	return inconsistentFEStatus(status.FEStatus, dcr.Status.FEStatus) ||
		inconsistentBEStatus(status.BEStatus, dcr.Status.BEStatus) ||
		inconsistentCnStatus(status.CnStatus, dcr.Status.CnStatus) ||
		inconsistentBrokerStatus(status.BrokerStatus, dcr.Status.BrokerStatus)
}

func inconsistentCnStatus(eStatus *v1.CnStatus, nStatus *v1.CnStatus) bool {
	if eStatus == nil && nStatus == nil {
		return false
	}

	eComponentStatus := v1.ComponentStatus{}
	nComponentStatus := v1.ComponentStatus{}
	var eHorizontalScaler, nHorizontalScaler *v1.HorizontalScaler
	if eStatus != nil {
		eComponentStatus = eStatus.ComponentStatus
		eHorizontalScaler = eStatus.HorizontalScaler
	}
	if nStatus != nil {
		nComponentStatus = nStatus.ComponentStatus
		nHorizontalScaler = nStatus.HorizontalScaler
	}

	return inconsistentComponentStatus(&eComponentStatus, &nComponentStatus) || inconsistentHorizontalStatus(eHorizontalScaler, nHorizontalScaler)
}

func inconsistentFEStatus(eFeStatus *v1.ComponentStatus, nFeStatus *v1.ComponentStatus) bool {
	return inconsistentComponentStatus(eFeStatus, nFeStatus)
}

func inconsistentBEStatus(eBeStatus *v1.ComponentStatus, nBeStatus *v1.ComponentStatus) bool {
	return inconsistentComponentStatus(eBeStatus, nBeStatus)
}

func inconsistentBrokerStatus(eBkStatus *v1.ComponentStatus, nBkStatus *v1.ComponentStatus) bool {
	return inconsistentComponentStatus(eBkStatus, nBkStatus)
}

func inconsistentComponentStatus(eStatus *v1.ComponentStatus, nStatus *v1.ComponentStatus) bool {
	if eStatus == nil && nStatus == nil {
		return false
	}

	//&{AccessService:doriscluster-sample-fe-service FailedMembers:[] CreatingMembers:[doriscluster-sample-fe-0 doriscluster-sample-fe-1] RunningMembers:[] ComponentCondition:{SubResourceName:doriscluster-sample-fe Phase:initializing LastTransitionTime:2024-06-17 15:06:27.277201 +0800 CST m=+9.790029793 Reason: Message:}},
	//&{AccessService:doriscluster-sample-fe-service FailedMembers:[] CreatingMembers:[doriscluster-sample-fe-0 doriscluster-sample-fe-1] RunningMembers:[] ComponentCondition:{SubResourceName:doriscluster-sample-fe Phase:initializing LastTransitionTime:2024-06-17 15:06:27 T Reason: Message:}}
	// check resource status, if status not equal return true.
	if (eStatus == nil || nStatus == nil) ||
		eStatus.ComponentCondition != nStatus.ComponentCondition ||
		eStatus.AccessService != nStatus.AccessService {
		return true
	}

	//check control pods equal or not, if not return true.
	if !equalSplice(eStatus.CreatingMembers, nStatus.CreatingMembers) ||
		!equalSplice(eStatus.RunningMembers, nStatus.RunningMembers) ||
		!equalSplice(eStatus.FailedMembers, nStatus.FailedMembers) {
		return true
	}

	return false
}

func inconsistentHorizontalStatus(eh *v1.HorizontalScaler, nh *v1.HorizontalScaler) bool {
	if eh != nil && nh != nil {
		return eh.Name != nh.Name || eh.Version != nh.Version
	}

	if eh == nil && nh == nil {
		return false
	}
	return true
}

func equalSplice(e []string, n []string) bool {
	if len(e) != len(n) {
		return false
	}

	sort.Strings(e)
	sort.Strings(n)
	for i, _ := range e {
		if e[i] != n[i] {
			return false
		}
	}

	return true
}
