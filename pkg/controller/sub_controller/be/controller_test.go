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
package be

import (
    "context"
    "fmt"
    v1 "github.com/apache/doris-operator/api/doris/v1"
    "github.com/apache/doris-operator/pkg/common/utils/resource"
    appv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    k8sruntime "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    "k8s.io/client-go/kubernetes/scheme"
    "k8s.io/client-go/rest"
    "path/filepath"
    "runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
    "testing"
)

var cfg *rest.Config
var testEnv *envtest.Environment
var mgr ctrl.Manager

func init() {
    testEnv = &envtest.Environment{
        Scheme: k8sruntime.NewScheme(),
        BinaryAssetsDirectory: filepath.Join("..", "..", "..", "..", "bin", "k8s",
            fmt.Sprintf("1.26.1-%s-%s", runtime.GOOS, runtime.GOARCH)),
        CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "..", "config", "crd", "bases")},
        ErrorIfCRDPathMissing: true,
    }

    var err error
    cfg, err = testEnv.Start()
    if err != nil {
        fmt.Println("init failed, err=" + err.Error())
    }
    mgr, err = ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
    if err != nil {
        fmt.Println("Test_safeScaleDown NewManager failed, err=" + err.Error())
    }
    go func() {
        err = mgr.Start(ctrl.SetupSignalHandler())
    }()
}

func Test_ClearResources(t *testing.T) {
    defer deferClear()
    bc := New(mgr.GetClient(), mgr.GetEventRecorderFor("test-be"))
    cluster := &v1.DorisCluster{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test",
            Namespace: "default",
        },
        //Spec: v1.DorisClusterSpec{
        //    BeSpec: &v1.BeSpec{
        //        BaseSpec: v1.BaseSpec{
        //            Replicas: resource.GetInt32Pointer(3),
        //        },
        //    },
        //},
        Status: v1.DorisClusterStatus{
            BEStatus: &v1.ComponentStatus{
                ComponentCondition: v1.ComponentCondition{
                    Phase: v1.Available,
                },
            },
        },
    }

    resources := []client.Object{&appv1.StatefulSet{
        ObjectMeta: metav1.ObjectMeta{
            Name:"test-be",
            Namespace: "default",
        },
        Spec: appv1.StatefulSetSpec{
            Selector: &metav1.LabelSelector{
                MatchLabels: map[string]string{
                    v1.OwnerReference:"test",
                    v1.ComponentLabelKey: "be",
                },
            },
            Replicas: resource.GetInt32Pointer(1),
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Name: "be",
                    Labels: map[string]string{
                        v1.OwnerReference:"test",
                        v1.ComponentLabelKey: "be",
                    },
                },
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        {
                            Name: "be",
                            Image: "test.fe:1",
                        },
                    },
                },
            },
        },
    },
    &corev1.Service{
     ObjectMeta: metav1.ObjectMeta{
         Name: "test-be-internal",
         Namespace: "default",
     },
     Spec: corev1.ServiceSpec{
         Selector: map[string]string{
             v1.OwnerReference:"test",
             v1.ComponentLabelKey: "be",
         },
         Ports: []corev1.ServicePort{
             {
                 Name: "test",
                 Protocol: corev1.ProtocolTCP,
                 Port: 9030,
             },
         },
     },
    },
    &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name: "test-be-service",
            Namespace: "default",
        },
        Spec: corev1.ServiceSpec{
            Selector: map[string]string{
                v1.OwnerReference:"test",
                v1.ComponentLabelKey: "be",
            },
            Ports: []corev1.ServicePort{
                {
                    Name: "test",
                    Protocol: corev1.ProtocolTCP,
                    Port: 9030,
                },
            },
        },
    }}

    for _, dcrResource := range resources {
        if err := mgr.GetClient().Create(context.Background(), dcrResource); err != nil {
            t.Errorf("Test_ClearResources  create resource name =%s, failed,err=%s",dcrResource.GetName(), err.Error())
        }
    }
    if _, err := bc.ClearResources(context.Background(), cluster); err != nil {
         t.Errorf("Test_ClearResources clear resources failed, err=%s", err.Error())
    }

    for i,_ := range resources {
        name := resources[i].GetName()
        if err := mgr.GetClient().Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name} ,resources[i]); err != nil && !apierrors.IsNotFound(err) {
            t.Errorf("Test_ClearResources the resource is not deleted, name=%s, err=%s",name, err.Error())
        }
    }
}

func deferClear() {
    err := testEnv.Stop()
    if err != nil {
        fmt.Println(err)
    }
}
