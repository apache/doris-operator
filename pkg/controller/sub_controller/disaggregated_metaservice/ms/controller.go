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

package ms

import (
	"context"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller struct {
	sub_controller.DisaggregatedSubDefaultController
}

var (
	disaggregatedMSController = "disaggregatedMSController"
)

func New(mgr ctrl.Manager) *Controller {
	return &Controller{
		sub_controller.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(disaggregatedMSController),
			ControllerName: disaggregatedMSController,
		}}
}

func (msc *Controller) Sync(ctx context.Context, obj client.Object) error {
	dms := obj.(*mv1.DorisDisaggregatedMetaService)

	if dms.Status.FDBStatus.AvailableStatus != mv1.Available {
		klog.Info("MS Controller Sync: the FDB is UnAvailable namespace ", dms.Namespace, " disaggregated doris cluster name ", dms.Name)
		return nil
	}

	msc.initMSStatus(dms)
	msSpec := dms.Spec.MS

	config, err := msc.GetMSConfig(ctx, msSpec.ConfigMaps, dms.Namespace, mv1.Component_MS)
	if err != nil {
		klog.Error("MS Controller Sync ", "resolve ms configmap failed, namespace ", dms.Namespace, " error :", err)
		return err
	}

	msc.CheckMSConfigMountPath(dms, mv1.Component_MS)

	// MS only Build Internal Service
	service := resource.BuildDMSService(dms, mv1.Component_MS, resource.GetPort(config, resource.BRPC_LISTEN_PORT))
	if err = k8s.ApplyService(ctx, msc.K8sclient, &service, resource.DMSServiceDeepEqual); err != nil {
		klog.Errorf("MS controller sync apply service name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			service.Name, service.Namespace, dms.Name, err.Error())
		return err
	}

	// TODO prepareStatefulsetApply for scaling
	st := msc.buildMSStatefulSet(dms)

	if err = k8s.ApplyStatefulSet(ctx, msc.K8sclient, &st, func(new *appv1.StatefulSet, old *appv1.StatefulSet) bool {
		msc.RestrictConditionsEqual(new, old)
		return resource.DMSStatefulSetDeepEqual(new, old, false)
	}); err != nil {
		klog.Errorf("MS controller sync statefulset name=%s, namespace=%s, disaggregated-metaservice-name=%s failed. message=%s.",
			st.Name, st.Namespace, dms.Name, err.Error())
		return err
	}

	return nil
}

// ClearResources clear resources for MS
// clear deleted statefulset Resources When CR is marked as cleared
func (msc *Controller) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	dms := obj.(*mv1.DorisDisaggregatedMetaService)
	// DeletionTimestamp is IsZero means dms not delete
	// clear deleted statefulset Resources
	if dms.DeletionTimestamp.IsZero() {
		return true, nil
	}

	if dms.Spec.MS == nil {
		return msc.ClearMSCommonResources(ctx, dms, mv1.Component_MS)
	}
	return true, nil
}

func (msc *Controller) GetControllerName() string {
	return msc.ControllerName
}

func (msc *Controller) UpdateComponentStatus(obj client.Object) error {
	dms := obj.(*mv1.DorisDisaggregatedMetaService)

	if dms.Spec.MS == nil {
		return nil
	}
	return msc.ClassifyPodsByStatus(dms.Namespace, &dms.Status.MSStatus, mv1.GenerateStatefulSetSelector(dms, mv1.Component_MS), mv1.DefaultMetaserviceNumber)
}

func (d *Controller) initMSStatus(dms *mv1.DorisDisaggregatedMetaService) {
	initPhase := mv1.Creating

	if mv1.IsReconcilingStatusPhase(dms.Status.MSStatus.Phase) {
		initPhase = dms.Status.MSStatus.Phase
	}
	status := mv1.BaseStatus{
		Phase:           initPhase,
		AvailableStatus: mv1.UnAvailable,
	}
	dms.Status.MSStatus = status
}
