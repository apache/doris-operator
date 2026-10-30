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

package namespacescope

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Scope filters reconciliation, not read permissions. The global cache remains
// available for explicit cross-namespace dependencies such as FDB ConfigMaps.
type Scope struct {
	reader   client.Reader
	selector labels.Selector
}

func New(reader client.Reader, expression string) (*Scope, error) {
	if expression == "" {
		return nil, nil
	}
	selector, err := labels.Parse(expression)
	if err != nil {
		return nil, err
	}
	if selector.Empty() {
		return nil, fmt.Errorf("namespace selector must not select every namespace")
	}
	return &Scope{reader: reader, selector: selector}, nil
}

func (s *Scope) Enabled() bool { return s != nil }

func (s *Scope) Allows(ctx context.Context, namespace string) (bool, error) {
	if s == nil {
		return true, nil
	}
	var ns corev1.Namespace
	// Use the API reader: cached namespace labels must not retain ownership after
	// an administrator removes the group label. Read failures never permit writes.
	if err := s.reader.Get(ctx, types.NamespacedName{Name: namespace}, &ns); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return s.selector.Matches(labels.Set(ns.Labels)), nil
}

// Watch also adopts existing CRs when a namespace joins the selected group.
func Watch(b *ctrl.Builder, c client.Client, list func() client.ObjectList) *ctrl.Builder {
	return b.Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, ns client.Object) []reconcile.Request {
		objects := list()
		if err := c.List(ctx, objects, client.InNamespace(ns.GetName())); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "list namespace resources", "namespace", ns.GetName())
			return nil
		}
		items, err := meta.ExtractList(objects)
		if err != nil {
			return nil
		}
		requests := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			obj, err := meta.Accessor(item)
			if err == nil {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}})
			}
		}
		return requests
	}), builder.WithPredicates(predicate.LabelChangedPredicate{}))
}
