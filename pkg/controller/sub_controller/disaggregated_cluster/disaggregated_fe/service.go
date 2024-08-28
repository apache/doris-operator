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

package disaggregated_fe

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (dfc *DisaggregatedFEController) newService(ddc *dv1.DorisDisaggregatedCluster, cvs map[string]interface{}) *corev1.Service {
	ddcService := ddc.Spec.FeSpec.CommonSpec.Service
	ports := newFEServicePorts(cvs, ddcService)
	svc := dfc.NewDefaultService(ddc)
	om := &svc.ObjectMeta
	om.Name = ddc.GetFEServiceName()
	om.Labels = dfc.newFESchedulerLabels(ddc.Namespace)

	spec := &svc.Spec
	spec.Selector = dfc.newFEPodsSelector(ddc.Name)
	spec.Ports = ports

	if ddcService != nil && ddcService.Type != "" {
		svc.Spec.Type = ddcService.Type
	}
	if ddcService != nil {
		svc.Annotations = ddcService.Annotations
	}

	// The external load balancer provided by the cloud provider may cause the client IP received by the service to change.
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		svc.Spec.SessionAffinity = corev1.ServiceAffinityNone
	}

	return svc
}

// new ports by start config that mounted into container by configMap.
func newFEServicePorts(config map[string]interface{}, svcConf *dv1.ExportService) []corev1.ServicePort {
	httpPort := resource.GetPort(config, resource.HTTP_PORT)
	rpcPort := resource.GetPort(config, resource.RPC_PORT)
	queryPort := resource.GetPort(config, resource.QUERY_PORT)
	editPort := resource.GetPort(config, resource.EDIT_LOG_PORT)
	ports := []corev1.ServicePort{
		{
			Port: httpPort, TargetPort: intstr.FromInt(int(httpPort)), Name: resource.GetPortKey(resource.HTTP_PORT),
		}, {
			Port: rpcPort, TargetPort: intstr.FromInt(int(rpcPort)), Name: resource.GetPortKey(resource.RPC_PORT),
		}, {
			Port: queryPort, TargetPort: intstr.FromInt(int(queryPort)), Name: resource.GetPortKey(resource.QUERY_PORT),
		}, {
			Port: editPort, TargetPort: intstr.FromInt(int(editPort)), Name: resource.GetPortKey(resource.EDIT_LOG_PORT),
		}}

	if svcConf == nil || svcConf.Type != corev1.ServiceTypeNodePort {
		return ports
	}

	for i, _ := range ports {
		for j, _ := range svcConf.PortMaps {
			if ports[i].Port == svcConf.PortMaps[j].TargetPort {
				ports[i].NodePort = svcConf.PortMaps[j].NodePort
			}
		}
	}

	return ports
}
