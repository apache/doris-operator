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

package broker

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils"
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
	config, err := bk.GetConfig(ctx, &brokerSpec.ConfigMapInfo, dcr.Namespace, v1.Component_Broker)
	if err != nil {
		klog.Error("BrokerController Sync ", "resolve broker configmap failed, namespace ", dcr.Namespace, " error ", err)
		return err
	}
	bk.CheckConfigMountPath(dcr, v1.Component_Broker)
	internalService := resource.BuildInternalService(dcr, v1.Component_Broker, config)
	if err := k8s.ApplyService(ctx, bk.K8sclient, &internalService, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("broker controller sync apply internalService name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			internalService.Name, internalService.Namespace, dcr.Name, err.Error())
		return err
	}

	st := bk.buildBKStatefulSet(dcr)
	if err = k8s.ApplyStatefulSet(ctx, bk.K8sclient, &st, func(new *appv1.StatefulSet, est *appv1.StatefulSet) bool {
		// if have restart annotation, we should exclude the interference for comparison.
		return resource.StatefulSetDeepEqual(new, est, false)
	}); err != nil {
		klog.Errorf("broker controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			st.Name, st.Namespace, dcr.Name, err.Error())
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

func (bk *Controller) getFeConfig(ctx context.Context, feconfigMapInfo *v1.ConfigMapInfo, namespace string) (map[string]interface{}, error) {
	cms := resource.GetMountConfigMapInfo(*feconfigMapInfo)
	if len(cms) == 0 {
		return make(map[string]interface{}), nil
	}
	feconfigMaps, err := k8s.GetConfigMaps(ctx, bk.K8sclient, namespace, cms)
	if err != nil {
		klog.Errorf("BrokerController getFeConfig fe config failed, namespace: %s,err: %s \n", namespace, err.Error())
	}
	res, resolveErr := resource.ResolveConfigMaps(feconfigMaps, v1.Component_FE)

	return res, utils.MergeError(err, resolveErr)
}
