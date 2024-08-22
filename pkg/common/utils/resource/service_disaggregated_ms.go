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
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
)

func BuildDMSService(ddm *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType, brpcPort int32) corev1.Service {
	labels := mv1.GenerateServiceLabels(ddm, componentType)
	selector := mv1.GenerateServiceSelector(ddm, componentType)
	spec, _ := GetDMSBaseSpecFromCluster(ddm, componentType)
	exportService := spec.Service

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mv1.GenerateCommunicateServiceName(ddm, componentType),
			Namespace: ddm.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: ddm.APIVersion,
					Kind:       ddm.Kind,
					Name:       ddm.Name,
					UID:        ddm.UID,
				},
			},
		}}

	ports := []corev1.ServicePort{
		getDMSServicePort(brpcPort, componentType),
	}

	constructDMSServiceSpec(exportService, &svc, selector, ports)
	if exportService != nil {
		svc.Annotations = exportService.Annotations
	}

	return svc
}

func getDMSServicePort(brpcPort int32, componentType mv1.ComponentType) corev1.ServicePort {
	switch componentType {
	case mv1.Component_MS:
		return corev1.ServicePort{
			Name:       GetPortKey(BRPC_LISTEN_PORT),
			Port:       brpcPort,
			TargetPort: intstr.FromInt(int(brpcPort)),
		}
	case mv1.Component_RC:
		return corev1.ServicePort{
			Name:       GetPortKey(BRPC_LISTEN_PORT),
			Port:       brpcPort,
			TargetPort: intstr.FromInt(int(brpcPort)),
		}
	default:
		klog.Infof("getDMSInternalServicePort not supported the type %s", componentType)
		return corev1.ServicePort{}
	}
}

func GetDMSContainerPorts(brpcPort int32, componentType mv1.ComponentType) []corev1.ContainerPort {
	switch componentType {
	case mv1.Component_MS:
		return getMSContainerPorts(brpcPort)
	case mv1.Component_RC:
		return getMSContainerPorts(brpcPort)
	default:
		klog.Infof("GetDMSContainerPorts the componentType %s not supported.", componentType)
		return []corev1.ContainerPort{}
	}
}

func getMSContainerPorts(brpcPort int32) []corev1.ContainerPort {
	return []corev1.ContainerPort{{
		Name:          GetPortKey(BRPC_LISTEN_PORT),
		ContainerPort: brpcPort,
		Protocol:      corev1.ProtocolTCP,
	}}
}

func DMSServiceDeepEqual(newSvc, oldSvc *corev1.Service) bool {
	var newHashValue, oldHashValue string
	if _, ok := newSvc.Annotations[mv1.ComponentResourceHash]; ok {
		newHashValue = newSvc.Annotations[mv1.ComponentResourceHash]
	} else {
		newHashService := dmsServiceHashObject(newSvc)
		newHashValue = hash.HashObject(newHashService)
	}

	if _, ok := oldSvc.Annotations[mv1.ComponentResourceHash]; ok {
		oldHashValue = oldSvc.Annotations[mv1.ComponentResourceHash]
	} else {
		oldHashService := dmsServiceHashObject(oldSvc)
		oldHashValue = hash.HashObject(oldHashService)
	}

	// set hash value in annotation for avoiding deep equal.
	newSvc.Annotations = mergeMaps(newSvc.Annotations, map[string]string{mv1.ComponentResourceHash: newHashValue})
	return newHashValue == oldHashValue &&
		newSvc.Namespace == oldSvc.Namespace
}

// hash service for diff new generate service and old service in kubernetes.
func dmsServiceHashObject(svc *corev1.Service) hashService {
	annos := make(map[string]string, len(svc.Annotations))
	//for support service annotations, avoid hash value in annotations interfere equal comparison.
	for key, value := range svc.Annotations {
		if key == mv1.ComponentResourceHash {
			continue
		}

		annos[key] = value
	}

	return hashService{
		name:        svc.Name,
		namespace:   svc.Namespace,
		ports:       svc.Spec.Ports,
		selector:    svc.Spec.Selector,
		serviceType: svc.Spec.Type,
		labels:      svc.Labels,
		annotations: annos,
	}
}

func constructDMSServiceSpec(exportService *mv1.ExportService, svc *corev1.Service, selector map[string]string, ports []corev1.ServicePort) {
	var portMaps []mv1.PortMap
	if exportService != nil {
		portMaps = exportService.PortMaps
	}

	for _, portMap := range portMaps {
		for i, _ := range ports {
			if int(portMap.TargetPort) == ports[i].TargetPort.IntValue() {
				ports[i].NodePort = portMap.NodePort
			}
		}
	}

	svc.Spec = corev1.ServiceSpec{
		Selector:        selector,
		Ports:           ports,
		SessionAffinity: corev1.ServiceAffinityClientIP,
	}

	// The external load balancer provided by the cloud provider may cause the client IP received by the service to change.
	if exportService != nil && exportService.Type == corev1.ServiceTypeLoadBalancer {
		svc.Spec.SessionAffinity = corev1.ServiceAffinityNone
	}

	setDMSServiceType(exportService, svc)
}

func setDMSServiceType(svc *mv1.ExportService, service *corev1.Service) {
	service.Spec.Type = corev1.ServiceTypeClusterIP
	if svc != nil && svc.Type != "" {
		service.Spec.Type = svc.Type
	}

	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && svc.LoadBalancerIP != "" {
		service.Spec.LoadBalancerIP = svc.LoadBalancerIP
	}
}
