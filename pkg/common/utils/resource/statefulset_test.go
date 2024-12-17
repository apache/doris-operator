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
	v1 "github.com/apache/doris-operator/api/doris/v1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_NewStatefulset(t *testing.T) {
	cts := []v1.ComponentType{v1.Component_FE, v1.Component_BE, v1.Component_CN, v1.Component_Broker}
	for _, ct := range cts {
		st := NewStatefulSet(dcr, ct)
		t.Log(st)
	}
}

func Test_StatefulsetSetDeepEqual(t *testing.T) {
	nst := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
			Labels: map[string]string{
				"name":      "test",
				"namespace": "default",
			},
		},
		Spec: appv1.StatefulSetSpec{
			Replicas: GetInt32Pointer(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fe",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "selectdb/doris.fe-ubuntu:latest",
							Env:   buildEnvFromPod(),
							Name:  "fe",
						},
					},
				},
			},
		},
	}

	ost := nst.DeepCopy()
	envNotEqualNst := nst.DeepCopy()
	envNotEqualOst := nst.DeepCopy()
	envNotEqualOst.Spec.Template.Spec.Containers[0].Env = append(envNotEqualOst.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  config_env_name,
		Value: config_env_path,
	})

	annoExistEqualNst := nst.DeepCopy()
	annoExistEqualNst.Annotations = map[string]string{
		v1.ComponentResourceHash: "123456",
	}
	annoExistEqualOst := nst.DeepCopy()
	annoExistEqualOst.Annotations = map[string]string{
		v1.ComponentResourceHash: "123456",
	}

	nsts := []*appv1.StatefulSet{nst, envNotEqualNst, annoExistEqualNst}
	osts := []*appv1.StatefulSet{ost, envNotEqualOst, annoExistEqualOst}
	ress := []bool{true, false, true}
	for i := 0; i < len(nsts); i++ {
		res := StatefulSetDeepEqual(nsts[i], osts[i], false)
		if res != ress[i] {
			t.Errorf("statefulsetDeepEqual failed in index %d", i)
		}
	}
}
