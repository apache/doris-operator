package be

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// prepareStatefulsetApply means Pre-operation and status control on the client side
func (be *Controller) prepareStatefulsetApply(ctx context.Context, cluster *v1.DorisCluster) error {
	initPhase := v1.Initializing
	if cluster.Status.BEStatus != nil && v1.IsReconcilingStatusPhase(cluster.Status.BEStatus) {
		initPhase = cluster.Status.BEStatus.ComponentCondition.Phase
	}

	status := &v1.ComponentStatus{
		ComponentCondition: v1.ComponentCondition{
			SubResourceName:    v1.GenerateComponentStatefulSetName(cluster, v1.Component_BE),
			Phase:              initPhase,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}
	status.AccessService = v1.GenerateExternalServiceName(cluster, v1.Component_BE)
	cluster.Status.BEStatus = status

	//TODO check upgrade ,restart, scale

	return nil
}
