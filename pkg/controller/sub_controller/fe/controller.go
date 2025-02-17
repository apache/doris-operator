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

package fe

import (
	"context"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	"github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller struct {
	sub_controller.SubDefaultController
}

func (fc *Controller) ClearResources(ctx context.Context, cluster *v1.DorisCluster) (bool, error) {
	//if the doris is not have fe.
	if cluster.Status.FEStatus == nil {
		return true, nil
	}
	if err := fc.RecycleResources(ctx, cluster, v1.Component_FE); err != nil {
		klog.Errorf("fe ClearResources recycle pvc resource for reconciling namespace %s name %s!", cluster.Namespace, cluster.Name)
		return false, err
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

	newCmHash := fc.BuildCoreConfigmapStatusHash(context.Background(), cluster, v1.Component_FE)
	cluster.Status.FEStatus.CoreConfigMapHashValue = newCmHash

	return fc.ClassifyPodsByStatus(cluster.Namespace, cluster.Status.FEStatus, v1.GenerateStatefulSetSelector(cluster, v1.Component_FE), *cluster.Spec.FeSpec.Replicas, v1.Component_FE)
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
	var oldStatus v1.ComponentStatus
	if cluster.Status.FEStatus != nil {
		oldStatus = *(cluster.Status.FEStatus.DeepCopy())
	}
	fc.InitStatus(cluster, v1.Component_FE)

	if cluster.Spec.EnableRestartWhenConfigChange {
		fc.CompareConfigmapAndTriggerRestart(cluster, oldStatus, v1.Component_FE)
	}

	feSpec := cluster.Spec.FeSpec
	//get the fe configMap for resolve ports.
	config, err := fc.GetConfig(ctx, &feSpec.BaseSpec.ConfigMapInfo, cluster.Namespace, v1.Component_FE)
	if err != nil {
		klog.Error("fe Controller Sync ", "resolve fe configmap failed, namespace ", cluster.Namespace, " error :", err)
		return err
	}
	fc.CheckConfigMountPath(cluster, v1.Component_FE)
	fc.CheckSecretMountPath(cluster, v1.Component_FE)
	fc.CheckSecretExist(ctx, cluster, v1.Component_FE)

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

	if !fc.PrepareReconcileResources(ctx, cluster, v1.Component_FE) {
		klog.Infof("fe controller sync preparing resource for reconciling namespace %s name %s!", cluster.Namespace, cluster.Name)
		return nil
	}

	if err = fc.prepareStatefulsetApply(ctx, cluster, oldStatus); err != nil {
		return err
	}

	st := fc.buildFEStatefulSet(cluster)
	if err = k8s.ApplyStatefulSet(ctx, fc.K8sclient, &st, func(new *appv1.StatefulSet, old *appv1.StatefulSet) bool {
		fc.RestrictConditionsEqual(new, old)
		return resource.StatefulSetDeepEqual(new, old, false)
	}); err != nil {
		klog.Errorf("fe controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			st.Name, st.Namespace, cluster.Name, err.Error())
		return err
	}

	return nil
}
