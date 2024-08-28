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

package computeclusters

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (dccs *DisaggregatedComputeClustersController) newService(ddc *dv1.DorisDisaggregatedCluster, cc *dv1.ComputeCluster, cvs map[string]interface{}) *corev1.Service {
	ccClusterId := ddc.GetCCId(cc)
	svcConf := cc.CommonSpec.Service
	sps := newComputeServicePorts(cvs, svcConf)
	svc := dccs.NewDefaultService(ddc)

	ob := &svc.ObjectMeta
	ob.Name = ddc.GetCCServiceName(cc)
	ob.Labels = dccs.newCC2LayerSchedulerLabels(ddc.Namespace, ccClusterId)

	spec := &svc.Spec
	spec.Selector = dccs.newCCPodsSelector(ddc.Name, ccClusterId)
	spec.Ports = sps

	if svcConf != nil && svcConf.Type != "" {
		svc.Spec.Type = svcConf.Type
	}
	if svcConf != nil {
		svc.Annotations = svcConf.Annotations
	}

	// The external load balancer provided by the cloud provider may cause the client IP received by the service to change.
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		svc.Spec.SessionAffinity = corev1.ServiceAffinityNone
	}

	return svc
}

// new ports by start config that mounted into container by configMap.
func newComputeServicePorts(cvs map[string]interface{}, svcConf *dv1.ExportService) []corev1.ServicePort {
	bePort := resource.GetPort(cvs, resource.BE_PORT)
	webserverPort := resource.GetPort(cvs, resource.WEBSERVER_PORT)
	heartbeatPort := resource.GetPort(cvs, resource.HEARTBEAT_SERVICE_PORT)
	brpcPort := resource.GetPort(cvs, resource.BRPC_PORT)
	sps := []corev1.ServicePort{{
		Name:       resource.GetPortKey(resource.BE_PORT),
		TargetPort: intstr.FromInt(int(bePort)),
		Port:       bePort,
	}, {
		Name:       resource.GetPortKey(resource.WEBSERVER_PORT),
		TargetPort: intstr.FromInt(int(webserverPort)),
		Port:       webserverPort,
	}, {
		Name:       resource.GetPortKey(resource.HEARTBEAT_SERVICE_PORT),
		TargetPort: intstr.FromInt(int(heartbeatPort)),
		Port:       heartbeatPort,
	}, {
		Name:       resource.GetPortKey(resource.BRPC_PORT),
		TargetPort: intstr.FromInt(int(brpcPort)),
		Port:       brpcPort,
	}}

	if svcConf == nil || svcConf.Type != corev1.ServiceTypeNodePort {
		return sps
	}

	for i, _ := range sps {
		for j, _ := range svcConf.PortMaps {
			if sps[i].Port == svcConf.PortMaps[j].TargetPort {
				sps[i].NodePort = svcConf.PortMaps[j].NodePort
			}
		}
	}

	return sps
}
