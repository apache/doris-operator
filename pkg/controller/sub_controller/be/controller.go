package be

import (
	"context"
	"github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
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
		if _, err := be.ClearResources(ctx, dcr); err != nil {
			klog.Errorf("beController sync clearResource namespace=%s,srcName=%s, err=%s\n", dcr.Namespace, dcr.Name, err.Error())
			return err
		}

		return nil
	}

	//TODO:  check fe available
	if !be.feAvailable(dcr) {
		return nil
	}
	beSpec := dcr.Spec.BeSpec
	//get the be configMap for resolve ports.
	//2. get config for generate statefulset and service.
	config, err := be.GetConfig(ctx, &beSpec.ConfigMapInfo, dcr.Namespace)
	if err != nil {
		klog.Error("BeController Sync ", "resolve cn configmap failed, namespace ", dcr.Namespace, " configmapName ", beSpec.ConfigMapInfo.ConfigMapName, " configMapKey ", beSpec.ConfigMapInfo.ResolveKey, " error ", err)
		return err
	}

	feconfig, _ := be.getFeConfig(ctx, &dcr.Spec.BeSpec.ConfigMapInfo, dcr.Namespace)
	//annotation: add query port in cnconfig.
	config[resource.QUERY_PORT] = strconv.FormatInt(int64(resource.GetPort(feconfig, resource.QUERY_PORT)), 10)
	//generate new cn external service.

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
	if err = k8s.ApplyStatefulSet(ctx, be.K8sclient, &st, func(new *appv1.StatefulSet, est *appv1.StatefulSet) bool {
		// if have restart annotation, we should exclude the interference for comparison.
		return resource.StatefulSetDeepEqual(new, est, false)
	}); err != nil {
		klog.Errorf("fe controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			st.Name, st.Namespace)
		return err
	}

	return nil
}

func (be *Controller) feAvailable(dcr *v1.DorisCluster) bool {
	addr, _ := v1.GetConfigFEAddrForAccess(dcr, v1.Component_BE)
	if addr != "" {
		return true
	}

	//if fe deploy in k8s, should wait fe available
	//1. wait for fe ok.
	endpoints := corev1.Endpoints{}
	if err := be.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: dcr.Namespace, Name: v1.GenerateExternalServiceName(dcr, v1.Component_FE)}, &endpoints); err != nil {
		klog.Infof("BeController Sync wait fe service name %s available occur failed %s\n", v1.GenerateExternalServiceName(dcr, v1.Component_FE), err.Error())
		return false
	}

	for _, sub := range endpoints.Subsets {
		if len(sub.Addresses) > 0 {
			return true
		}
	}
	return false
}

func (be *Controller) UpdateComponentStatus(cluster *v1.DorisCluster) error {
	//if spec is not exist, status is empty. but before clear status we must clear all resource about be used by ClearResources.
	if cluster.Spec.BeSpec == nil {
		cluster.Status.BEStatus = nil
		return nil
	}

	bs := &v1.ComponentStatus{
		ComponentCondition: v1.ComponentCondition{
			SubResourceName:    v1.GenerateComponentStatefulSetName(cluster, v1.Component_FE),
			Phase:              v1.Reconciling,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}

	if cluster.Status.BEStatus != nil {
		bs = cluster.Status.BEStatus.DeepCopy()
	}
	cluster.Status.BEStatus = bs
	bs.AccessService = v1.GenerateExternalServiceName(cluster, v1.Component_BE)
	return be.UpdateStatus(cluster.Namespace, bs, v1.GenerateStatefulSetSelector(cluster, v1.Component_BE), *cluster.Spec.BeSpec.Replicas)
}

func (be *Controller) ClearResources(ctx context.Context, dcr *v1.DorisCluster) (bool, error) {
	//if the doris is not have be.
	if dcr.Status.BEStatus == nil {
		return true, nil
	}

	if dcr.DeletionTimestamp.IsZero() {
		return true, nil
	}

	//if the doris is not have cn.
	beStName := v1.GenerateComponentStatefulSetName(dcr, v1.Component_BE)
	externalServiceName := v1.GenerateExternalServiceName(dcr, v1.Component_BE)
	internalServiceName := v1.GenerateInternalCommunicateServiceName(dcr, v1.Component_BE)
	if err := k8s.DeleteStatefulset(ctx, be.K8sclient, dcr.Namespace, beStName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("beController ClearResources delete statefulset failed, namespace=%s,name=%s, error=%s.", dcr.Namespace, beStName, err.Error())
		return false, err
	}

	if err := k8s.DeleteService(ctx, be.K8sclient, dcr.Namespace, internalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("feController ClearResources delete search service, namespace=%s,name=%s,error=%s.", dcr.Namespace, internalServiceName, err.Error())
		return false, err
	}
	if err := k8s.DeleteService(ctx, be.K8sclient, dcr.Namespace, externalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("feController ClearResources delete external service, namespace=%s, name=%s,error=%s.", dcr.Namespace, externalServiceName, err.Error())
		return false, err
	}

	return true, nil
}

func (be *Controller) getFeConfig(ctx context.Context, feconfigMapInfo *v1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	if feconfigMapInfo.ConfigMapName == "" {
		return make(map[string]interface{}), nil
	}

	feconfigMap, err := k8s.GetConfigMap(ctx, be.K8sclient, namespace, feconfigMapInfo.ConfigMapName)
	if err != nil && apierrors.IsNotFound(err) {
		klog.Info("BeController getFeConfig fe config not exist namespace ", namespace, " configmapName ", feconfigMapInfo.ConfigMapName)
		return make(map[string]interface{}), nil
	} else if err != nil {
		return make(map[string]interface{}), err
	}
	res, err := resource.ResolveConfigMap(feconfigMap, feconfigMapInfo.ResolveKey)
	return res, err
}

func (be *Controller) GetConfig(ctx context.Context, configMapInfo *v1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	configMap, err := k8s.GetConfigMap(ctx, be.K8sclient, namespace, configMapInfo.ConfigMapName)
	if err != nil && apierrors.IsNotFound(err) {
		klog.Info("BeController GetCnConfig config is not exist namespace ", namespace, " configmapName ", configMapInfo.ConfigMapName)
		return make(map[string]interface{}), nil
	} else if err != nil {
		return make(map[string]interface{}), err
	}

	res, err := resource.ResolveConfigMap(configMap, configMapInfo.ResolveKey)
	return res, err
}
