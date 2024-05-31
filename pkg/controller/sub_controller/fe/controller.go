package fe

import (
	"context"
	"errors"
	"fmt"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Controller struct {
	sub_controller.SubDefaultController
}

func (fc *Controller) ClearResources(ctx context.Context, cluster *v1.DorisCluster) (bool, error) {
	//if the doris is not have fe.
	if cluster.Status.FEStatus == nil {
		return true, nil
	}

	if cluster.DeletionTimestamp.IsZero() {
		return true, nil
	}

	return fc.ClearCommonResources(ctx, cluster, v1.Component_FE)
}

func (fc *Controller) UpdateComponentStatus(cluster *v1.DorisCluster) error {
	//if spec is not exist, status is empty. but before clear status we must clear all resource about be used by ClearResources.
	if cluster.Spec.FeSpec == nil {
		cluster.Status.FEStatus = nil
		return nil
	}

	fs := &v1.ComponentStatus{
		ComponentCondition: v1.ComponentCondition{
			SubResourceName:    v1.GenerateComponentStatefulSetName(cluster, v1.Component_FE),
			Phase:              v1.Reconciling,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}

	if cluster.Status.FEStatus != nil {
		fs = cluster.Status.FEStatus.DeepCopy()
	}

	cluster.Status.FEStatus = fs
	fs.AccessService = v1.GenerateExternalServiceName(cluster, v1.Component_FE)

	return fc.ClassifyPodsByStatus(cluster.Namespace, fs, v1.GenerateStatefulSetSelector(cluster, v1.Component_FE), *cluster.Spec.FeSpec.Replicas)
}

func (fc *Controller) GetComponentStatus(cluster *v1.DorisCluster) v1.ComponentPhase {
	return cluster.Status.FEStatus.ComponentCondition.Phase
}

// New construct a FeController.
func New(k8sclient client.Client, k8sRecorder record.EventRecorder) *Controller {
	return &Controller{
		SubDefaultController: sub_controller.SubDefaultController{
			K8sclient:   k8sclient,
			K8srecorder: k8sRecorder,
		},
	}
}

func (fc *Controller) GetControllerName() string {
	return "feController"
}

// Sync DorisCluster to fe statefulset and service.
func (fc *Controller) Sync(ctx context.Context, cluster *v1.DorisCluster) error {
	if cluster.Spec.FeSpec == nil {
		klog.Info("fe Controller Sync ", "the fe component is not needed ", "namespace ", cluster.Namespace, " doris cluster name ", cluster.Name)
		return nil
	}

	feSpec := cluster.Spec.FeSpec
	//get the fe configMap for resolve ports.
	config, err := fc.GetConfig(ctx, &feSpec.BaseSpec.ConfigMapInfo, cluster.Namespace, v1.Component_FE)
	if err != nil {
		klog.Error("fe Controller Sync ", "resolve fe configmap failed, namespace ", cluster.Namespace, " error :", err)
		return err
	}
	fc.CheckConfigMountPath(cluster, v1.Component_FE)

	//generate new fe service.
	svc := resource.BuildExternalService(cluster, v1.Component_FE, config)
	//create or update fe external and domain search service, update the status of fe on src.
	internalService := resource.BuildInternalService(cluster, v1.Component_FE, config)
	if err := k8s.ApplyService(ctx, fc.K8sclient, &internalService, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("fe controller sync apply internalService name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			internalService.Name, internalService.Namespace, cluster.Name, err.Error())
		return err
	}
	if err := k8s.ApplyService(ctx, fc.K8sclient, &svc, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("fe controller sync apply external service name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			svc.Name, svc.Namespace, cluster.Name, err.Error())
		return err
	}

	st := fc.buildFEStatefulSet(cluster)
	if !fc.PrepareReconcileResources(ctx, cluster, v1.Component_FE) {
		klog.Infof("fe controller sync preparing resource for reconciling namespace %s name %s!", cluster.Namespace, cluster.Name)
		return nil
	}

	// fe cluster operator
	if err2 := fc.operator(ctx, st, *cluster); err2 != nil {
		return err
	}

	if err = k8s.ApplyStatefulSet(ctx, fc.K8sclient, &st, func(new *appv1.StatefulSet, old *appv1.StatefulSet) bool {
		fc.RestrictConditionsEqual(new, old)
		return resource.StatefulSetDeepEqual(new, old, false)
	}); err != nil {
		klog.Errorf("fe controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			st.Name, st.Namespace, cluster.Name, err.Error())
		return err
	}

	currentSituation, err := k8s.GetDorisClusterSituation(ctx, fc.K8sclient, cluster.Name, cluster.Namespace)
	if err != nil {
		klog.Errorf("after cluster operation GetDorisClusterSituation failed, err:%s ", err.Error())
	}

	if currentSituation.Situation != v1.SITUATION_INITIALIZING && currentSituation.Situation != "" {
		err = k8s.SetDorisClusterSituation(ctx, fc.K8sclient, cluster.Name, cluster.Namespace, v1.ClusterSituation{v1.SITUATION_OPERABLE, v1.RETRY_OPERATOR_NO})
		if err != nil {
			klog.Errorf("SetDorisClusterSituation 'OPERABLE' failed, err:%s ", err.Error())
		}
	}

	return nil
}

func (fc *Controller) operator(ctx context.Context, st appv1.StatefulSet, cluster v1.DorisCluster) error {
	var oldSt appv1.StatefulSet
	err := fc.K8sclient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &oldSt)
	situation, _ := k8s.GetDorisClusterSituation(ctx, fc.K8sclient, cluster.Name, cluster.Namespace)

	if err != nil || situation == nil || situation.Situation == v1.SITUATION_INITIALIZING {
		klog.Infof("skip cluster operation, cluster is in INITIALIZING situation")
		return nil
	}

	// update cluster not start cluster

	//klog.Errorf("new.Spec.Replicas: %d ", *(st.Spec.Replicas))
	//klog.Errorf("old.Spec.Replicas: %d ", *(oldSt.Spec.Replicas))
	//klog.Errorf("cluster.Spec.FeSpec.Replicas: %d ", *(cluster.Spec.FeSpec.Replicas))
	scaleNumber := *(cluster.Spec.FeSpec.Replicas) - *(oldSt.Spec.Replicas)
	//klog.Errorf("scaleNumber : %d ", scaleNumber)

	if situation.Situation != v1.SITUATION_OPERABLE && situation.Retry != v1.RETRY_OPERATOR_FE {
		// means other task running, send Event warning
		fc.K8srecorder.Eventf(
			&cluster, sub_controller.EventWarning,
			sub_controller.ClusterOperationalConflicts,
			"There is a conflict in crd operation. currently, cluster situation is %+v ", situation.Situation,
		)
		return errors.New(fmt.Sprintf("There is a conflict in crd operation. currently, cluster situation is %+v ", situation.Situation))
	}

	if situation.Situation == v1.SITUATION_OPERABLE || situation.Retry == v1.RETRY_OPERATOR_FE {

		// fe scale
		if scaleNumber != 0 { // set Situation as SCALING
			if err := k8s.SetDorisClusterSituation(ctx, fc.K8sclient, cluster.Name, cluster.Namespace,
				v1.ClusterSituation{
					Situation: v1.SITUATION_SCALING,
					Retry:     v1.RETRY_OPERATOR_FE, // must set Retry as RETRY_OPERATOR_FE for an error occurs, Retry will be reset as RETRY_OPERATOR_NO after a success.
				},
			); err != nil {
				klog.Errorf("SetDorisClusterSituation 'SCALING' failed err:%s ", err.Error())
				return err
			}
		}
		if scaleNumber < 0 {
			if err := fc.ScaleDownObserver(ctx, fc.K8sclient, &st, &cluster, -scaleNumber); err != nil {
				klog.Errorf("ScaleDownObserver failed, err:%s ", err.Error())
				return err
			}
		}

		//TODO check upgrade ,restart
	}

	return nil
}
