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

package recycler

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

type RecyclerController struct {
	sub_controller.DisaggregatedSubDefaultController
}

var (
	disaggregatedRecyclerController = "disaggregatedRecyclerController"
)

func New(mgr ctrl.Manager) *RecyclerController {
	return &RecyclerController{
		sub_controller.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(disaggregatedRecyclerController),
			ControllerName: disaggregatedRecyclerController,
		}}
}

func (rc *RecyclerController) Sync(ctx context.Context, obj client.Object) error {
	ddm := obj.(*mv1.DorisDisaggregatedMetaService)
	if ddm.Spec.Recycler == nil {
		return nil
	}

	if ddm.Status.FDBStatus.AvailableStatus != mv1.Available {
		klog.Infof("recycle controller waiting fdb ready namespace %s name %s.", ddm.Namespace, ddm.Name)
		return nil
	}

	rc.initRCStatus(ddm)
	rc.CheckMSConfigMountPath(ddm, mv1.Component_RC)

	// recycler is a special start mode of ms.
	config, err := rc.GetMSConfig(ctx, ddm.Spec.Recycler.ConfigMaps, ddm.Namespace, mv1.Component_RC)
	if err != nil {
		return err
	}

	service := resource.BuildDMSService(ddm, mv1.Component_RC, resource.GetPort(config, resource.BRPC_LISTEN_PORT))
	if err = k8s.ApplyService(ctx, rc.K8sclient, &service, resource.DMSServiceDeepEqual); err != nil {
		klog.Errorf("rc controller sync apply service name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			service.Name, service.Namespace, ddm.Name, err.Error())
		return err
	}

	st := rc.buildRCStatefulSet(ddm)
	if err = k8s.ApplyStatefulSet(ctx, rc.K8sclient, &st, func(new *appv1.StatefulSet, old *appv1.StatefulSet) bool {
		rc.RestrictConditionsEqual(new, old)
		return resource.DMSStatefulSetDeepEqual(new, old, false)
	}); err != nil {
		klog.Errorf("rc controller sync statefulset name=%s, namespace=%s, disaggregated-metaservice-name=%s failed. message=%s.",
			st.Name, st.Namespace, ddm.Name, err.Error())
		return err
	}

	return nil
}

func (rc *RecyclerController) initRCStatus(ddm *mv1.DorisDisaggregatedMetaService) {
	initPhase := mv1.Creating

	if mv1.IsReconcilingStatusPhase(ddm.Status.RecyclerStatus.Phase) {
		initPhase = ddm.Status.RecyclerStatus.Phase
	}
	status := mv1.BaseStatus{
		Phase:           initPhase,
		AvailableStatus: mv1.UnAvailable,
	}
	ddm.Status.RecyclerStatus = status
}

func (rc *RecyclerController) initRecyclerStatus(ddm *mv1.DorisDisaggregatedMetaService) {
	initPhase := mv1.Creating

	if mv1.IsReconcilingStatusPhase(ddm.Status.RecyclerStatus.Phase) {
		initPhase = ddm.Status.MSStatus.Phase
	}
	status := mv1.BaseStatus{
		Phase:           initPhase,
		AvailableStatus: mv1.UnAvailable,
	}
	ddm.Status.RecyclerStatus = status
}

func (rc *RecyclerController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	dms := obj.(*mv1.DorisDisaggregatedMetaService)
	// DeletionTimestamp is IsZero means dms not delete
	// clear deleted statefulset Resources
	if dms.DeletionTimestamp.IsZero() {
		return true, nil
	}

	if dms.Spec.Recycler == nil {
		return rc.ClearMSCommonResources(ctx, dms, mv1.Component_RC)
	}
	return true, nil
}

func (rc *RecyclerController) GetControllerName() string {
	return disaggregatedRecyclerController
}

func (rc *RecyclerController) UpdateComponentStatus(obj client.Object) error {
	ddm := obj.(*mv1.DorisDisaggregatedMetaService)

	if ddm.Spec.Recycler == nil {
		return nil
	}
	return rc.ClassifyPodsByStatus(ddm.Namespace, &ddm.Status.RecyclerStatus, mv1.GenerateStatefulSetSelector(ddm, mv1.Component_RC), mv1.DefaultRecyclerNumber)
}
