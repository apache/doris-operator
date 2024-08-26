package resource

import (
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
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
