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
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/autoscaling/v1"
	v2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"unsafe"
)

var (
	AutoscalerKind  = "HorizontalPodAutoscaler"
	StatefulSetKind = "StatefulSet"
	ServiceKind     = "Service"
)

type PodAutoscalerParams struct {
	AutoscalerType  dorisv1.AutoScalerVersion
	Namespace       string
	Name            string
	Labels          Labels
	TargetName      string
	OwnerReferences []metav1.OwnerReference
	ScalerPolicy    *dorisv1.AutoScalingPolicy
}

func BuildHorizontalPodAutoscaler(pap *PodAutoscalerParams) client.Object {
	switch pap.AutoscalerType {
	case dorisv1.AutoScalerV1:
		return buildAutoscalerV1(pap)
	case dorisv1.AutoSclaerV2:
		return buildAutoscalerV2(pap)
	default:
		return nil
	}
}

// build v1 autoscaler
func buildAutoscalerV1(pap *PodAutoscalerParams) *v1.HorizontalPodAutoscaler {
	ha := &v1.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       AutoscalerKind,
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            pap.Name,
			Namespace:       pap.Namespace,
			Labels:          pap.Labels,
			OwnerReferences: pap.OwnerReferences,
		},
		Spec: v1.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: v1.CrossVersionObjectReference{
				Name:       pap.TargetName,
				Kind:       StatefulSetKind,
				APIVersion: appv1.SchemeGroupVersion.String(),
			},
			MaxReplicas: pap.ScalerPolicy.MaxReplicas,
			MinReplicas: pap.ScalerPolicy.MinReplicas,
		},
	}

	return ha
}

func buildAutoscalerV2(pap *PodAutoscalerParams) *v2.HorizontalPodAutoscaler {
	ha := &v2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       AutoscalerKind,
			APIVersion: v2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            pap.Name,
			Namespace:       pap.Namespace,
			Labels:          pap.Labels,
			OwnerReferences: pap.OwnerReferences,
		},
		Spec: v2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: v2.CrossVersionObjectReference{
				Name:       pap.TargetName,
				Kind:       StatefulSetKind,
				APIVersion: appv1.SchemeGroupVersion.String(),
			},
			MaxReplicas: pap.ScalerPolicy.MaxReplicas,
			MinReplicas: pap.ScalerPolicy.MinReplicas,
		},
	}

	//the codes use unsafe.Pointer to convert struct, when audit please notice the correctness about memory assign.
	if pap.ScalerPolicy != nil && pap.ScalerPolicy.HPAPolicy != nil {
		if len(pap.ScalerPolicy.HPAPolicy.Metrics) != 0 {
			metrics := unsafe.Slice((*v2.MetricSpec)(unsafe.Pointer(&pap.ScalerPolicy.HPAPolicy.Metrics[0])), len(pap.ScalerPolicy.HPAPolicy.Metrics))
			ha.Spec.Metrics = metrics
		}
		ha.Spec.Behavior = (*v2.HorizontalPodAutoscalerBehavior)(unsafe.Pointer(pap.ScalerPolicy.HPAPolicy.Behavior))
	}

	return ha
}
