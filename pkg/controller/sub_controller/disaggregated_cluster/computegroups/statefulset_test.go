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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	dcgs := &DisaggregatedComputeGroupsController{}
	envs := dcgs.newSpecificEnvs(ddc, cg)

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

func TestUpdateCGStatus_KeepGracefulPhaseWhenAnnotationExists(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appv1 scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	replicas := int32(3)
	sts := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doris-cg1",
			Namespace: "default",
			Annotations: map[string]string{
				gracefulActionAnnotation: `{"type":"RollingUpdate","phase":"WaitBEAlive"}`,
			},
		},
		Spec: appv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					dv1.DorisDisaggregatedClusterName:          "doris",
					dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
					dv1.DorisDisaggregatedPodType:              "compute",
				},
			},
		},
		Status: appv1.StatefulSetStatus{
			UpdateRevision: "rev-new",
		},
	}

	pods := []runtime.Object{
		sts,
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "doris-cg1-0",
				Namespace: "default",
				Labels: map[string]string{
					dv1.DorisDisaggregatedClusterName:          "doris",
					dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
					dv1.DorisDisaggregatedPodType:              "compute",
					resource.POD_CONTROLLER_REVISION_HASH_KEY:  "rev-new",
				},
			},
			Status: corev1.PodStatus{
				Phase:             corev1.PodRunning,
				Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
				ContainerStatuses: []corev1.ContainerStatus{{Name: "compute", Ready: true}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "doris-cg1-1",
				Namespace: "default",
				Labels: map[string]string{
					dv1.DorisDisaggregatedClusterName:          "doris",
					dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
					dv1.DorisDisaggregatedPodType:              "compute",
					resource.POD_CONTROLLER_REVISION_HASH_KEY:  "rev-new",
				},
			},
			Status: corev1.PodStatus{
				Phase:             corev1.PodRunning,
				Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
				ContainerStatuses: []corev1.ContainerStatus{{Name: "compute", Ready: true}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "doris-cg1-2",
				Namespace: "default",
				Labels: map[string]string{
					dv1.DorisDisaggregatedClusterName:          "doris",
					dv1.DorisDisaggregatedComputeGroupUniqueId: "cg1",
					dv1.DorisDisaggregatedPodType:              "compute",
					resource.POD_CONTROLLER_REVISION_HASH_KEY:  "rev-new",
				},
			},
			Status: corev1.PodStatus{
				Phase:             corev1.PodRunning,
				Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
				ContainerStatuses: []corev1.ContainerStatus{{Name: "compute", Ready: true}},
			},
		},
	}

	dcgs := &DisaggregatedComputeGroupsController{}
	dcgs.K8sclient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pods...).Build()

	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "doris",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
	}
	cgs := &dv1.ComputeGroupStatus{
		UniqueId:        "cg1",
		StatefulsetName: "doris-cg1",
		Replicas:        replicas,
		Phase:           dv1.Ready,
	}

	if err := dcgs.updateCGStatus(ddc, cgs); err != nil {
		t.Fatalf("updateCGStatus failed: %v", err)
	}
	if cgs.Phase != dv1.GracefulRolling {
		t.Fatalf("expected phase %s, got %s", dv1.GracefulRolling, cgs.Phase)
	}
	if cgs.AvailableReplicas != replicas {
		t.Fatalf("expected available replicas %d, got %d", replicas, cgs.AvailableReplicas)
	}
}
