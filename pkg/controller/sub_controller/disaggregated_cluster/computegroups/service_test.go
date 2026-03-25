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

func newTestDDC() *dv1.DorisDisaggregatedCluster {
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

func newTestCG(uniqueId string) *dv1.ComputeGroup {
	return &dv1.ComputeGroup{
		UniqueId: uniqueId,
	}
}

func newTestController() *DisaggregatedComputeGroupsController {
	return &DisaggregatedComputeGroupsController{}
}

func Test_GetCGExternalServiceName(t *testing.T) {
	ddc := newTestDDC()
	cg := newTestCG("cg1")
	expected := "test-ddc-cg1-external"
	actual := ddc.GetCGExternalServiceName(cg)
	if actual != expected {
		t.Errorf("GetCGExternalServiceName() = %s, want %s", actual, expected)
	}
}

func Test_GetCGExternalServiceName_WithUnderscore(t *testing.T) {
	ddc := newTestDDC()
	cg := newTestCG("cg_group_1")
	expected := "test-ddc-cg-group-1-external"
	actual := ddc.GetCGExternalServiceName(cg)
	if actual != expected {
		t.Errorf("GetCGExternalServiceName() = %s, want %s", actual, expected)
	}
}

func Test_newInternalService(t *testing.T) {
	dcgs := newTestController()
	ddc := newTestDDC()
	cg := newTestCG("cg1")
	cvs := map[string]interface{}{}

	svc := dcgs.newInternalService(ddc, cg, cvs)

	// Verify service name
	expectedName := "test-ddc-cg1"
	if svc.Name != expectedName {
		t.Errorf("internal service name = %s, want %s", svc.Name, expectedName)
	}

	// Verify headless (ClusterIP: None)
	if svc.Spec.ClusterIP != "None" {
		t.Errorf("internal service ClusterIP = %s, want None", svc.Spec.ClusterIP)
	}

	// Verify publishNotReadyAddresses
	if !svc.Spec.PublishNotReadyAddresses {
		t.Error("internal service PublishNotReadyAddresses should be true")
	}

	// Verify only heartbeat port is exposed
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("internal service should have 1 port, got %d", len(svc.Spec.Ports))
	}
	if svc.Spec.Ports[0].Name != resource.GetPortKey(resource.HEARTBEAT_SERVICE_PORT) {
		t.Errorf("internal service port name = %s, want %s", svc.Spec.Ports[0].Name, resource.GetPortKey(resource.HEARTBEAT_SERVICE_PORT))
	}

	// Verify internal service role label
	if svc.Labels[dv1.ServiceRoleForCluster] != string(dv1.Service_Role_Internal) {
		t.Errorf("internal service role label = %s, want %s", svc.Labels[dv1.ServiceRoleForCluster], string(dv1.Service_Role_Internal))
	}

	// Verify OwnerReference
	if len(svc.OwnerReferences) != 1 || svc.OwnerReferences[0].Name != "test-ddc" {
		t.Error("internal service should have correct OwnerReference")
	}
}

func Test_newExternalService(t *testing.T) {
	dcgs := newTestController()
	ddc := newTestDDC()
	cg := newTestCG("cg1")
	cvs := map[string]interface{}{}

	svc := dcgs.newExternalService(ddc, cg, cvs)

	// Verify service name
	expectedName := "test-ddc-cg1-external"
	if svc.Name != expectedName {
		t.Errorf("external service name = %s, want %s", svc.Name, expectedName)
	}

	// Verify NOT headless
	if svc.Spec.ClusterIP == "None" {
		t.Error("external service should not be headless")
	}

	// Verify has all ports (be_port, webserver, heartbeat, brpc)
	if len(svc.Spec.Ports) < 4 {
		t.Errorf("external service should have at least 4 ports, got %d", len(svc.Spec.Ports))
	}

	// Verify OwnerReference
	if len(svc.OwnerReferences) != 1 || svc.OwnerReferences[0].Name != "test-ddc" {
		t.Error("external service should have correct OwnerReference")
	}
}

func Test_newExternalService_WithExportServiceConfig(t *testing.T) {
	dcgs := newTestController()
	ddc := newTestDDC()
	cg := newTestCG("cg1")
	cg.CommonSpec.Service = &dv1.ExportService{
		Type:        corev1.ServiceTypeNodePort,
		Annotations: map[string]string{"cloud.provider/lb": "true"},
	}
	cvs := map[string]interface{}{}

	svc := dcgs.newExternalService(ddc, cg, cvs)

	// Verify service type from ExportService config
	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("external service type = %s, want NodePort", svc.Spec.Type)
	}

	// Verify annotations from ExportService config
	if svc.Annotations["cloud.provider/lb"] != "true" {
		t.Error("external service should have annotations from ExportService config")
	}
}

func Test_newExternalService_LoadBalancer(t *testing.T) {
	dcgs := newTestController()
	ddc := newTestDDC()
	cg := newTestCG("cg1")
	cg.CommonSpec.Service = &dv1.ExportService{
		Type: corev1.ServiceTypeLoadBalancer,
	}
	cvs := map[string]interface{}{}

	svc := dcgs.newExternalService(ddc, cg, cvs)

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("external service type = %s, want LoadBalancer", svc.Spec.Type)
	}

	// Verify SessionAffinity is None for LoadBalancer
	if svc.Spec.SessionAffinity != corev1.ServiceAffinityNone {
		t.Errorf("external service SessionAffinity = %s, want None for LoadBalancer", svc.Spec.SessionAffinity)
	}
}

func Test_InternalAndExternalServiceSelectors(t *testing.T) {
	dcgs := newTestController()
	ddc := newTestDDC()
	cg := newTestCG("cg1")
	cvs := map[string]interface{}{}

	internalSvc := dcgs.newInternalService(ddc, cg, cvs)
	externalSvc := dcgs.newExternalService(ddc, cg, cvs)

	// Both services should have the same selector so they route to the same pods
	if len(internalSvc.Spec.Selector) != len(externalSvc.Spec.Selector) {
		t.Fatal("internal and external services should have the same number of selector labels")
	}
	for k, v := range internalSvc.Spec.Selector {
		if externalSvc.Spec.Selector[k] != v {
			t.Errorf("selector mismatch for key %s: internal=%s, external=%s", k, v, externalSvc.Spec.Selector[k])
		}
	}
}
