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
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

func liveService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "fe", Namespace: "test", Labels: map[string]string{"app": "fe"}}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, SessionAffinity: corev1.ServiceAffinityNone, LoadBalancerIP: "10.0.0.1", Selector: map[string]string{"app": "fe"}, Ports: []corev1.ServicePort{{Name: "query", Port: 9030, TargetPort: intstr.FromInt32(9030)}}}}
}
func TestApplyServiceCorrectsLiveDrift(t *testing.T) {
	for _, key := range []string{v1.ComponentResourceHash, dv1.DisaggregatedSpecHashValueAnnotation} {
		for _, field := range []string{"selector", "port", "type", "loadBalancerIP", "sessionAffinity", "publishNotReady", "labels"} {
			t.Run(key+"/"+field, func(t *testing.T) {
				old := liveService()
				resource.ServiceDeepEqualWithAnnoKey(old, old.DeepCopy(), key)
				old.Spec.ClusterIP = "10.2.0.1"
				old.Spec.ClusterIPs = []string{"10.2.0.1"}
				old.Spec.Ports[0].NodePort = 30090
				old.Spec.Ports[0].Protocol = corev1.ProtocolTCP
				old.Spec.AllocateLoadBalancerNodePorts = ptr.To(true)
				old.Annotations["cloud.example/allocated"] = "keep"
				switch field {
				case "selector":
					old.Spec.Selector = map[string]string{"app": "wrong"}
				case "port":
					old.Spec.Ports[0].Port = 19030
				case "type":
					old.Spec.Type = corev1.ServiceTypeNodePort
				case "loadBalancerIP":
					old.Spec.LoadBalancerIP = "10.0.0.2"
				case "sessionAffinity":
					old.Spec.SessionAffinity = corev1.ServiceAffinityClientIP
				case "publishNotReady":
					old.Spec.PublishNotReadyAddresses = true
				case "labels":
					old.Labels["app"] = "wrong"
				}
				c := fake.NewClientBuilder().WithObjects(old).Build()
				desired := liveService()
				eq := func(n, o *corev1.Service) bool { return resource.ServiceDeepEqualWithAnnoKey(n, o, key) }
				if err := k8s.ApplyService(context.Background(), c, desired, eq); err != nil {
					t.Fatal(err)
				}
				got := &corev1.Service{}
				if err := c.Get(context.Background(), client.ObjectKeyFromObject(old), got); err != nil {
					t.Fatal(err)
				}
				if got.Spec.Selector["app"] != "fe" || got.Spec.Ports[0].Port != 9030 || got.Spec.Type != corev1.ServiceTypeLoadBalancer || got.Spec.LoadBalancerIP != "10.0.0.1" || got.Spec.SessionAffinity != corev1.ServiceAffinityNone || got.Spec.PublishNotReadyAddresses || got.Labels["app"] != "fe" {
					t.Fatalf("live drift not repaired: %+v", got.Spec)
				}
				if got.Spec.ClusterIP != "10.2.0.1" || got.Spec.Ports[0].NodePort != 30090 || got.Annotations["cloud.example/allocated"] != "keep" {
					t.Fatal("allocated fields or external annotation changed")
				}
				repeat := liveService()
				if !eq(repeat, got) {
					t.Fatal("repeat reconcile not equal")
				}
			})
		}
	}
}
func TestServiceDefaultsAndAllocatedPortsAreEqual(t *testing.T) {
	desired := liveService()
	old := desired.DeepCopy()
	old.Spec.ClusterIP = "10.2.0.1"
	old.Spec.Ports[0].Protocol = corev1.ProtocolTCP
	old.Spec.Ports[0].NodePort = 30090
	old.Annotations = map[string]string{"cloud.example/allocated": "keep"}
	old.Labels["external"] = "keep"
	if !resource.ServiceDeepEqual(desired, old) {
		t.Fatal("API defaults or external metadata cause unnecessary update")
	}
}

func TestServiceLiveReconcileWithAPIDefaulting(t *testing.T) {
	environment := &envtest.Environment{}
	cfg, err := environment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Error(err)
		}
	})
	c, err := client.New(cfg, client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}); err != nil {
		t.Fatal(err)
	}
	service := liveService()
	resource.ServiceDeepEqual(service, service.DeepCopy())
	if err := c.Create(ctx, service); err != nil {
		t.Fatal(err)
	}
	allocatedIP, nodePort := service.Spec.ClusterIP, service.Spec.Ports[0].NodePort
	if err := c.Patch(ctx, service, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"selector":{"app":"wrong"},"publishNotReadyAddresses":true}}`))); err != nil {
		t.Fatal(err)
	}
	if err := k8s.ApplyService(ctx, c, liveService(), resource.ServiceDeepEqual); err != nil {
		t.Fatal(err)
	}
	got := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(service), got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.Selector["app"] != "fe" || got.Spec.PublishNotReadyAddresses || got.Spec.ClusterIP != allocatedIP || got.Spec.Ports[0].NodePort != nodePort {
		t.Fatalf("incorrect repaired service: %+v", got.Spec)
	}
	rv := got.ResourceVersion
	if err := k8s.ApplyService(ctx, c, liveService(), resource.ServiceDeepEqual); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(service), got); err != nil {
		t.Fatal(err)
	}
	if got.ResourceVersion != rv {
		t.Fatal("unchanged service was patched again")
	}
}
