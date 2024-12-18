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
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
	kr "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
	"testing"
)

func Test_NewPodTemplateSpec(t *testing.T) {
	for _, ct := range []v1.ComponentType{"fe", "be", "cn", "broker"} {
		pt := NewPodTemplateSpec(dcr, ct)
		t.Log(pt)
	}
}

func Test_NewContainerWithCommonSpec(t *testing.T) {
	cs := &dv1.CommonSpec{
		Replicas:                 pointer.Int32(1),
		Image:                    "selectdb/doris.be-ubuntu:latest",
		ContainerSecurityContext: &corev1.SecurityContext{},
		ResourceRequirements: corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]kr.Quantity{
				"cpu":    kr.MustParse("4"),
				"memory": kr.MustParse("8Gi"),
			},
		},
	}
	c := NewContainerWithCommonSpec(cs)
	t.Log(c)
}

func Test_NewPodTemplateSpecWithCommonSpec(t *testing.T) {
	tm := make(map[dv1.DisaggregatedComponentType]*dv1.CommonSpec)
	ccs := &dv1.CommonSpec{
		Replicas: pointer.Int32(1),
		Image:    "selectdb/doris.be-ubuntu:latest",
		ResourceRequirements: corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]kr.Quantity{
				"cpu":    kr.MustParse("4"),
				"memory": kr.MustParse("8Gi"),
			},
		},
		SystemInitialization: &dv1.SystemInitialization{
			InitImage: "selectdb/doris.alpine:latest",
		},
		PersistentVolume: &dv1.PersistentVolume{
			MountPaths:  []string{"/opt/apache-doris/be/storage"},
			LogNotStore: true,
			Annotations: map[string]string{
				"name":      "test",
				"namespace": "default",
			},
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]kr.Quantity{
						"storage": kr.MustParse("200Gi"),
					},
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{"ReadAndWriteOnce"},
			},
		},
	}
	tm[dv1.DisaggregatedBE] = ccs
	fcs := &dv1.CommonSpec{
		Replicas: pointer.Int32(1),
		Image:    "selectdb/doris.fe-ubuntu:latest",
		ResourceRequirements: corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]kr.Quantity{
				"cpu":    kr.MustParse("4"),
				"memory": kr.MustParse("8Gi"),
			},
		},
		PersistentVolume: &dv1.PersistentVolume{
			MountPaths:  []string{"/opt/apache-doris/fe/doris-meta"},
			LogNotStore: false,
			Annotations: map[string]string{
				"name":      "test",
				"namespace": "default",
			},
			PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]kr.Quantity{
						"storage": kr.MustParse("200Gi"),
					},
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{"ReadAndWriteOnce"},
			},
		},
	}
	tm[dv1.DisaggregatedFE] = fcs
	mcs := &dv1.CommonSpec{
		Replicas: pointer.Int32(1),
		Image:    "selectdb/doris.ms-ubuntu:latest",
		ResourceRequirements: corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]kr.Quantity{
				"cpu":    kr.MustParse("4"),
				"memory": kr.MustParse("8Gi"),
			},
		},
	}
	tm[dv1.DisaggregatedMS] = mcs

	for dct, cs := range tm {
		pts := NewPodTemplateSpecWithCommonSpec(cs, dct)
		t.Log(pts)
	}
}

func Test_NewBaseMainContainer(t *testing.T) {
	for _, dct := range []v1.ComponentType{v1.Component_FE, v1.Component_BE, v1.Component_CN, v1.Component_Broker} {
		c := NewBaseMainContainer(dcr, cm, dct)
		t.Log(c)
	}
}

func Test_LifeCycleWithPreStopScript(t *testing.T) {
	lcs := []*corev1.Lifecycle{nil, {}}
	for i, _ := range lcs {
		lc := LifeCycleWithPreStopScript(lcs[i], "/opt/apache-doris/prestop.sh")
		if lc.PreStop == nil {
			t.Errorf("build lifeCycleWithPreStopScript failed. %d", i)
		}
	}
}

func Test_BuildDisaggregatedProbe(t *testing.T) {
	c := &corev1.Container{}
	cs := &dv1.CommonSpec{
		StartTimeout: 600,
		LiveTimeout:  30,
	}
	BuildDisaggregatedProbe(c, cs, dv1.DisaggregatedBE)
	if c.StartupProbe == nil {
		t.Errorf("startupProbe not build")
	}
	fts := 600 / 5
	if c.StartupProbe.FailureThreshold != int32(fts) {
		t.Errorf("startupProbe failureThreshold build failed.")
	}
	if c.LivenessProbe.TimeoutSeconds != int32(30) {
		t.Errorf("livenessProbe TimeoutSeconds build failed.")
	}
}
