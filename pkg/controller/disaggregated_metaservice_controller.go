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

package controller

import (
	"context"
	"github.com/google/go-cmp/cmp"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_metaservice/fdb"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_metaservice/ms"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_metaservice/recycler"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

var (
	_ reconcile.Reconciler = &DisaggregatedMetaServiceReconciler{}

	_ Controller = &DisaggregatedMetaServiceReconciler{}
)

var (
	disaggregatedMetaServiceControllerName = "disaggregatedMetaServiceControllerName"
)

type DisaggregatedMetaServiceReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Scs      map[string]sub_controller.DisaggregatedSubController
}

func (dmsr *DisaggregatedMetaServiceReconciler) Init(mgr ctrl.Manager, options *Options) {
	scs := make(map[string]sub_controller.DisaggregatedSubController)
	dfdbc := fdb.New(mgr)
	scs[dfdbc.GetControllerName()] = dfdbc
	dmsc := ms.New(mgr)
	scs[dmsc.GetControllerName()] = dmsc
	dryc := recycler.New(mgr)
	scs[dryc.GetControllerName()] = dryc

	if err := (&DisaggregatedMetaServiceReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor(disaggregatedMetaServiceControllerName),
		Scs:      scs,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller ", "disaggregatedMetaServiceReconciler")
		os.Exit(1)
	}

	if options.EnableWebHook {
		if err := (&mv1.DorisDisaggregatedMetaService{}).SetupWebhookWithManager(mgr); err != nil {
			klog.Error(err, "  unable to create unamedwatches ", " controller ", " DorisDisaggregatedMetaService ")
			os.Exit(1)
		}
	}
}

func (dmsr *DisaggregatedMetaServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return dmsr.resourceBuilder(ctrl.NewControllerManagedBy(mgr)).Complete(dmsr)
}

func (dmsr *DisaggregatedMetaServiceReconciler) resourceBuilder(builder *ctrl.Builder) *ctrl.Builder {
	return builder.For(&mv1.DorisDisaggregatedMetaService{}).Owns(&appv1.StatefulSet{}).Owns(&corev1.Service{})
}

func (dmsr *DisaggregatedMetaServiceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var dms mv1.DorisDisaggregatedMetaService
	err := dmsr.Get(ctx, req.NamespacedName, &dms)
	if apierrors.IsNotFound(err) {
		klog.Warningf("disaggregatedMetaServiceReconciler not found resource DisaggregatedMetaService namespaceName %s", req.NamespacedName)
		return ctrl.Result{}, nil
	} else if err != nil {
		klog.Errorf("disaggregatedMetaServiceReconciler DisaggregatedMetaService namespaceName %s not found.", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// clear Resources if statefulset delete
	if dms.DeletionTimestamp != nil {
		dmsr.resourceClean(ctx, &dms)
		return ctrl.Result{}, nil
	}

	for _, rc := range dmsr.Scs {
		if err := rc.Sync(ctx, &dms); err != nil {
			klog.Errorf("disaggregatedMetaServiceReconciler sub reconciler %s reconcile err=%s.", rc.GetControllerName(), err.Error())
			return requeueIfError(err)
		}
	}

	// clear pvc Resources if pod scale down
	dmsr.resourceClean(ctx, &dms)
	for _, rc := range dmsr.Scs {
		if err := rc.UpdateComponentStatus(&dms); err != nil {
			klog.Errorf("disaggregatedMetaServiceReconciler sub reconciler %s update status err=%s.", rc.GetControllerName(), err.Error())
			return requeueIfError(err)
		}
	}

	return dmsr.updateDisaggregatedMetaServiceStatus(ctx, &dms)
}

// updateDisaggregatedMetaServiceStatus confirm and correct CR status and control reconcile according to the status.
func (dmsr *DisaggregatedMetaServiceReconciler) updateDisaggregatedMetaServiceStatus(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService) (ctrl.Result, error) {
	var edms mv1.DorisDisaggregatedMetaService
	if err := dmsr.Get(ctx, types.NamespacedName{Namespace: dms.Namespace, Name: dms.Name}, &edms); err != nil {
		return requeueIfError(err)
	}

	// if the status is not equal before and now the status is not available should requeue.
	if cmp.Equal(dms.Status, edms.Status) {
		if needReconcile(edms) {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, nil
	}

	dms.Status.DeepCopyInto(&edms.Status)
	return ctrl.Result{}, retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return dmsr.Client.Status().Update(ctx, &edms)
	})
}

// resourceClean provides unified resource cleanup capabilities, including clearing CR marks and clearing some resources.
func (dmsr *DisaggregatedMetaServiceReconciler) resourceClean(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService) {
	for _, rc := range dmsr.Scs {
		rc.ClearResources(ctx, dms)
	}
}

func needReconcile(dms mv1.DorisDisaggregatedMetaService) bool {
	if dms.Spec.FDB != nil {
		if dms.Status.FDBStatus.AvailableStatus == mv1.UnAvailable {
			return true
		}
	}

	if dms.Spec.MS != nil {
		if dms.Status.MSStatus.AvailableStatus == mv1.UnAvailable || dms.Status.MSStatus.Phase != mv1.Ready {
			return true
		}
	}

	if dms.Spec.Recycler != nil {
		if dms.Status.RecyclerStatus.AvailableStatus == mv1.UnAvailable || dms.Status.RecyclerStatus.Phase != mv1.Ready {
			return true
		}
	}

	return false
}
