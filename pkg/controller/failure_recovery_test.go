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
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/controller/sub_controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"testing"
)

func TestDDCStatusWriteFailureIsReturned(t *testing.T) {
	scheme := runtime.NewScheme()
	dv1.AddToScheme(scheme)
	ddc := &dv1.DorisDisaggregatedCluster{ObjectMeta: metav1.ObjectMeta{Name: "ddc", Namespace: "test"}}
	sentinel := errors.New("status write failed")
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ddc).WithObjects(ddc).WithInterceptorFuncs(interceptor.Funcs{SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
		return sentinel
	}}).Build()
	r := &DisaggregatedClusterReconciler{Client: c}
	_, err := r.updateDorisDisaggregatedClusterStatus(context.Background(), ddc)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected status failure, got %v", err)
	}
}
func TestNativeStatusConflictRefetchesLatestResource(t *testing.T) {
	scheme := runtime.NewScheme()
	v1.AddToScheme(scheme)
	dcr := &v1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "doris", Namespace: "test"}}
	gets, updates := 0, 0
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(dcr).WithObjects(dcr).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, k client.ObjectKey, o client.Object, opts ...client.GetOption) error {
			gets++
			return c.Get(ctx, k, o, opts...)
		},
		SubResourceUpdate: func(ctx context.Context, c client.Client, name string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			updates++
			if updates == 1 {
				latest := &v1.DorisCluster{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), latest); err != nil {
					return err
				}
				latest.Annotations = map[string]string{"concurrent": "writer"}
				if err := c.Update(ctx, latest); err != nil {
					return err
				}
				return apierrors.NewConflict(v1.Resource("dorisclusters"), obj.GetName(), errors.New("stale version"))
			}
			return c.SubResource(name).Update(ctx, obj, opts...)
		}}).Build()
	r := &DorisClusterReconciler{Client: c}
	_, err := r.updateDorisClusterStatus(context.Background(), dcr)
	if err != nil || gets < 2 || updates != 2 {
		t.Fatalf("gets=%d updates=%d err=%v", gets, updates, err)
	}
}

type failingNativeCleanup struct {
	name   string
	err    error
	called *int
}

func (f failingNativeCleanup) Sync(context.Context, *v1.DorisCluster) error { return nil }
func (f failingNativeCleanup) ClearResources(context.Context, *v1.DorisCluster) (bool, error) {
	*f.called++
	return false, f.err
}
func (f failingNativeCleanup) GetControllerName() string                    { return f.name }
func (f failingNativeCleanup) UpdateComponentStatus(*v1.DorisCluster) error { return nil }
func TestNativeCleanupCollectsFailures(t *testing.T) {
	a, b := errors.New("first delete failed"), errors.New("second delete failed")
	calls := 0
	r := &DorisClusterReconciler{Scs: map[string]sub_controller.SubController{"a": failingNativeCleanup{"a", a, &calls}, "b": failingNativeCleanup{"b", b, &calls}}}
	err := r.clearNoEffectResources(context.Background(), &v1.DorisCluster{})
	if calls != 2 || !errors.Is(err, a) || !errors.Is(err, b) {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
