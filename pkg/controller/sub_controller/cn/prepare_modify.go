package cn

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

	var oldSt appv1.StatefulSet
	err := cn.K8sclient.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: v1.GenerateComponentStatefulSetName(cluster, v1.Component_CN)}, &oldSt)
	if err != nil {
		klog.Infof("cn controller controlClusterPhaseAndPreOperation get StatefulSet failed, err: %s", err.Error())
		return nil
	}
	scaleNumber := *(cluster.Spec.CnSpec.Replicas) - *(oldSt.Spec.Replicas)
	// scale
	if scaleNumber != 0 { // set Phase as SCALING
		cluster.Status.CnStatus.ComponentCondition.Phase = v1.Scaling
		if err := k8s.SetDorisClusterPhase(ctx, cn.K8sclient, cluster.Name, cluster.Namespace, v1.Scaling, v1.Component_CN); err != nil {
			klog.Errorf("cn SetDorisClusterPhase 'SCALING' failed err:%s ", err.Error())
			return err
		}
	}
	if scaleNumber < 0 {
		return nil
	}

	//TODO check upgrade ,restart

	return nil
}
