// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package cn

import (
	"context"
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	if !cn.FeAvailable(dcr) {
		return nil
	}

	cnSpec := dcr.Spec.CnSpec

	config, err := cn.GetConfig(ctx, &cnSpec.ConfigMapInfo, dcr.Namespace)
	if err != nil {
		klog.Errorf("cn controller sync resolve cn configMap failed, namespace %s ，err :", dcr.Namespace, err)
		return err
	}
	cn.CheckConfigMountPath(dcr, dorisv1.Component_CN)
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
	if !cn.PrepareReconcileResources(ctx, dcr, dorisv1.Component_CN) {
		klog.Infof("cn controller sync preparing resource for reconciling namespace %s name %s!", dcr.Namespace, dcr.Name)
		return nil
	}

	if err = cn.applyStatefulSet(ctx, &cnStatefulSet, cnSpec.AutoScalingPolicy != nil); err != nil {
		klog.Errorf("cn controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			cnStatefulSet.Name, cnStatefulSet.Namespace)
		return err
	}

	//create autoscaler.
	if cnSpec.AutoScalingPolicy != nil {
		err = cn.deployAutoScaler(ctx, *cnSpec.AutoScalingPolicy, &cnStatefulSet, dcr)
	}

	return nil
}

func (cn *Controller) UpdateComponentStatus(cluster *dorisv1.DorisCluster) error {
	// if spec is not exit, status is empty. but before clear status we must clear all resource about cn.
	if cluster.Spec.CnSpec == nil {
		cluster.Status.CnStatus = nil
		return nil
	}

	cs := &dorisv1.CnStatus{
		ComponentStatus: dorisv1.ComponentStatus{
			ComponentCondition: dorisv1.ComponentCondition{
				SubResourceName: dorisv1.GenerateComponentStatefulSetName(cluster, dorisv1.Component_CN),
				Phase:           dorisv1.Reconciling,

				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		},
	}

	if cluster.Spec.CnSpec.AutoScalingPolicy != nil {
		cs.HorizontalScaler = &dorisv1.HorizontalScaler{
			Version: cluster.Spec.CnSpec.AutoScalingPolicy.Version,
			Name:    cn.generateAutoScalerName(cluster),
		}
	}

	cluster.Status.CnStatus = cs

	// start autoscaler, the replicas should get from statefulset, statefulset's replicas will update by autoscaler when not set.
	var est appv1.StatefulSet
	if err := cn.K8sclient.Get(context.Background(), types.NamespacedName{Namespace: cluster.Namespace, Name: dorisv1.GenerateComponentStatefulSetName(cluster, dorisv1.Component_CN)}, &est); err != nil {
		cn.K8srecorder.Eventf(cluster, string(sub_controller.EventWarning), sub_controller.StatefulSetNotExist, "the cn statefulset %s not exist.", dorisv1.GenerateComponentStatefulSetName(cluster, dorisv1.Component_CN))
		return nil
	}

	replicas := *est.Spec.Replicas
	cs.AccessService = dorisv1.GenerateExternalServiceName(cluster, dorisv1.Component_CN)
	return cn.ClassifyPodsByStatus(cluster.Namespace, &cs.ComponentStatus, dorisv1.GenerateStatefulSetSelector(cluster, dorisv1.Component_CN), replicas)
}

// autoscaler represents start autoscaler or not.
func (cn *Controller) applyStatefulSet(ctx context.Context, st *appv1.StatefulSet, autoscaler bool) error {
	//create or update the status. create statefulset return, must ensure the
	var est appv1.StatefulSet
	if err := cn.K8sclient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est); apierrors.IsNotFound(err) {
		return k8s.CreateClientObject(ctx, cn.K8sclient, st)
	} else if err != nil {
		klog.Errorf("CnController Sync create statefulset name=%s, namespace=%s error=%s", st.Name, st.Namespace, err.Error())
		return err
	}
	//if the spec is changed, update the status of cn on src.
	var excludeReplica bool
	//if replicas =0 and not the first time, exclude the hash for autoscaler
	if st.Spec.Replicas == nil && !autoscaler {
		excludeReplica = true
	}

	//the statefulset equal should exclude pvc. pvc not allowed update when use statefulset manage, when use `operator` mode for management that pvc not allow updated in statetfulset spec.
	cn.RestrictConditionsEqual(st, &est)
	if !resource.StatefulSetDeepEqual(st, &est, excludeReplica) {
		//if the replicas not zero, represent user have cancel autoscaler.
		if st.Spec.Replicas != nil {
			resource.MergeStatefulSets(st, est)
			return k8s.UpdateClientObject(ctx, cn.K8sclient, st)
		}

		st.ResourceVersion = est.ResourceVersion
		return k8s.UpdateClientObject(ctx, cn.K8sclient, st)
	}

	return nil
}

func (cn *Controller) deleteAutoScaler(ctx context.Context, dcr *dorisv1.DorisCluster) error {
	if dcr.Status.CnStatus == nil {
		return nil
	}

	if dcr.Status.CnStatus.HorizontalScaler.Name == "" {
		klog.V(4).Infof("cnController not need delete the autoScaler, namespace=%s, src name=%s.", dcr.Namespace, dcr.Name)
		return nil
	}

	autoScalerName := dcr.Status.CnStatus.HorizontalScaler.Name
	version := dcr.Status.CnStatus.HorizontalScaler.Version
	if err := k8s.DeleteAutoscaler(ctx, cn.K8sclient, dcr.Namespace, autoScalerName, version); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("cnController sync deploy or delete failed, namespace=%s, autosclaer name=%s, autoscaler version=%s", dcr.GetNamespace(), autoScalerName, version)
		return err
	}

	dcr.Status.CnStatus.HorizontalScaler = nil
	return nil
}

func (cn *Controller) deployAutoScaler(ctx context.Context, policy dorisv1.AutoScalingPolicy, target *appv1.StatefulSet, dcr *dorisv1.DorisCluster) error {
	params := cn.buildCnAutoscalerParams(policy, target, dcr)
	autoScaler := resource.BuildHorizontalPodAutoscaler(params)
	if err := k8s.CreateOrUpdateClientObject(ctx, cn.K8sclient, autoScaler); err != nil {
		klog.Errorf("cnController deployAutoscaler failed, namespace=%s,name=%s,version=%s,error=%s", autoScaler.GetNamespace(), autoScaler.GetName(), policy.Version, err.Error())
		return err
	}

	return nil
}

func (cn *Controller) ClearResources(ctx context.Context, dcr *dorisv1.DorisCluster) (bool, error) {
	cnStatus := dcr.Status.CnStatus
	if cnStatus == nil {
		klog.Info("Doris cluster is not have cn")
		return true, nil
	}

	// clear autoscaler when autoscaler config deleted or the doriscluster deleted.
	if dcr.Spec.CnSpec.AutoScalingPolicy == nil || !dcr.DeletionTimestamp.IsZero() {
		if err := cn.DeleteAutoscaler(ctx, dcr); err != nil {
			cn.K8srecorder.Eventf(dcr, string(sub_controller.EventWarning), sub_controller.AutoScalerDeleteFailed, "cn autoscaler deleted failed."+err.Error())
		}
	}

	if dcr.Spec.CnSpec == nil {
		cn.ClearCommonResources(ctx, dcr, dorisv1.Component_CN)
	}

	return true, nil
}

func (cn *Controller) DeleteAutoscaler(ctx context.Context, dcr *dorisv1.DorisCluster) error {
	if dcr.Status.CnStatus == nil || dcr.Status.CnStatus.HorizontalScaler == nil {
		return nil
	}

	autoScalerName := dcr.Status.CnStatus.HorizontalScaler.Name
	version := dcr.Status.CnStatus.HorizontalScaler.Version
	if err := k8s.DeleteAutoscaler(ctx, cn.K8sclient, dcr.Namespace, autoScalerName, version); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("cnController delete failed, namespace=%s, autosclaer name=%s, autoscaler version=%s", dcr.GetNamespace(), autoScalerName, version)
		return err
	}

	dcr.Status.CnStatus.HorizontalScaler = &dorisv1.HorizontalScaler{}
	return nil
}

func (cn *Controller) GetConfig(ctx context.Context, configMapInfo *dorisv1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	cms := resource.GetMountConfigMapInfo(*configMapInfo)
	if len(cms) == 0 {
		return make(map[string]interface{}), nil
	}
	configMaps, err := k8s.GetConfigMaps(ctx, cn.K8sclient, namespace, cms)
	if err != nil {
		klog.Errorf("CnController GetConfig get configmap failed, namespace: %s, err: %s \n", namespace, err.Error())
	}
	res, resolveErr := resource.ResolveConfigMaps(configMaps, dorisv1.Component_CN)
	return res, utils.MergeError(err, resolveErr)
}

func (cn *Controller) getFeConfig(ctx context.Context, configMapInfo *dorisv1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	cms := resource.GetMountConfigMapInfo(*configMapInfo)
	if len(cms) == 0 {
		return make(map[string]interface{}), nil
	}
	configMaps, err := k8s.GetConfigMaps(ctx, cn.K8sclient, namespace, cms)
	if err != nil {
		klog.Errorf("CnController GetFeConfig get configmap failed, namespace: %s, err: %s \n", namespace, err.Error())
	}
	res, resolveErr := resource.ResolveConfigMaps(configMaps, dorisv1.Component_FE)
	return res, utils.MergeError(err, resolveErr)
}
