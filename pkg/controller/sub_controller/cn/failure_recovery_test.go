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

package cn

import (
	"context"
	"errors"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"testing"
)

func TestCNCleanupRetriesAfterDeleteFailure(t *testing.T) {
	dcr := &v1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "doris", Namespace: "test"}, Status: v1.DorisClusterStatus{CnStatus: &v1.CnStatus{}}}
	old := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: v1.GenerateComponentStatefulSetName(dcr, v1.Component_CN), Namespace: dcr.Namespace}}
	denied := true
	sentinel := errors.New("delete denied")
	c := newCnController(t)
	c.K8sclient = fake.NewClientBuilder().WithObjects(old).WithInterceptorFuncs(interceptor.Funcs{Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
		if denied {
			return sentinel
		}
		return c.Delete(ctx, obj, opts...)
	}}).Build()
	if done, err := c.ClearResources(context.Background(), dcr); done || !errors.Is(err, sentinel) {
		t.Fatalf("done=%v err=%v", done, err)
	}
	if dcr.Status.CnStatus == nil {
		t.Fatal("failed cleanup discarded status")
	}
	// Retry retains the status until all component resources have been removed.
	denied = false
	if done, err := c.ClearResources(context.Background(), dcr); !done || err != nil {
		t.Fatalf("done=%v err=%v", done, err)
	}
	list := &appsv1.StatefulSetList{}
	if err := c.K8sclient.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatal("cleanup skipped resources after status was lost")
	}
}
