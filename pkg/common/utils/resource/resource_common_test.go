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

//contains the resource for test functions.
import (
	v1 "github.com/apache/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
	kr "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var dcr = &v1.DorisCluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
		Labels: map[string]string{
			"name":      "test",
			"namespace": "default",
		},
	},
	Spec: v1.DorisClusterSpec{
		AdminUser: &v1.AdminUser{
			Name:     "root",
			Password: "123456",
		},
		FeSpec: &v1.FeSpec{
			ElectionNumber: pointer.Int32(1),
			BaseSpec: v1.BaseSpec{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]kr.Quantity{
						"cpu":    kr.MustParse("4"),
						"memory": kr.MustParse("8Gi"),
					},
				},
				Replicas: pointer.Int32(1),
				Image:    "selectdb/doris.fe-ubuntu:latest",
				Service: &v1.ExportService{
					Type: "NodePort",
					Annotations: Annotations{
						"name":      "test",
						"namespace": "default",
					},
				},
				PersistentVolumes: []v1.PersistentVolume{
					{
						MountPath: "/opt/apache-doris/fe/doris-meta",
						Annotations: Annotations{
							"name":      "test",
							"component": "fe",
						},
						PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]kr.Quantity{
									"storage": kr.MustParse("100Gi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						},
					},
					{
						MountPath: "/opt/apache-doris/fe/log",
						Annotations: Annotations{
							"name":      "test",
							"component": "fe",
						},
						PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]kr.Quantity{
									"storage": kr.MustParse("100Gi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						},
					},
				},
			},
		},
		BeSpec: &v1.BeSpec{
			BaseSpec: v1.BaseSpec{
				Replicas: pointer.Int32(3),
				Image:    "selectdb/doris.be-ubuntu:latest",
				Service: &v1.ExportService{
					Type: "NodePort",
					Annotations: Annotations{
						"name":      "test",
						"namespace": "default",
					},
				},
				PersistentVolumes: []v1.PersistentVolume{
					{
						MountPath: "/opt/apache-doris/be/storage",
						Annotations: Annotations{
							"name":      "test",
							"component": "be",
						},
						PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]kr.Quantity{
									"storage": kr.MustParse("200Gi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						},
					},
					{
						MountPath: "/opt/apache-doris/be/log",
						Annotations: Annotations{
							"name":      "test",
							"component": "be",
						},
						PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]kr.Quantity{
									"storage": kr.MustParse("100Gi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						},
					},
				},
			},
		},
		CnSpec: &v1.CnSpec{
			BaseSpec: v1.BaseSpec{
				Replicas: pointer.Int32(1),
				Image:    "selectdb/doris.be-ubuntu:latest",
				Service: &v1.ExportService{
					Type: "LoadBalancer",
				},
				PersistentVolumes: []v1.PersistentVolume{
					{
						MountPath: "/opt/apache-doris/be/log",
						Annotations: Annotations{
							"name":      "test",
							"component": "be",
						},
						PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]kr.Quantity{
									"storage": kr.MustParse("100Gi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						},
					},
					{
						MountPath: "/opt/apache-doris/be/storage",
						Annotations: Annotations{
							"name":      "test",
							"component": "be",
						},
						PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]kr.Quantity{
									"storage": kr.MustParse("100Gi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						},
					},
				},
			},
		},
		BrokerSpec: &v1.BrokerSpec{
			BaseSpec: v1.BaseSpec{
				Image:    "selectdb/doris.broker-ubuntu:latest",
				Replicas: pointer.Int32(1),
				Service: &v1.ExportService{
					Type: "ClusterIP",
				},
			},
		},
	},
}

var cm = map[string]interface{}{
	"http_port":              "8030",
	"rpc_port":               "9020",
	"query_port":             "9030",
	"edit_log_port":          "9010",
	"arrow_flight_sql_port":  "9090",
	"be_port":                "9060",
	"webserver_port":         "8040",
	"heartbeat_service_port": "9050",
	"brpc_port":              "8060",
}
