package cn

import (
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
)

func (cn *Controller) buildCnStatefulSet(dcr *v1.DorisCluster) appv1.StatefulSet {
	statefulSet := resource.NewStatefulSet(dcr, v1.Component_CN)
	statefulSet.Spec.Template = cn.buildCnPodTemplateSpec(dcr)
	return statefulSet
}
