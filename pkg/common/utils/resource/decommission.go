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

package resource

import (
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
)

type DecommissionPhase string

const (
	Decommissioned           DecommissionPhase = "Decommissioned"
	Decommissioning          DecommissionPhase = "Decommissioning"
	DecommissionAcceptable   DecommissionPhase = "DecommissionAcceptable"
	DecommissionPhaseUnknown DecommissionPhase = "Unknown"
)

type DecommissionTaskStatus struct {
	AllBackendsSize       int
	UnDecommissionedCount int
	DecommissioningCount  int
	DecommissionedCount   int
	BeKeepAmount          int
}

func ConstructDecommissionTaskStatus(allBackends []*mysql.Backend, cgKeepAmount int32) DecommissionTaskStatus {
	var unDecommissionedCount, decommissioningCount, decommissionedCount int

	for i := range allBackends {
		node := allBackends[i]
		if !node.SystemDecommissioned {
			unDecommissionedCount++
		} else {
			if node.TabletNum == 0 {
				decommissionedCount++
			} else {
				decommissioningCount++
			}
		}
	}

	return DecommissionTaskStatus{
		AllBackendsSize:       len(allBackends),
		UnDecommissionedCount: unDecommissionedCount,
		DecommissioningCount:  decommissioningCount,
		DecommissionedCount:   decommissionedCount,
		BeKeepAmount:          int(cgKeepAmount),
	}
}

func (d *DecommissionTaskStatus) GetDecommissionPhase() DecommissionPhase {
	if d.DecommissioningCount == 0 && d.DecommissionedCount == 0 && d.UnDecommissionedCount > d.BeKeepAmount {
		return DecommissionAcceptable
	}
	if d.UnDecommissionedCount == d.BeKeepAmount && d.DecommissioningCount > 0 {
		return Decommissioning
	}

	if d.UnDecommissionedCount == d.BeKeepAmount && d.UnDecommissionedCount+d.DecommissionedCount == d.AllBackendsSize {
		return Decommissioned
	}
	return DecommissionPhaseUnknown
}
