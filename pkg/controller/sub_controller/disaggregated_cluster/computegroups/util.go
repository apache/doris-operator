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

package computegroups

import (
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// regex
var (
	compute_group_name_regex = "[a-zA-Z](_?[0-9a-zA-Z])*"
	compute_group_id_regex   = "[a-zA-Z](_?[0-9a-zA-Z])*"
)

func ownerReference2ddc(obj client.Object, cluster *dv1.DorisDisaggregatedCluster) bool {
	if obj == nil {
		return false
	}

	ors := obj.GetOwnerReferences()
	for _, or := range ors {
		if or.Name == cluster.Name && or.UID == cluster.UID {
			return true
		}
	}

	return false
}

func getUniqueIdFromClientObject(obj client.Object) string {
	if obj == nil {
		return ""
	}
	labels := obj.GetLabels()
	return labels[dv1.DorisDisaggregatedComputeGroupUniqueId]
}
