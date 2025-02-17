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

package be

import (
	"context"
	"github.com/apache/doris-operator/api/doris/v1"
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

	var oldStatus v1.ComponentStatus
	if dcr.Status.BEStatus != nil {
		oldStatus = *(dcr.Status.BEStatus.DeepCopy())
	}
	be.InitStatus(dcr, v1.Component_BE)
	if !be.FeAvailable(dcr) {
		return nil
	}

	if dcr.Spec.EnableRestartWhenConfigChange {
		be.CompareConfigmapAndTriggerRestart(dcr, oldStatus, v1.Component_BE)
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
	be.CheckSecretMountPath(dcr, v1.Component_BE)
	be.CheckSecretExist(ctx, dcr, v1.Component_BE)
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

	if err = be.prepareStatefulsetApply(dcr, oldStatus); err != nil {
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

	newCmHash := be.BuildCoreConfigmapStatusHash(context.Background(), cluster, v1.Component_BE)
	cluster.Status.BEStatus.CoreConfigMapHashValue = newCmHash

	return be.ClassifyPodsByStatus(cluster.Namespace, cluster.Status.BEStatus, v1.GenerateStatefulSetSelector(cluster, v1.Component_BE), *cluster.Spec.BeSpec.Replicas, v1.Component_BE)
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
