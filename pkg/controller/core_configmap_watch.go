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

/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	builder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sort"
)

const coreConfigMapIndex = "doris.coreConfigMap"

func coreConfigMapIndexKeys(obj client.Object) []string {
	dcr, ok := obj.(*v1.DorisCluster)
	if !ok || !dcr.Spec.EnableRestartWhenConfigChange {
		return nil
	}
	names := map[string]struct{}{}
	for _, name := range resource.GetDorisCoreConfigMapNames(dcr) {
		if name != "" {
			names[name] = struct{}{}
		}
	}
	keys := make([]string, 0, len(names))
	for name := range names {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}
func (r *DorisClusterReconciler) mapCoreConfigMapToClusters(ctx context.Context, obj client.Object) []reconcile.Request {
	var clusters v1.DorisClusterList
	if err := r.List(ctx, &clusters, client.InNamespace(obj.GetNamespace()), client.MatchingFields{coreConfigMapIndex: obj.GetName()}); err != nil {
		klog.Errorf("list clusters for ConfigMap %s/%s: %v", obj.GetNamespace(), obj.GetName(), err)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(clusters.Items))
	for i := range clusters.Items {
		dcr := &clusters.Items[i]
		if dcr.DeletionTimestamp.IsZero() {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(dcr)})
		}
	}
	return requests
}
func coreConfigMapPredicate() predicate.Predicate {
	return predicate.Funcs{UpdateFunc: func(e event.UpdateEvent) bool {
		old, ok := e.ObjectOld.(*corev1.ConfigMap)
		if !ok {
			return false
		}
		current, ok := e.ObjectNew.(*corev1.ConfigMap)
		if !ok {
			return false
		}
		return !reflect.DeepEqual(old.Data, current.Data) || !reflect.DeepEqual(old.BinaryData, current.BinaryData)
	}, GenericFunc: func(event.GenericEvent) bool { return false }}
}
func (r *DorisClusterReconciler) watchConfigMapBuilder(b *ctrl.Builder) *ctrl.Builder {
	return b.Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.mapCoreConfigMapToClusters), builder.WithPredicates(coreConfigMapPredicate()))
}
