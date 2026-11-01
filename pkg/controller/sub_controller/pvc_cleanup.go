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

package sub_controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ErrPVCInUse means a scheduled Pod must release its claim before finalization.
var ErrPVCInUse = errors.New("waiting for scheduled Pods to release deleting PVC")

// PVCReader bypasses the informer cache for destructive lifecycle decisions.
func (d *DisaggregatedSubDefaultController) PVCReader() client.Reader {
	if d.APIReader != nil {
		return d.APIReader
	}
	return d.K8sclient
}

func (d *DisaggregatedSubDefaultController) pvcReferenced(ctx context.Context, pvc *corev1.PersistentVolumeClaim, scheduledOnly bool) (bool, error) {
	var pods corev1.PodList
	if err := d.PVCReader().List(ctx, &pods, client.InNamespace(pvc.Namespace)); err != nil {
		return false, err
	}
	for _, pod := range pods.Items {
		// An unscheduled Pod cannot have mounted a claim. In particular, the
		// pending Pod from a failed scale-up must not block deletion recovery.
		if scheduledOnly && pod.Spec.NodeName == "" {
			continue
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				return true, nil
			}
		}
	}
	return false, nil
}

// DeleteUnusedPVC only starts deletion once all referencing Pods have gone.
// UID/resourceVersion preconditions protect a recreated or concurrently changed
// claim; Kubernetes and third-party finalizers retain their own protections.
func (d *DisaggregatedSubDefaultController) DeleteUnusedPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	used, err := d.pvcReferenced(ctx, pvc, false)
	if err != nil {
		return err
	}
	if used {
		return fmt.Errorf("waiting for Pods to release PVC %s/%s", pvc.Namespace, pvc.Name)
	}
	if pvc.DeletionTimestamp.IsZero() {
		uid, rv := pvc.UID, pvc.ResourceVersion
		if err := d.K8sclient.Delete(ctx, pvc, &client.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &rv}}); err != nil {
			return client.IgnoreNotFound(err)
		}
	}
	return d.FinishPVCDeletion(ctx, pvc)
}

// FinishPVCDeletion releases only our finalizer on an operator-managed claim
// whose deletion was already requested. Never unprotect a mounted claim.
func (d *DisaggregatedSubDefaultController) FinishPVCDeletion(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	var current corev1.PersistentVolumeClaim
	if err := d.PVCReader().Get(ctx, client.ObjectKeyFromObject(pvc), &current); err != nil {
		return client.IgnoreNotFound(err)
	}
	if current.UID != pvc.UID || current.DeletionTimestamp.IsZero() {
		return nil
	}
	finalizers := resource.ManagedPVCFinalizers(&current)
	if len(finalizers) == 0 {
		return nil
	}
	used, err := d.pvcReferenced(ctx, &current, true)
	if err != nil {
		return err
	}
	if used {
		return fmt.Errorf("%w %s/%s", ErrPVCInUse, current.Namespace, current.Name)
	}
	before := current.DeepCopy()
	for _, finalizer := range finalizers {
		controllerutil.RemoveFinalizer(&current, finalizer)
	}
	return client.IgnoreNotFound(d.K8sclient.Patch(ctx, &current,
		client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})))
}
