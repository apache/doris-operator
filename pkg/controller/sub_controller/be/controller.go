package be

import (
	"context"
	dorisv1 "github.com/selectdb/doris-operator/api/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (be *Controller) Sync(ctx context.Context, dcr *dorisv1.DorisCluster) error {
	if dcr.Spec.BeSpec == nil {
		if _, err := be.ClearResources(ctx, dcr); err != nil {
			klog.Errorf("beController sync clearResource namespace=%s,srcName=%s, err=%s\n", dcr.Namespace, dcr.Name, err.Error())
			return err
		}

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
	svc := resource.BuildExternalService(dcr, dorisv1.Component_BE, config)
	//create or update be external and domain search service, update the status of fe on src.
	internalService := resource.BuildInternalService(dcr, dorisv1.Component_BE, config)
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

	st := be.buildFEStatefulSet(dcr)
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

func (be *Controller) UpdateComponentStatus(cluster *dorisv1.DorisCluster) error {
	//if spec is not exist, status is empty. but before clear status we must clear all resource about be used by ClearResources.
	if cluster.Spec.BeSpec == nil {
		cluster.Status.BEStatus = nil
		return nil
	}

	bs := &dorisv1.ComponentStatus{
		ComponentCondition: dorisv1.ComponentCondition{
			Phase:              dorisv1.Reconciling,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}

	if cluster.Status.BEStatus != nil {
		bs = cluster.Status.BEStatus.DeepCopy()
	}
	cluster.Status.BEStatus = bs
	bs.AccessService = dorisv1.GenerateExternalServiceName(cluster, dorisv1.Component_BE)
	return be.UpdateStatus(cluster.Namespace, bs, dorisv1.GenerateStatefulSetSelector(cluster, dorisv1.Component_BE), *cluster.Spec.BeSpec.Replicas)
}

func (be *Controller) ClearResources(ctx context.Context, dcr *dorisv1.DorisCluster) (bool, error) {
	//if the doris is not have cn.
	beStName := dorisv1.GenerateComponentStatefulSetName(dcr, dorisv1.Component_BE)
	externalServiceName := dorisv1.GenerateExternalServiceName(dcr, dorisv1.Component_BE)
	internalServiceName := dorisv1.GenerateInternalCommunicateServiceName(dcr, dorisv1.Component_BE)
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

func (be *Controller) getFeConfig(ctx context.Context, feconfigMapInfo *dorisv1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
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

func (be *Controller) GetConfig(ctx context.Context, configMapInfo *dorisv1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
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
