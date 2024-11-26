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
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_BuildHorizontalPodAutoScaler(t *testing.T) {
	tests := []*PodAutoscalerParams{
		{
			AutoscalerType: "v1",
			Namespace:      "default",
			Name:           "test",
			Labels:         map[string]string{"version": "v1", "name": "test"},
			TargetName:     "test-statefulset",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "Statefulset",
					APIVersion: "v1",
					Name:       "test",
					UID:        "12222333",
				},
			},
			ScalerPolicy: &dorisv1.AutoScalingPolicy{
				Version:     "v1",
				MinReplicas: GetInt32Pointer(1),
				MaxReplicas: 2,
				HPAPolicy: &dorisv1.HPAPolicy{
					Metrics:  []dorisv1.MetricSpec{},
					Behavior: &dorisv1.HorizontalPodAutoscalerBehavior{},
				},
			},
		},
		{
			AutoscalerType: "v2",
			Namespace:      "default",
			Name:           "test",
			Labels:         map[string]string{"version": "v2", "name": "test"},
			TargetName:     "test-statefulset",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "Statefulset",
					APIVersion: "v1",
					Name:       "test",
					UID:        "12222333",
				},
			},
			ScalerPolicy: &dorisv1.AutoScalingPolicy{
				Version:     "v2",
				MinReplicas: GetInt32Pointer(1),
				MaxReplicas: 2,
				HPAPolicy: &dorisv1.HPAPolicy{
					Metrics:  []dorisv1.MetricSpec{},
					Behavior: &dorisv1.HorizontalPodAutoscalerBehavior{},
				},
			},
		},
		{},
	}

	for _, test := range tests {
		BuildHorizontalPodAutoscaler(test)
	}
}
