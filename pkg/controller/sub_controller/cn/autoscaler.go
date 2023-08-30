package cn

import (
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
)

func (cn *Controller) generateAutoScalerName(dcr *dorisv1.DorisCluster) string {
	return dorisv1.GenerateComponentStatefulSetName(dcr, dorisv1.Component_CN) + "-autoscaler"
}

func (cn *Controller) buildCnAutoscalerParams(scalerInfo dorisv1.AutoScalingPolicy, target *appv1.StatefulSet, dcr *dorisv1.DorisCluster) *resource.PodAutoscalerParams {
	labels := resource.Labels{}
	labels.AddLabel(target.Labels)
	labels.Add(dorisv1.ComponentLabelKey, "autoscaler")

	return &resource.PodAutoscalerParams{
		Namespace:      target.Namespace,
		Name:           cn.generateAutoScalerName(dcr),
		Labels:         labels,
		AutoscalerType: dcr.Spec.CnSpec.AutoScalingPolicy.Version,
		TargetName:     target.Name,
		//use src as ownerReference for reconciling on autoscaler updated.
		OwnerReferences: target.OwnerReferences,
		ScalerPolicy:    &scalerInfo,
	}
}
