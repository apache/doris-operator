package cn

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// prepareStatefulsetApply means Pre-operation and status control on the client side
func (cn *Controller) prepareStatefulsetApply(ctx context.Context, cluster *v1.DorisCluster) error {
	initPhase := v1.Initializing
	if cluster.Status.CnStatus != nil && v1.IsReconcilingStatusPhase(&cluster.Status.CnStatus.ComponentStatus) {
		initPhase = cluster.Status.CnStatus.ComponentCondition.Phase
	}
	cs := &v1.CnStatus{
		ComponentStatus: v1.ComponentStatus{
			ComponentCondition: v1.ComponentCondition{
				SubResourceName:    v1.GenerateComponentStatefulSetName(cluster, v1.Component_CN),
				Phase:              initPhase,
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		},
	}

	if cluster.Spec.CnSpec.AutoScalingPolicy != nil {
		cs.HorizontalScaler = &v1.HorizontalScaler{
			Version: cluster.Spec.CnSpec.AutoScalingPolicy.Version,
			Name:    cn.generateAutoScalerName(cluster),
		}
	}

	cs.AccessService = v1.GenerateExternalServiceName(cluster, v1.Component_CN)
	cluster.Status.CnStatus = cs

	//TODO  upgrade, restart, scale

	return nil
}
