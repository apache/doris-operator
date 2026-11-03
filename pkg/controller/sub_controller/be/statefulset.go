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

package be

import (
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/ptr"
)

func (be *Controller) buildBEStatefulSet(dcr *v1.DorisCluster, config map[string]interface{}) appv1.StatefulSet {
	st := resource.NewStatefulSet(dcr, config, v1.Component_BE)
	st.Spec.Template = be.buildBEPodTemplateSpec(dcr, config)
	return st
}

func (be *Controller) useNewDefaultValuesInStatefulset(st *appv1.StatefulSet) {
	resource.UseNewDefaultInitContainerImage(&st.Spec.Template)
}

// statefulSetsEqual preserves the shared hash and legacy-env comparison, then
// verifies the replica count owned by the native BE controller.
func (be *Controller) statefulSetsEqual(new, old *appv1.StatefulSet) bool {
	be.RestrictConditionsEqual(new, old)
	equal := resource.StatefulSetDeepEqual(new, old, false)
	return equal && ptr.Equal(new.Spec.Replicas, old.Spec.Replicas)
}
