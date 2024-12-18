// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package resource

import (
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/magiconair/properties/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_BuildInternalService(t *testing.T) {
	for _, ct := range []v1.ComponentType{v1.Component_FE, v1.Component_CN, v1.Component_Broker, v1.Component_BE} {
		svc := BuildInternalService(dcr, ct, cm)
		t.Log(svc)
	}
}

func Test_BuildExternalService(t *testing.T) {
	for _, ct := range []v1.ComponentType{v1.Component_FE, v1.Component_CN, v1.Component_Broker, v1.Component_BE} {
		svc := BuildExternalService(dcr, ct, cm)
		t.Log(svc)
	}
}

func Test_GetDisaggregatedContainerPorts(t *testing.T) {
	fcps := []corev1.ContainerPort{
		{
			Name:          GetPortKey(HTTP_PORT),
			ContainerPort: GetPort(cm, HTTP_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(RPC_PORT),
			ContainerPort: GetPort(cm, RPC_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(QUERY_PORT),
			ContainerPort: GetPort(cm, QUERY_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(EDIT_LOG_PORT),
			ContainerPort: GetPort(cm, EDIT_LOG_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(ARROW_FLIGHT_SQL_PORT),
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: GetPort(cm, ARROW_FLIGHT_SQL_PORT),
		},
	}

	bcps := []corev1.ContainerPort{
		{
			Name:          GetPortKey(BE_PORT),
			ContainerPort: GetPort(cm, BE_PORT),
		}, {
			Name:          GetPortKey(WEBSERVER_PORT),
			ContainerPort: GetPort(cm, WEBSERVER_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(HEARTBEAT_SERVICE_PORT),
			ContainerPort: GetPort(cm, HEARTBEAT_SERVICE_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(BRPC_PORT),
			ContainerPort: GetPort(cm, BRPC_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(ARROW_FLIGHT_SQL_PORT),
			ContainerPort: GetPort(cm, ARROW_FLIGHT_SQL_PORT),
			Protocol:      corev1.ProtocolTCP,
		},
	}

	fgfps := GetDisaggregatedContainerPorts(cm, dv1.DisaggregatedFE)
	for i := 0; i < len(fcps); i++ {
		assert.Equal(t, fcps[i], fgfps[i])
	}

	bgcps := GetDisaggregatedContainerPorts(cm, dv1.DisaggregatedBE)
	for i := 0; i < len(bcps); i++ {
		assert.Equal(t, bcps[i], bgcps[i])
	}
}

func Test_ServiceDeepEqual(t *testing.T) {
	src := []corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1-hash",
				Namespace: "default",
				Annotations: Annotations{
					v1.ComponentResourceHash: "123456",
				},
			},
			Spec: corev1.ServiceSpec{},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2-annotation",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test3-query-port",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name: "query-port",
						Port: 9030,
					},
				},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test4-label",
				Namespace: "default",
				Labels: map[string]string{
					"test": "test",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name: "query-port",
						Port: 9030,
					},
				},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test5-selector",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name: "query-port",
						Port: 9030,
					},
				},
				Selector: map[string]string{
					"test": "test",
				},
			},
		},
	}
	dst := []corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1-hash",
				Namespace: "default",
				Annotations: Annotations{
					v1.ComponentResourceHash: "1234",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2-annotation",
				Namespace: "default",
				Annotations: Annotations{
					"test": "test",
				},
			},
			Spec: corev1.ServiceSpec{},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test3-query-port",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name: "query-port",
						Port: 9030,
					},
				},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test4-label",
				Namespace: "default",
				Labels: map[string]string{
					"test": "test",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name: "query-port",
						Port: 9030,
					},
				},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test5-selector",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name: "query-port",
						Port: 9030,
					},
				},
				Selector: map[string]string{
					"test": "test",
				},
			},
		},
	}
	ress := []bool{false, false, true, true, true}
	for i := 0; i < len(ress); i++ {
		res := ServiceDeepEqual(&src[i], &dst[i])
		if res != ress[i] {
			t.Errorf("the index %d not right.", i)
		}
	}
}

func Test_GetContainerPorts(t *testing.T) {
	fcps := []corev1.ContainerPort{
		{
			Name:          GetPortKey(HTTP_PORT),
			ContainerPort: GetPort(cm, HTTP_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(RPC_PORT),
			ContainerPort: GetPort(cm, RPC_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(QUERY_PORT),
			ContainerPort: GetPort(cm, QUERY_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(EDIT_LOG_PORT),
			ContainerPort: GetPort(cm, EDIT_LOG_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(ARROW_FLIGHT_SQL_PORT),
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: GetPort(cm, ARROW_FLIGHT_SQL_PORT),
		},
	}

	bcps := []corev1.ContainerPort{
		{
			Name:          GetPortKey(BE_PORT),
			ContainerPort: GetPort(cm, BE_PORT),
		}, {
			Name:          GetPortKey(WEBSERVER_PORT),
			ContainerPort: GetPort(cm, WEBSERVER_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(HEARTBEAT_SERVICE_PORT),
			ContainerPort: GetPort(cm, HEARTBEAT_SERVICE_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(BRPC_PORT),
			ContainerPort: GetPort(cm, BRPC_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          GetPortKey(ARROW_FLIGHT_SQL_PORT),
			ContainerPort: GetPort(cm, ARROW_FLIGHT_SQL_PORT),
			Protocol:      corev1.ProtocolTCP,
		},
	}
	brokercps := []corev1.ContainerPort{
		{
			Name:          GetPortKey(BROKER_IPC_PORT),
			ContainerPort: GetPort(cm, BROKER_IPC_PORT),
		},
	}
	fgcps := GetContainerPorts(cm, v1.Component_FE)
	bgcps := GetContainerPorts(cm, v1.Component_BE)
	brokergcps := GetContainerPorts(cm, v1.Component_Broker)
	for i := 0; i < len(fcps); i++ {
		assert.Equal(t, fcps[i], fgcps[i])
	}

	for i := 0; i < len(bcps); i++ {
		assert.Equal(t, bcps[i], bgcps[i])
	}
	for i := 0; i < len(brokergcps); i++ {
		assert.Equal(t, brokercps[i], brokergcps[i])
	}
}
