package be

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
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
	//}

	var oldSt appv1.StatefulSet
	err := be.K8sclient.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: v1.GenerateComponentStatefulSetName(cluster, v1.Component_BE)}, &oldSt)
	if err != nil {
		klog.Infof("be controller controlClusterPhaseAndPreOperation get StatefulSet failed, err: %s", err.Error())
		return nil
	}
	scaleNumber := *(cluster.Spec.BeSpec.Replicas) - *(oldSt.Spec.Replicas)
	// scale
	if scaleNumber != 0 { // set Phase as SCALING
		cluster.Status.BEStatus.ComponentCondition.Phase = v1.Scaling
		if err := k8s.SetDorisClusterPhase(ctx, be.K8sclient, cluster.Name, cluster.Namespace, v1.Scaling, v1.Component_BE); err != nil {
			klog.Errorf("be SetDorisClusterPhase 'SCALING' failed err:%s ", err.Error())
			return err
		}
	}
	if scaleNumber < 0 {
		return nil
	}

	//TODO check upgrade ,restart

	return nil
}
