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

package fe

import (
	"context"
	"strconv"

	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
)

var (
	Default_Election_Number int32 = 3
)

func (fc *Controller) buildFEPodTemplateSpec(dcr *v1.DorisCluster) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewPodTemplateSpec(dcr, v1.Component_FE)
	var containers []corev1.Container
	//containers = append(containers, podTemplateSpec.Spec.Containers...)
	config, _ := fc.GetConfig(context.Background(), &dcr.Spec.FeSpec.ConfigMapInfo, dcr.Namespace, v1.Component_FE)
	feContainer := fc.feContainer(dcr, config)
	containers = append(containers, feContainer)
	containers = resource.ApplySecurityContext(containers, dcr.Spec.FeSpec.ContainerSecurityContext)
	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

func (fc *Controller) feContainer(dcr *v1.DorisCluster, config map[string]interface{}) corev1.Container {
	c := resource.NewBaseMainContainer(dcr, config, v1.Component_FE)
	feAddr, port := v1.GetConfigFEAddrForAccess(dcr, v1.Component_FE)
	queryPort := strconv.FormatInt(int64(resource.GetPort(config, resource.QUERY_PORT)), 10)
	//if fe addr not config, use external service as addr, if port not config in configmap use default value.
	if feAddr == "" {
		feAddr = v1.GenerateExternalServiceName(dcr, v1.Component_FE)
	}
	if port != -1 {
		queryPort = strconv.FormatInt(int64(port), 10)
	}

	if dcr.Spec.FeSpec.ElectionNumber == nil {
		dcr.Spec.FeSpec.ElectionNumber = resource.GetInt32Pointer(Default_Election_Number)
	}

	ports := resource.GetContainerPorts(config, v1.Component_FE)
	c.Name = "fe"
	c.Ports = append(c.Ports, ports...)
	c.Env = append(c.Env, corev1.EnvVar{
		Name:  resource.ENV_FE_ADDR,
		Value: feAddr,
	}, corev1.EnvVar{
		Name:  resource.ENV_FE_PORT,
		Value: queryPort,
	})

	if dcr.Spec.FeSpec.ElectionNumber != nil {
		c.Env = append(c.Env, corev1.EnvVar{
			Name:  resource.ENV_FE_ELECT_NUMBER,
			Value: strconv.FormatInt(int64(*dcr.Spec.FeSpec.ElectionNumber), 10),
		})
	}

	return c
}
