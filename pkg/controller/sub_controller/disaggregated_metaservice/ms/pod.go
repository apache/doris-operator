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

package ms

import (
	"context"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
)

func (dmc *Controller) buildMSPodTemplateSpec(dms *mv1.DorisDisaggregatedMetaService) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewDMSPodTemplateSpec(dms, mv1.Component_MS)
	var containers []corev1.Container
	config, _ := dmc.GetMSConfig(context.Background(), dms.Spec.MS.ConfigMaps, dms.Namespace, mv1.Component_MS)
	msContainer := dmc.msContainer(dms, config)
	containers = append(containers, msContainer)
	containers = resource.ApplySecurityContext(containers, dms.Spec.MS.ContainerSecurityContext)
	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

func (dmc *Controller) msContainer(dms *mv1.DorisDisaggregatedMetaService, config map[string]interface{}) corev1.Container {
	brpcPort := resource.GetPort(config, resource.BRPC_LISTEN_PORT)
	c := resource.NewDMSBaseMainContainer(dms, brpcPort, config, mv1.Component_MS)

	ports := resource.GetDMSContainerPorts(brpcPort, mv1.Component_MS)
	c.Name = "metaservice"
	c.Ports = append(c.Ports, ports...)

	return c
}
