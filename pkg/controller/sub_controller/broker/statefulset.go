package broker

import (
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
)

func (broker *Controller) buildBKStatefulSet(dcr *v1.DorisCluster) appv1.StatefulSet {
	st := resource.NewStatefulSet(dcr, v1.Component_Broker)
	st.Spec.Template = broker.buildBrokerPodTemplateSpec(dcr)
	return st
}
