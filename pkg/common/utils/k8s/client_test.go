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
package k8s

import (
	"context"
	"testing"

	"github.com/FoundationDB/fdb-kubernetes-operator/api/v1beta2"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_ApplyService(t *testing.T) {
	svcs := []client.Object{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: "test",
			}, Spec: corev1.ServiceSpec{}},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			}, Spec: corev1.ServiceSpec{Selector: map[string]string{"namespace": "test", "name": "test2"}}}}

	fakeClient := fake.NewClientBuilder().WithObjects(svcs...).Build()
	tsvcs := []*corev1.Service{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testnoexist",
				Namespace: "test",
			},
			Spec: corev1.ServiceSpec{},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"namespace": "test", "name": "test2"}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"namespace": "test", "name": "test2"}, Type: corev1.ServiceTypeNodePort},
		},
	}

	for _, svc := range tsvcs {
		err := ApplyService(context.Background(), fakeClient, svc, resource.ServiceDeepEqual)
		if err != nil {
			t.Errorf("apply service %s failed, err %s", svc.Name, err.Error())
		}
	}
}

func Test_ApplyStatefulSet(t *testing.T) {
	svcs := []client.Object{
		&appv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: "test",
			}, Spec: appv1.StatefulSetSpec{}},
		&appv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			}, Spec: appv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"namespace": "test", "name": "test2"}},
				Replicas: pointer.Int32(1),
			}}}

	fakeClient := fake.NewClientBuilder().WithObjects(svcs...).Build()
	tsts := []*appv1.StatefulSet{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testnoexist",
				Namespace: "test",
			},
			Spec: appv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"namespace": "test", "name": "testnoexist"}},
				Replicas: pointer.Int32(1),
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe", Image: "test", Env: []corev1.EnvVar{{Name: "k", Value: "v"}}}}}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			},
			Spec: appv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"namespace": "test", "name": "test2"}},
				Replicas: pointer.Int32(1),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			},
			Spec: appv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"namespace": "test", "name": "test2"}},
				Replicas: pointer.Int32(1),
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe", Image: "test"}}}},
			},
		},
	}

	for _, st := range tsts {
		err := ApplyStatefulSet(context.Background(), fakeClient, st, func(st1 *appv1.StatefulSet, st2 *appv1.StatefulSet) bool {
			return resource.StatefulSetDeepEqual(st1, st2, false)
		})
		if err != nil {
			t.Errorf("apply service %s failed, err %s", st.Name, err.Error())
		}
	}
}

func Test_ApplyFoundationDBCluster(t *testing.T) {
	fdbs := []client.Object{
		&v1beta2.FoundationDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: "test",
			}, Spec: v1beta2.FoundationDBClusterSpec{}},
		&v1beta2.FoundationDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			}, Spec: v1beta2.FoundationDBClusterSpec{Version: "7.1.38"}}}

	scheme := runtime.NewScheme()
	v1beta2.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fdbs...).Build()
	tfdbs := []*v1beta2.FoundationDBCluster{
		&v1beta2.FoundationDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testnoexist",
				Namespace: "test",
			}, Spec: v1beta2.FoundationDBClusterSpec{}},
		&v1beta2.FoundationDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			}, Spec: v1beta2.FoundationDBClusterSpec{Version: "7.1.38", UseUnifiedImage: pointer.Bool(true)}}}

	for _, tfdb := range tfdbs {
		err := ApplyFoundationDBCluster(context.Background(), fakeClient, tfdb)
		if err != nil {
			t.Errorf("apply foundationdb cluster %s failed, err=%s", tfdb.Name, err.Error())
		}
	}
}

func Test_DeleteFoundationDBCluster(t *testing.T) {
	fdbs := []client.Object{
		&v1beta2.FoundationDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: "test",
			}, Spec: v1beta2.FoundationDBClusterSpec{}},
		&v1beta2.FoundationDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			}, Spec: v1beta2.FoundationDBClusterSpec{Version: "7.1.38"}}}
	scheme := runtime.NewScheme()
	v1beta2.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fdbs...).Build()
	tfdbs := []*v1beta2.FoundationDBCluster{
		&v1beta2.FoundationDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testnoexist",
				Namespace: "test",
			}, Spec: v1beta2.FoundationDBClusterSpec{}},
		&v1beta2.FoundationDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "test",
			}, Spec: v1beta2.FoundationDBClusterSpec{Version: "7.1.38", UseUnifiedImage: pointer.Bool(true)}}}

	for _, tfdb := range tfdbs {
		err := DeleteFoundationDBCluster(context.Background(), fakeClient, tfdb.Namespace, tfdb.Name)
		if err != nil {
			t.Errorf("apply foundationdb cluster %s failed, err=%s", tfdb.Name, err.Error())
		}
	}
}

func Test_DeletePVC(t *testing.T) {
	pvcs := []client.Object{
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: "test",
			},
			Spec: corev1.PersistentVolumeClaimSpec{},
		},
	}
	fakeClient := fake.NewClientBuilder().WithObjects(pvcs...).Build()

	testInnamespaces := []types.NamespacedName{
		{
			Namespace: "test",
			Name:      "noexist",
		},
		{
			Name:      "test1",
			Namespace: "test",
		},
	}
	for _, nn := range testInnamespaces {
		err := DeletePVC(context.Background(), fakeClient, nn.Namespace, nn.Name, map[string]string{})
		if err != nil {
			t.Errorf("delete pvc failed, pvc name=%s, err=%s", nn.Name, err.Error())
		}
	}
}
