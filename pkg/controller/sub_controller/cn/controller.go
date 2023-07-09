package cn

import (
	"context"
	dorisv1 "github.com/selectdb/doris-operator/api/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type Controller struct {
	sub_controller.SubDefaultController
}

const (
	CN_SEARCH_SUFFIX = "-search"
)

func (cn *Controller) GetControllerName() string {
	return "cnController"
}
func New(k8sclient client.Client, k8srecorder record.EventRecorder) *Controller {
	return &Controller{
		SubDefaultController: sub_controller.SubDefaultController{
			K8sclient:   k8sclient,
			K8srecorder: k8srecorder,
		},
	}
}

func (cn *Controller) Sync(ctx context.Context, dcr *dorisv1.DorisCluster) error {
	if dcr.Spec.CnSpec == nil {
		if _, err := cn.ClearResources(ctx, dcr); err != nil {
			klog.Errorf("cn controller sync clearResource  namespace=%s,srcName=%s, err=%s\n", dcr.Namespace, dcr.Name, err.Error())
			return err
		}
		return nil
	}
	cnSpec := dcr.Spec.CnSpec

	config, err := cn.GetConfig(ctx, &cnSpec.ConfigMapInfo, dcr.Namespace)
	if err != nil {
		klog.Errorf("cn controller sync",
			"resolve cn configMap failed, namespace ", dcr.Namespace,
			"configMap", dcr.Spec.CnSpec.ConfigMapInfo.ConfigMapName)
		return err
	}
	feconfig, _ := cn.getFeConfig(ctx, &dcr.Spec.FeSpec.ConfigMapInfo, dcr.Namespace)
	config[resource.QUERY_PORT] = strconv.FormatInt(int64(resource.GetPort(feconfig, resource.QUERY_PORT)), 10)

	svc := resource.BuildExternalService(dcr, dorisv1.Component_CN, config)

	internalSVC := resource.BuildInternalService(dcr, dorisv1.Component_CN, config)

	if err := k8s.ApplyService(ctx, cn.K8sclient, &internalSVC, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("cn controller sync apply internalService name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			internalSVC.Name, internalSVC.Namespace, dcr.Name, err.Error())
		return err
	}

	if err := k8s.ApplyService(ctx, cn.K8sclient, &svc, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("cn controller sync apply externalService name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			svc.Name, svc.Namespace, dcr.Name, err.Error())
		return err
	}
	cnStatefulSet := cn.buildCnStatefulSet(dcr)
	if err = k8s.ApplyStatefulSet(ctx, cn.K8sclient, &cnStatefulSet, func(new *appv1.StatefulSet, est *appv1.StatefulSet) bool {
		// if have restart annotation, we should exclude the interference for comparison.
		return resource.StatefulSetDeepEqual(new, est, false)
	}); err != nil {
		klog.Errorf("cn controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			cnStatefulSet.Name, cnStatefulSet.Namespace)
		return err
	}
	return nil

}

func (cn *Controller) ClearResource(ctx context.Context, dcr *dorisv1.DorisCluster) (bool, error) {
	cnStatus := dcr.Status.CnStatus
	if cnStatus == nil {
		klog.Info("Doris cluster is not have cn")
		return true, nil
	}

	if dcr.DeletionTimestamp.IsZero() {
		return true, nil
	}

	cnStName := dorisv1.GenerateComponentStatefulSetName(dcr, dorisv1.Component_CN)
	externalServiceName := dorisv1.GenerateExternalServiceName(dcr, dorisv1.Component_CN)
	internalServiceName := dorisv1.GenerateInternalCommunicateServiceName(dcr, dorisv1.Component_CN)
	if err := k8s.DeleteStatefulset(ctx, cn.K8sclient, dcr.Namespace, cnStName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("cnController ClearResources delete statefulset failed, namespace=%s,name=%s, error=%s.", dcr.Namespace, cnStName, err.Error())
		return false, err
	}

	if err := k8s.DeleteService(ctx, cn.K8sclient, dcr.Namespace, internalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("cnController ClearResources delete search service, namespace=%s,name=%s,error=%s.", dcr.Namespace, internalServiceName, err.Error())
		return false, err
	}
	if err := k8s.DeleteService(ctx, cn.K8sclient, dcr.Namespace, externalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("cnController ClearResources delete external service, namespace=%s, name=%s,error=%s.", dcr.Namespace, externalServiceName, err.Error())
		return false, err
	}

	return true, nil
}

func (cn *Controller) getFeConfig(ctx context.Context, feconfigMapInfo *dorisv1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	feconfigMap, err := k8s.GetConfigMap(ctx, cn.K8sclient, namespace, feconfigMapInfo.ConfigMapName)
	if err != nil && apierrors.IsNotFound(err) {
		klog.V(4).Info("cn controller get fe config is not exists namespace ", namespace, " configmapName ", feconfigMapInfo.ConfigMapName)
		return make(map[string]interface{}), nil
	} else if err != nil {
		return make(map[string]interface{}), err
	}
	res, err := resource.ResolveConfigMap(feconfigMap, feconfigMapInfo.ResolveKey)
	return res, err
}

func (cn *Controller) GetConfig(ctx context.Context, configMapInfo *dorisv1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	configMap, err := k8s.GetConfigMap(ctx, cn.K8sclient, namespace, configMapInfo.ConfigMapName)
	if err != nil && apierrors.IsNotFound(err) {
		klog.Info("cnController GetCnConfig config is not exist namespace ", namespace, " configmapName ", configMapInfo.ConfigMapName)
		return make(map[string]interface{}), nil
	} else if err != nil {
		return make(map[string]interface{}), err
	}
	res, err := resource.ResolveConfigMap(configMap, configMapInfo.ResolveKey)
	return res, err
}
