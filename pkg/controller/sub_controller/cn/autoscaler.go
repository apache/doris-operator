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

package cn

import (
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
)

func (cn *Controller) generateAutoScalerName(dcr *dorisv1.DorisCluster) string {
	return dorisv1.GenerateComponentStatefulSetName(dcr, dorisv1.Component_CN) + "-autoscaler"
}

func (cn *Controller) buildCnAutoscalerParams(scalerInfo dorisv1.AutoScalingPolicy, target *appv1.StatefulSet, dcr *dorisv1.DorisCluster) *resource.PodAutoscalerParams {
	labels := resource.Labels{}
	labels.AddLabel(target.Labels)
	labels.Add(dorisv1.ComponentLabelKey, "autoscaler")

	return &resource.PodAutoscalerParams{
		Namespace:      target.Namespace,
		Name:           cn.generateAutoScalerName(dcr),
		Labels:         labels,
		AutoscalerType: dcr.Spec.CnSpec.AutoScalingPolicy.Version,
		TargetName:     target.Name,
		//use src as ownerReference for reconciling on autoscaler updated.
		OwnerReferences: target.OwnerReferences,
		ScalerPolicy:    &scalerInfo,
	}
}
