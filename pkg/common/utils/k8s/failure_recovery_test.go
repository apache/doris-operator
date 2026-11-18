// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package k8s_test

import (
	"context"
	"errors"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"testing"
)

func auditService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "fe", Namespace: "audit"}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, LoadBalancerIP: "10.0.0.1", SessionAffinity: corev1.ServiceAffinityClientIP, Selector: map[string]string{"app": "fe"}, Ports: []corev1.ServicePort{{Name: "query", Port: 9030}}}}
}
func TestApplyServiceReturnsReadFailure(t *testing.T) {
	sentinel := errors.New("injected Get failure")
	patches := 0
	c := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
		return sentinel
	}, Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
		patches++
		return nil
	}}).Build()
	err := k8s.ApplyService(context.Background(), c, auditService(), resource.ServiceDeepEqual)
	if !errors.Is(err, sentinel) || patches != 0 {
		t.Fatalf("err=%v patches=%d", err, patches)
	}

}
func TestApplyDorisClusterReturnsConflict(t *testing.T) {
	conflict := apierrors.NewConflict(schema.GroupResource{Group: "doris.selectdb.com", Resource: "dorisclusters"}, "doris", errors.New("injected conflict"))
	c := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
		return conflict
	}}).Build()
	if err := k8s.ApplyDorisCluster(context.Background(), c, &dorisv1.DorisCluster{}); !apierrors.IsConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}

}
func TestCoreConfigReadReturnsMissingConfigMap(t *testing.T) {
	c := fake.NewClientBuilder().Build()
	d := &sc.SubDefaultController{K8sclient: c}
	conf, err := d.GetConfig(context.Background(), &dorisv1.ConfigMapInfo{ConfigMapName: "missing"}, "audit", dorisv1.Component_FE)
	if err == nil {
		t.Fatal("error correctly propagated", err)
	}
	_ = conf
}
