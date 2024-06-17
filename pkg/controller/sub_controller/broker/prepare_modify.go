package broker

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
func (bk *Controller) prepareStatefulsetApply(ctx context.Context, cluster *v1.DorisCluster) error {
	initPhase := v1.Initializing
	if cluster.Status.BrokerStatus != nil && v1.IsReconcilingStatusPhase(cluster.Status.BrokerStatus) {
		initPhase = cluster.Status.BrokerStatus.ComponentCondition.Phase
	}
	status := &v1.ComponentStatus{
		ComponentCondition: v1.ComponentCondition{
			SubResourceName:    v1.GenerateComponentStatefulSetName(cluster, v1.Component_Broker),
			Phase:              initPhase,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}
	status.AccessService = v1.GenerateExternalServiceName(cluster, v1.Component_Broker)
	cluster.Status.BrokerStatus = status

	var oldSt appv1.StatefulSet
	err := bk.K8sclient.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: v1.GenerateComponentStatefulSetName(cluster, v1.Component_Broker)}, &oldSt)
	if err != nil {
		klog.Infof("broker controller controlClusterPhaseAndPreOperation get StatefulSet failed, err: %s", err.Error())
		return nil
	}
	scaleNumber := *(cluster.Spec.BrokerSpec.Replicas) - *(oldSt.Spec.Replicas)
	// scale
	if scaleNumber != 0 { // set Phase as SCALING
		cluster.Status.BrokerStatus.ComponentCondition.Phase = v1.Scaling
		if err := k8s.SetDorisClusterPhase(ctx, bk.K8sclient, cluster.Name, cluster.Namespace, v1.Scaling, v1.Component_Broker); err != nil {
			klog.Errorf("broker SetDorisClusterPhase 'SCALING' failed err:%s ", err.Error())
			return err
		}
	}
	if scaleNumber < 0 {
		return nil
	}

	//TODO check upgrade ,restart

	return nil
}
