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

package metaservice

import (
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateComponentStatusDowngradesMetaServicePhase(t *testing.T) {
	replicas := int32(2)
	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ddc",
			Namespace: "default",
		},
		Spec: dv1.DorisDisaggregatedClusterSpec{
			MetaService: dv1.MetaService{
				CommonSpec: dv1.CommonSpec{
					Replicas: &replicas,
				},
			},
		},
		Status: dv1.DorisDisaggregatedClusterStatus{
			MetaServiceStatus: dv1.MetaServiceStatus{
				AvailableStatus: dv1.Available,
				Phase:           dv1.Ready,
			},
		},
	}

	controller := &DisaggregatedMSController{}
	selector := controller.newMSPodsSelector(ddc.Name)
	sts := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ddc.GetMSStatefulsetName(),
			Namespace: ddc.Namespace,
		},
		Status: appv1.StatefulSetStatus{
			UpdateRevision: "revision-1",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ddc.GetMSStatefulsetName() + "-0",
			Namespace: ddc.Namespace,
			Labels:    selector,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	if err := appv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	controller.DisaggregatedSubDefaultController = sub_controller.DisaggregatedSubDefaultController{
		K8sclient: fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts, pod).Build(),
	}

	if err := controller.UpdateComponentStatus(ddc); err != nil {
		t.Fatalf("UpdateComponentStatus returned error: %v", err)
	}
	if ddc.Status.MetaServiceStatus.AvailableStatus != dv1.Available {
		t.Fatalf("available status = %s, want %s", ddc.Status.MetaServiceStatus.AvailableStatus, dv1.Available)
	}
	if ddc.Status.MetaServiceStatus.Phase != dv1.Reconciling {
		t.Fatalf("phase = %s, want %s", ddc.Status.MetaServiceStatus.Phase, dv1.Reconciling)
	}
}
