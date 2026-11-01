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
	"errors"
	"os"
	"time"

	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PVCFinalizerReconciler completes an already requested PVC deletion independently
// of the parent CR. It never initiates deletion or removes other controllers'
// protection, and covers DCR as well as every DDC component.
type PVCFinalizerReconciler struct {
	client.Client
	APIReader client.Reader
}

var _ Controller = &PVCFinalizerReconciler{}

func (r *PVCFinalizerReconciler) Init(mgr ctrl.Manager, options *Options) {
	reconciler := &PVCFinalizerReconciler{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader()}
	b := ctrl.NewControllerManagedBy(mgr).Named("doris-pvc-finalizer").
		For(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(predicate.NewPredicateFuncs(pvcDeletionRequested)))
	if err := b.Complete(reconciler); err != nil {
		ctrl.Log.Error(err, "unable to create PVC finalizer controller")
		os.Exit(1)
	}
}

func pvcDeletionRequested(obj client.Object) bool {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	return ok && !pvc.DeletionTimestamp.IsZero() && len(resource.ManagedPVCFinalizers(pvc)) != 0
}

// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
func (r *PVCFinalizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reader := r.APIReader
	if reader == nil {
		reader = r.Client
	}
	var pvc corev1.PersistentVolumeClaim
	if err := reader.Get(ctx, req.NamespacedName, &pvc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !pvcDeletionRequested(&pvc) {
		return ctrl.Result{}, nil
	}
	lifecycle := &sc.DisaggregatedSubDefaultController{K8sclient: r.Client, APIReader: reader}
	if err := lifecycle.FinishPVCDeletion(ctx, &pvc); err != nil {
		if errors.Is(err, sc.ErrPVCInUse) {
			// Pod termination need not produce another PVC event. Poll until
			// scheduled references are gone, including terminating Pods.
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
