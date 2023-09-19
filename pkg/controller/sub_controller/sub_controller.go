package sub_controller

import (
	"context"
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SubController interface {
	//Sync reconcile for sub controller. bool represent the component have updated.
	Sync(ctx context.Context, cluster *dorisv1.DorisCluster) error
	//clear all resource about sub-component.
	ClearResources(ctx context.Context, cluster *dorisv1.DorisCluster) (bool, error)

	//return the controller name, beController, feController,cnController for log.
	GetControllerName() string

	//UpdateStatus update the component status on src.
	UpdateComponentStatus(cluster *dorisv1.DorisCluster) error
}

// SubDefaultController define common function for all component about doris.
type SubDefaultController struct {
	K8sclient   client.Client
	K8srecorder record.EventRecorder
}

// UpdateStatus update the component status on src.
func (d *SubDefaultController) UpdateStatus(namespace string, status *dorisv1.ComponentStatus, labels map[string]string, replicas int32) error {
	return d.ClassifyPodsByStatus(namespace, status, labels, replicas)
}

func (d *SubDefaultController) ClassifyPodsByStatus(namespace string, status *dorisv1.ComponentStatus, labels map[string]string, replicas int32) error {
	var podList corev1.PodList
	if err := d.K8sclient.List(context.Background(), &podList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return err
	}

	var creatings, readys, faileds []string
	podmap := make(map[string]corev1.Pod)
	//get all pod status that controlled by st.
	for _, pod := range podList.Items {
		podmap[pod.Name] = pod
		if ready := k8s.PodIsReady(&pod.Status); ready {
			readys = append(readys, pod.Name)
		} else if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			creatings = append(creatings, pod.Name)
		} else {
			faileds = append(faileds, pod.Name)
		}
	}

	status.ComponentCondition.Phase = dorisv1.Reconciling
	if len(readys) == int(replicas) {
		status.ComponentCondition.Phase = dorisv1.Available
	} else if len(faileds) != 0 {
		status.ComponentCondition.Phase = dorisv1.HaveMemberFailed
		status.ComponentCondition.Reason = podmap[faileds[0]].Status.Reason
		status.ComponentCondition.Message = podmap[faileds[0]].Status.Message
	} else if len(creatings) != 0 {
		status.ComponentCondition.Reason = podmap[creatings[0]].Status.Reason
		status.ComponentCondition.Message = podmap[creatings[0]].Status.Message
	}

	status.RunningMembers = readys
	status.FailedMembers = faileds
	status.CreatingMembers = creatings
	return nil
}

func (d *SubDefaultController) GetConfig(ctx context.Context, configMapInfo *dorisv1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	if configMapInfo.ConfigMapName == "" {
		return make(map[string]interface{}), nil
	}

	configMap, err := k8s.GetConfigMap(ctx, d.K8sclient, namespace, configMapInfo.ConfigMapName)
	if err != nil && apierrors.IsNotFound(err) {
		klog.Info("SubDefaultController GetCnConfig config is not exist namespace ", namespace, " configmapName ", configMapInfo.ConfigMapName)
		return make(map[string]interface{}), nil
	} else if err != nil {
		return make(map[string]interface{}), err
	}

	res, err := resource.ResolveConfigMap(configMap, configMapInfo.ResolveKey)
	return res, err
}

// ClearCommonResources clear common resources all component have, as statefulset, service.
// response `bool` represents all resource have deleted, if not and delete resource failed return false for next reconcile retry.
func (d *SubDefaultController) ClearCommonResources(ctx context.Context, dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) (bool, error) {
	//if the doris is not have cn.
	stName := dorisv1.GenerateComponentStatefulSetName(dcr, componentType)
	externalServiceName := dorisv1.GenerateExternalServiceName(dcr, componentType)
	internalServiceName := dorisv1.GenerateInternalCommunicateServiceName(dcr, componentType)
	if err := k8s.DeleteStatefulset(ctx, d.K8sclient, dcr.Namespace, stName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("SubDefaultController ClearResources delete statefulset failed, namespace=%s,name=%s, error=%s.", dcr.Namespace, stName, err.Error())
		return false, err
	}

	if err := k8s.DeleteService(ctx, d.K8sclient, dcr.Namespace, internalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("SubDefaultController ClearResources delete search service, namespace=%s,name=%s,error=%s.", dcr.Namespace, internalServiceName, err.Error())
		return false, err
	}
	if err := k8s.DeleteService(ctx, d.K8sclient, dcr.Namespace, externalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("SubDefaultController ClearResources delete external service, namespace=%s, name=%s,error=%s.", dcr.Namespace, externalServiceName, err.Error())
		return false, err
	}

	return true, nil
}

func (d *SubDefaultController) FeAvailable(dcr *dorisv1.DorisCluster) bool {
	addr, _ := dorisv1.GetConfigFEAddrForAccess(dcr, dorisv1.Component_BE)
	if addr != "" {
		return true
	}

	//if fe deploy in k8s, should wait fe available
	//1. wait for fe ok.
	endpoints := corev1.Endpoints{}
	if err := d.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: dcr.Namespace, Name: dorisv1.GenerateExternalServiceName(dcr, dorisv1.Component_FE)}, &endpoints); err != nil {
		klog.Infof("SubDefaultController Sync wait fe service name %s available occur failed %s\n", dorisv1.GenerateExternalServiceName(dcr, dorisv1.Component_FE), err.Error())
		return false
	}

	for _, sub := range endpoints.Subsets {
		if len(sub.Addresses) > 0 {
			return true
		}
	}
	return false
}
