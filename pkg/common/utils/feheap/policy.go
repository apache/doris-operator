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

// Package feheap adapts image-provided FE heap defaults to container memory limits.
package feheap

import (
	"context"
	"fmt"
	"reflect"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PolicyAnnotation = "doris.apache.org/fe-heap-policy"
	Auto             = "auto"
	Legacy           = "legacy"
)

// EnsurePolicy saves the decision before initializing status or creating workloads.
// An old cluster must not silently opt in when its StatefulSet is later recreated.
func EnsurePolicy(ctx context.Context, c client.Client, owner client.Object) error {
	if !owner.GetDeletionTimestamp().IsZero() {
		return nil
	}
	switch policy := owner.GetAnnotations()[PolicyAnnotation]; policy {
	case Auto, Legacy:
		return nil
	case "":
	default:
		return fmt.Errorf("invalid %s value %q: expected auto or legacy", PolicyAnnotation, policy)
	}
	var hasStatus bool
	switch o := owner.(type) {
	case *v1.DorisCluster:
		hasStatus = !reflect.ValueOf(o.Status).IsZero()
	case *dv1.DorisDisaggregatedCluster:
		hasStatus = !reflect.ValueOf(o.Status).IsZero()
	default:
		return fmt.Errorf("unsupported FE heap owner %T", owner)
	}
	policy := Auto
	if hasStatus {
		policy = Legacy
	} else {
		var workloads appsv1.StatefulSetList
		if err := c.List(ctx, &workloads, client.InNamespace(owner.GetNamespace())); err != nil {
			return err
		}
		for _, st := range workloads.Items {
			for _, ref := range st.OwnerReferences {
				// Older operators did not set Controller=true.
				if owner.GetUID() != "" && ref.UID == owner.GetUID() {
					policy = Legacy
				}
			}
		}
	}
	before := owner.DeepCopyObject().(client.Object)
	annotations := owner.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[PolicyAnnotation] = policy
	owner.SetAnnotations(annotations)
	return c.Patch(ctx, owner, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}
