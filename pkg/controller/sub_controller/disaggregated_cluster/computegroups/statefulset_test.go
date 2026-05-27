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
	"k8s.io/utils/pointer"
)

func Test_NewPodTemplateSpec_TerminationGracePeriodSeconds(t *testing.T) {
	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ddc",
			Namespace: "default",
		},
	}
	cg := &dv1.ComputeGroup{
		UniqueId: "cg1",
		CommonSpec: dv1.CommonSpec{
			Replicas: pointer.Int32(1),
			Image:    "selectdb/doris.be-ubuntu:latest",
		},
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	pts := dcgs.NewPodTemplateSpec(ddc, map[string]string{}, map[string]interface{}{}, cg)
	if pts.Spec.TerminationGracePeriodSeconds == nil {
		t.Fatalf("expected BE terminationGracePeriodSeconds")
	}
	if *pts.Spec.TerminationGracePeriodSeconds != resource.DEFAULT_BE_TERMINATION_GRACE_PERIOD_SECONDS {
		t.Errorf("expected BE terminationGracePeriodSeconds=%d, got %d", resource.DEFAULT_BE_TERMINATION_GRACE_PERIOD_SECONDS, *pts.Spec.TerminationGracePeriodSeconds)
	}
}

func Test_newSpecificEnvs_AlwaysUseFQDNHostType(t *testing.T) {
	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ddc",
			Namespace: "default",
		},
	}
	cg := &dv1.ComputeGroup{
		UniqueId: "cg1",
	}
	cvs := map[string]interface{}{
		resource.ENABLE_FQDN: "false",
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	envs := dcgs.newSpecificEnvs(ddc, cg, cvs)

	if got := findEnvValue(envs, "HOST_TYPE"); got != resource.START_MODEL_FQDN {
		t.Fatalf("expected HOST_TYPE=%s, got %s", resource.START_MODEL_FQDN, got)
	}
}

func findEnvValue(envs []corev1.EnvVar, name string) string {
	for _, env := range envs {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}
