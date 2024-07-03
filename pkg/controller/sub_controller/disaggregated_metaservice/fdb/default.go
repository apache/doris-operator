package fdb

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	FDBSpecHashValueKey = "disaggregated.cluster.doris.com/fdbspec.hash"
	ProcessClassLabel   = "disaggregated.cluster.doris.com/fdb-cluster-name"
	ProcessGroupIDLabel = "disaggregated.cluster.doris.com/fdb-process-group-id"
	FoundationVersion   = "7.1.38"
)

func getDefaultResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
	}
}
