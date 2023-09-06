package broker

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	BROKER_SEARCH_SUFFIX = "-search"
)

func New(k8sclient client.Client, k8srecorder record.EventRecorder) *Controller {
	return &Controller{
		SubDefaultController: sub_controller.SubDefaultController{
			K8sclient:   k8sclient,
			K8srecorder: k8srecorder,
		},
	}
}

func (bk *Controller) GetControllerName() string {
	return "brokerController"
}

func (bk *Controller) Sync(ctx context.Context, dcr *v1.DorisCluster) error {

	if dcr.Spec.BrokerSpec == nil {
		return nil
	}

	if !bk.FeAvailable(dcr) {
		return nil
	}
	brokerSpec := dcr.Spec.BrokerSpec
	//get the broker configMap for resolve ports.
	//2. get config for generate statefulset and service.
	config, err := bk.GetConfig(ctx, &brokerSpec.ConfigMapInfo, dcr.Namespace)
	if err != nil {
		klog.Error("BrokerController Sync ", "resolve cn configmap failed, namespace ", dcr.Namespace, " configmapName ", brokerSpec.ConfigMapInfo.ConfigMapName, " configMapKey ", brokerSpec.ConfigMapInfo.ResolveKey, " error ", err)
		return err
	}

	//generate new broker service.
	svc := resource.BuildExternalService(dcr, v1.Component_Broker, config)
	//create or update bk external and domain search service, update the status of fe on src.
	internalService := resource.BuildInternalService(dcr, v1.Component_Broker, config)
	if err := k8s.ApplyService(ctx, bk.K8sclient, &internalService, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("broker controller sync apply internalService name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			internalService.Name, internalService.Namespace, dcr.Name, err.Error())
		return err
	}
	if err := k8s.ApplyService(ctx, bk.K8sclient, &svc, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("broker controller sync apply external service name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			svc.Name, svc.Namespace, dcr.Name, err.Error())
		return err
	}

	st := bk.buildBKStatefulSet(dcr)
	if err = k8s.ApplyStatefulSet(ctx, bk.K8sclient, &st, func(new *appv1.StatefulSet, est *appv1.StatefulSet) bool {
		// if have restart annotation, we should exclude the interference for comparison.
		return resource.StatefulSetDeepEqual(new, est, false)
	}); err != nil {
		klog.Errorf("broker controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			st.Name, st.Namespace)
		return err
	}

	return nil
}

func (bk *Controller) UpdateComponentStatus(cluster *v1.DorisCluster) error {

	if cluster.Spec.BrokerSpec == nil {
		cluster.Status.BrokerStatus = nil
		return nil
	}

	bs := &v1.ComponentStatus{
		ComponentCondition: v1.ComponentCondition{
			SubResourceName:    v1.GenerateComponentStatefulSetName(cluster, v1.Component_Broker),
			Phase:              v1.Reconciling,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}

	if cluster.Status.BrokerStatus != nil {
		bs = cluster.Status.BrokerStatus.DeepCopy()
	}

	cluster.Status.BrokerStatus = bs
	bs.AccessService = v1.GenerateExternalServiceName(cluster, v1.Component_Broker)
	return bk.ClassifyPodsByStatus(cluster.Namespace, bs, v1.GenerateStatefulSetSelector(cluster, v1.Component_Broker), *cluster.Spec.BrokerSpec.Replicas)

}

func (bk *Controller) ClearResources(ctx context.Context, dcr *v1.DorisCluster) (bool, error) {
	//if the doris is not have broker.
	if dcr.Status.BrokerStatus == nil {
		return true, nil
	}

	if dcr.Spec.BrokerSpec == nil {
		return bk.ClearCommonResources(ctx, dcr, v1.Component_Broker)
	}

	return true, nil
}

//func (bk *Controller) GetConfig(ctx context.Context, configMapInfo *v1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
//	configMap, err := k8s.GetConfigMap(ctx, bk.K8sclient, namespace, configMapInfo.ConfigMapName)
//	if err != nil && apierrors.IsNotFound(err) {
//		klog.Info("BrokerController GetCnConfig config is not exist namespace ", namespace, " configmapName ", configMapInfo.ConfigMapName)
//		return make(map[string]interface{}), nil
//	} else if err != nil {
//		return make(map[string]interface{}), err
//	}
//
//	res, err := resource.ResolveConfigMap(configMap, configMapInfo.ResolveKey)
//	return res, err
//}

func (bk *Controller) getFeConfig(ctx context.Context, feconfigMapInfo *v1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	if feconfigMapInfo.ConfigMapName == "" {
		return make(map[string]interface{}), nil
	}

	feconfigMap, err := k8s.GetConfigMap(ctx, bk.K8sclient, namespace, feconfigMapInfo.ConfigMapName)
	if err != nil && apierrors.IsNotFound(err) {
		klog.Info("BrokerController getFeConfig fe config not exist namespace ", namespace, " configmapName ", feconfigMapInfo.ConfigMapName)
		return make(map[string]interface{}), nil
	} else if err != nil {
		return make(map[string]interface{}), err
	}
	res, err := resource.ResolveConfigMap(feconfigMap, feconfigMapInfo.ResolveKey)
	return res, err
}

//func (bk *Controller) feAvailable(dcr *v1.DorisCluster) bool {
//	addr, _ := v1.GetConfigFEAddrForAccess(dcr, v1.Component_Broker)
//	if addr != "" {
//		return true
//	}
//
//	//if fe deploy in k8s, should wait fe available
//	//1. wait for fe ok.
//	endpoints := corev1.Endpoints{}
//	if err := bk.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: dcr.Namespace, Name: v1.GenerateExternalServiceName(dcr, v1.Component_FE)}, &endpoints); err != nil {
//		klog.Infof("BrokerController Sync wait fe service name %s available occur failed %s\n", v1.GenerateExternalServiceName(dcr, v1.Component_FE), err.Error())
//		return false
//	}
//
//	for _, sub := range endpoints.Subsets {
//		if len(sub.Addresses) > 0 {
//			return true
//		}
//	}
//	return false
//}
