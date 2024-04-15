package be

import (
	"context"
	"github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Controller struct {
	sub_controller.SubDefaultController
}

const (
	BE_SEARCH_SUFFIX = "-search"
)

func New(k8sclient client.Client, k8srecorder record.EventRecorder) *Controller {
	return &Controller{
		SubDefaultController: sub_controller.SubDefaultController{
			K8sclient:   k8sclient,
			K8srecorder: k8srecorder,
		},
	}
}

func (be *Controller) GetControllerName() string {
	return "beController"
}

func (be *Controller) Sync(ctx context.Context, dcr *v1.DorisCluster) error {
	if dcr.Spec.BeSpec == nil {
		return nil
	}

	if !be.FeAvailable(dcr) {
		return nil
	}
	beSpec := dcr.Spec.BeSpec
	//get the be configMap for resolve ports.
	//2. get config for generate statefulset and service.
	config, err := be.GetConfig(ctx, &beSpec.ConfigMapInfo, dcr.Namespace, v1.Component_BE)
	if err != nil {
		klog.Error("BeController Sync ", "resolve be configmap failed, namespace ", dcr.Namespace, " error :", err)
		return err
	}

	be.CheckConfigMountPath(dcr, v1.Component_BE)
	//generate new be service.
	svc := resource.BuildExternalService(dcr, v1.Component_BE, config)
	//create or update be external and domain search service, update the status of fe on src.
	internalService := resource.BuildInternalService(dcr, v1.Component_BE, config)
	if err := k8s.ApplyService(ctx, be.K8sclient, &internalService, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("be controller sync apply internalService name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			internalService.Name, internalService.Namespace, dcr.Name, err.Error())
		return err
	}
	if err := k8s.ApplyService(ctx, be.K8sclient, &svc, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("be controller sync apply external service name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			svc.Name, svc.Namespace, dcr.Name, err.Error())
		return err
	}

	st := be.buildBEStatefulSet(dcr)
	if !be.PrepareReconcileResources(ctx, dcr, v1.Component_BE) {
		klog.Infof("be controller sync preparing resource for reconciling namespace %s name %s!", dcr.Namespace, dcr.Name)
		return nil
	}

	if err = k8s.ApplyStatefulSet(ctx, be.K8sclient, &st, func(new *appv1.StatefulSet, est *appv1.StatefulSet) bool {
		// if have restart annotation, we should exclude the interference for comparison.
		be.RestrictConditionsEqual(new, est)
		return resource.StatefulSetDeepEqual(new, est, false)
	}); err != nil {
		klog.Errorf("fe controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			st.Name, st.Namespace, dcr.Name, err.Error())
		return err
	}

	return nil
}

func (be *Controller) UpdateComponentStatus(cluster *v1.DorisCluster) error {
	//if spec is not exist, status is empty. but before clear status we must clear all resource about be.
	if cluster.Spec.BeSpec == nil {
		cluster.Status.BEStatus = nil
		return nil
	}

	bs := &v1.ComponentStatus{
		ComponentCondition: v1.ComponentCondition{
			SubResourceName:    v1.GenerateComponentStatefulSetName(cluster, v1.Component_BE),
			Phase:              v1.Reconciling,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}

	if cluster.Status.BEStatus != nil {
		bs = cluster.Status.BEStatus.DeepCopy()
	}
	cluster.Status.BEStatus = bs
	bs.AccessService = v1.GenerateExternalServiceName(cluster, v1.Component_BE)
	return be.ClassifyPodsByStatus(cluster.Namespace, bs, v1.GenerateStatefulSetSelector(cluster, v1.Component_BE), *cluster.Spec.BeSpec.Replicas)
}

func (be *Controller) ClearResources(ctx context.Context, dcr *v1.DorisCluster) (bool, error) {
	//if the doris is not have be.
	if dcr.Status.BEStatus == nil {
		return true, nil
	}

	if dcr.Spec.BeSpec == nil {
		return be.ClearCommonResources(ctx, dcr, v1.Component_BE)
	}

	return true, nil
}
