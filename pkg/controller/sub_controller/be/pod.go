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
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"strconv"

	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
)

func (be *Controller) buildBEPodTemplateSpec(dcr *v1.DorisCluster) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewPodTemplateSpec(dcr, v1.Component_BE)
	//if enable fe affinity, should not add fe antiAffinity and set the weight of affinity less than be antiAffinity.
	if dcr.Spec.BeSpec.EnableFeAffinity == true {
		be.addFeAffinity(&podTemplateSpec)
	} else {
		be.addFeAntiAffinity(&podTemplateSpec)
	}

	be.addTerminationGracePeriodSeconds(dcr, &podTemplateSpec)

	var containers []corev1.Container
	containers = append(containers, podTemplateSpec.Spec.Containers...)
	beContainer := be.beContainer(dcr)
	containers = append(containers, beContainer)

	if dcr.Spec.BeSpec.EnableWorkloadGroup {
		if dcr.Spec.BeSpec.ContainerSecurityContext == nil {
			dcr.Spec.BeSpec.ContainerSecurityContext = &corev1.SecurityContext{}
		}
		dcr.Spec.BeSpec.ContainerSecurityContext.Privileged = pointer.Bool(true)
	}
	containers = resource.ApplySecurityContext(containers, dcr.Spec.BeSpec.ContainerSecurityContext)

	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

// @Notice, the logic is error, should use MatchExpressions not matchLabels, the label used for select nodes, and the key:value "kubernetes.io/hostname=fe" is not exist in default k8s without assign to node by manual.
// although, the code is not harmless, so for stable the codes not need deleted.
// be pods add fe anti affinity for prefer deploy fe and be on different nodes.
func (be *Controller) addFeAntiAffinity(tplSpec *corev1.PodTemplateSpec) {
	preferedScheduleTerm := corev1.WeightedPodAffinityTerm{
		Weight: 80,
		PodAffinityTerm: corev1.PodAffinityTerm{
			TopologyKey: resource.NODE_TOPOLOGYKEY,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					resource.NODE_TOPOLOGYKEY: string(v1.Component_FE),
				},
			},
		},
	}

	if tplSpec.Spec.Affinity == nil {
		tplSpec.Spec.Affinity = &corev1.Affinity{}
	}
	if tplSpec.Spec.Affinity.PodAntiAffinity == nil {
		tplSpec.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	}

	tplSpec.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(tplSpec.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
		preferedScheduleTerm)
}

//aff fe affinity for be, wish the fe and be will 1:1 deployed in same node.
func (be *Controller) addFeAffinity(tplSpec *corev1.PodTemplateSpec) {
	pst := corev1.WeightedPodAffinityTerm{
		// the weight of be antiAffinity with be is 20.
		Weight: 15,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key: v1.ComponentLabelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{string(v1.Component_FE)},
					},
				},
			},
			TopologyKey: resource.NODE_TOPOLOGYKEY,
		},
	}

	if tplSpec.Spec.Affinity == nil {
		tplSpec.Spec.Affinity = &corev1.Affinity{}
	}
	if tplSpec.Spec.Affinity.PodAffinity == nil {
		tplSpec.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
	}
	tplSpec.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(tplSpec.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
		pst)
}

func (be *Controller) beContainer(dcr *v1.DorisCluster) corev1.Container {
	config, _ := be.GetConfig(context.Background(), &dcr.Spec.BeSpec.ConfigMapInfo, dcr.Namespace, v1.Component_BE)
	c := resource.NewBaseMainContainer(dcr, config, v1.Component_BE)
	addr, port := v1.GetConfigFEAddrForAccess(dcr, v1.Component_BE)
	var feConfig map[string]interface{}
	//if fe addr not config, we should use external service as addr and port get from fe config.
	if addr == "" {
		if dcr.Spec.FeSpec != nil {
			feConfig, _ = be.GetConfig(context.Background(), &dcr.Spec.FeSpec.ConfigMapInfo, dcr.Namespace, v1.Component_FE)
		}

		addr = v1.GenerateExternalServiceName(dcr, v1.Component_FE)
	}

	feQueryPort := strconv.FormatInt(int64(resource.GetPort(feConfig, resource.QUERY_PORT)), 10)
	if port != -1 {
		feQueryPort = strconv.FormatInt(int64(port), 10)
	}

	ports := resource.GetContainerPorts(config, v1.Component_BE)
	c.Name = "be"
	c.Ports = append(c.Ports, ports...)
	c.Env = append(c.Env, corev1.EnvVar{
		Name:  resource.ENV_FE_ADDR,
		Value: addr,
	}, corev1.EnvVar{
		Name:  resource.ENV_FE_PORT,
		Value: feQueryPort,
	})

	if dcr.Spec.BeSpec.EnableWorkloadGroup {
		c.Env = append(c.Env, corev1.EnvVar{
			Name:  resource.ENABLE_WORKLOAD_GROUP,
			Value: fmt.Sprintf("%t", dcr.Spec.BeSpec.EnableWorkloadGroup),
		})
	}

	return c
}

// Only configure the TerminationGracePeriodSeconds when grace_shutdown_wait_seconds configured in be.conf
func (be *Controller) addTerminationGracePeriodSeconds(dcr *v1.DorisCluster, tplSpec *corev1.PodTemplateSpec) {
	config, _ := be.GetConfig(context.Background(), &dcr.Spec.BeSpec.ConfigMapInfo, dcr.Namespace, v1.Component_BE)
	seconds := resource.GetTerminationGracePeriodSeconds(config)
	if seconds > 0 {
		tplSpec.Spec.TerminationGracePeriodSeconds = &seconds
		return
	}
	return
}
