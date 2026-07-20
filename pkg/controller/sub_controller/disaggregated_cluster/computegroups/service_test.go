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
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newServiceTestDDC() *dv1.DorisDisaggregatedCluster {
	return &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ddc",
			Namespace: "default",
			UID:       "test-uid",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "disaggregated.cluster.doris.com/v1",
			Kind:       "DorisDisaggregatedCluster",
		},
	}
}

func newServiceTestCG(uniqueId string) *dv1.ComputeGroup {
	return &dv1.ComputeGroup{
		UniqueId: uniqueId,
	}
}

func TestGetCGExternalServiceName(t *testing.T) {
	ddc := newServiceTestDDC()
	cg := newServiceTestCG("cg1")
	if actual, expected := ddc.GetCGExternalServiceName(cg), "test-ddc-cg1-external"; actual != expected {
		t.Errorf("GetCGExternalServiceName() = %s, want %s", actual, expected)
	}
}

func TestGetCGExternalServiceNameWithUnderscore(t *testing.T) {
	ddc := newServiceTestDDC()
	cg := newServiceTestCG("cg_group_1")
	if actual, expected := ddc.GetCGExternalServiceName(cg), "test-ddc-cg-group-1-external"; actual != expected {
		t.Errorf("GetCGExternalServiceName() = %s, want %s", actual, expected)
	}
}

func TestNewInternalService(t *testing.T) {
	dcgs := &DisaggregatedComputeGroupsController{}
	ddc := newServiceTestDDC()
	cg := newServiceTestCG("cg1")

	svc := dcgs.newInternalService(ddc, cg, map[string]interface{}{})

	if svc.Name != "test-ddc-cg1" {
		t.Errorf("internal service name = %s, want test-ddc-cg1", svc.Name)
	}
	if svc.Spec.ClusterIP != "None" {
		t.Errorf("internal service ClusterIP = %s, want None", svc.Spec.ClusterIP)
	}
	if !svc.Spec.PublishNotReadyAddresses {
		t.Error("internal service PublishNotReadyAddresses should be true")
	}
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("internal service should expose only heartbeat port, got %d ports", len(svc.Spec.Ports))
	}
	if svc.Spec.Ports[0].Name != resource.GetPortKey(resource.HEARTBEAT_SERVICE_PORT) {
		t.Errorf("internal service port = %s, want heartbeat port", svc.Spec.Ports[0].Name)
	}
	if svc.Labels[dv1.ServiceRoleForCluster] != string(dv1.Service_Role_Internal) {
		t.Errorf("internal service role = %s, want %s", svc.Labels[dv1.ServiceRoleForCluster], dv1.Service_Role_Internal)
	}
	if len(svc.OwnerReferences) != 1 || svc.OwnerReferences[0].Name != "test-ddc" {
		t.Error("internal service should have correct OwnerReference")
	}
}

func TestNewExternalService(t *testing.T) {
	dcgs := &DisaggregatedComputeGroupsController{}
	ddc := newServiceTestDDC()
	cg := newServiceTestCG("cg1")
	cg.CommonSpec.Service = &dv1.ExportService{
		Type:        corev1.ServiceTypeLoadBalancer,
		Annotations: map[string]string{"service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type": "intranet"},
	}

	svc := dcgs.newExternalService(ddc, cg, map[string]interface{}{})

	if svc.Name != "test-ddc-cg1-external" {
		t.Errorf("external service name = %s, want test-ddc-cg1-external", svc.Name)
	}
	if svc.Spec.ClusterIP == "None" {
		t.Error("external service should not be headless")
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("external service type = %s, want LoadBalancer", svc.Spec.Type)
	}
	if svc.Annotations["service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type"] != "intranet" {
		t.Error("external service should keep ExportService annotations")
	}
	if svc.Spec.SessionAffinity != corev1.ServiceAffinityNone {
		t.Errorf("external service SessionAffinity = %s, want None", svc.Spec.SessionAffinity)
	}
	if len(svc.Spec.Ports) < 4 {
		t.Errorf("external service should expose BE access ports, got %d ports", len(svc.Spec.Ports))
	}
	if len(svc.OwnerReferences) != 1 || svc.OwnerReferences[0].Name != "test-ddc" {
		t.Error("external service should have correct OwnerReference")
	}
}

func TestNewExternalServiceWithNodePortConfig(t *testing.T) {
	dcgs := &DisaggregatedComputeGroupsController{}
	ddc := newServiceTestDDC()
	cg := newServiceTestCG("cg1")
	cg.CommonSpec.Service = &dv1.ExportService{
		Type:        corev1.ServiceTypeNodePort,
		Annotations: map[string]string{"cloud.provider/lb": "true"},
	}

	svc := dcgs.newExternalService(ddc, cg, map[string]interface{}{})

	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("external service type = %s, want NodePort", svc.Spec.Type)
	}
	if svc.Annotations["cloud.provider/lb"] != "true" {
		t.Error("external service should have annotations from ExportService config")
	}
}

func TestInternalAndExternalServiceSelectors(t *testing.T) {
	dcgs := &DisaggregatedComputeGroupsController{}
	ddc := newServiceTestDDC()
	cg := newServiceTestCG("cg1")

	internalSvc := dcgs.newInternalService(ddc, cg, map[string]interface{}{})
	externalSvc := dcgs.newExternalService(ddc, cg, map[string]interface{}{})

	if len(internalSvc.Spec.Selector) != len(externalSvc.Spec.Selector) {
		t.Fatal("internal and external services should use the same pod selector")
	}
	for key, expected := range internalSvc.Spec.Selector {
		if actual := externalSvc.Spec.Selector[key]; actual != expected {
			t.Errorf("selector %s = %s, want %s", key, actual, expected)
		}
	}
}
