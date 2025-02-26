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
package fe

import (
    "context"
    "fmt"
    v1 "github.com/apache/doris-operator/api/doris/v1"
    "github.com/apache/doris-operator/pkg/common/utils/resource"
    corev1 "k8s.io/api/core/v1"
    kr "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    k8sruntime "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/client-go/kubernetes/scheme"
    "k8s.io/client-go/rest"
    "path/filepath"
    "runtime"
    ctrl "sigs.k8s.io/controller-runtime"
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

    pvcs := []corev1.PersistentVolumeClaim{
        {
            ObjectMeta: metav1.ObjectMeta{
                Name:      "test-fe-0",
                Namespace: "default",
                Labels: map[string]string{
                    v1.OwnerReference:    "test-fe",
                    v1.ComponentLabelKey: "fe",
                },
            },
            Spec: corev1.PersistentVolumeClaimSpec{
                AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
                Resources: corev1.VolumeResourceRequirements{
                    Requests: map[corev1.ResourceName]kr.Quantity{
                        "storage": kr.MustParse("500Gi"),
                    },
                },
                Selector: &metav1.LabelSelector{
                    MatchLabels: map[string]string{
                        v1.OwnerReference:    "test-fe",
                        v1.ComponentLabelKey: "fe",
                    },
                },
            },
        },
        {
            ObjectMeta: metav1.ObjectMeta{
                Name:      "test-fe-1",
                Namespace: "default",
                Labels: map[string]string{
                    v1.OwnerReference:    "test-fe",
                    v1.ComponentLabelKey: "fe",
                },
            },
            Spec: corev1.PersistentVolumeClaimSpec{
                AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
                Resources: corev1.VolumeResourceRequirements{
                    Requests: map[corev1.ResourceName]kr.Quantity{
                        "storage": kr.MustParse("500Gi"),
                    },
                },
                Selector: &metav1.LabelSelector{
                    MatchLabels: map[string]string{
                        v1.OwnerReference:    "test-fe",
                        v1.ComponentLabelKey: "fe",
                    },
                },
            },
        },
        {
            ObjectMeta: metav1.ObjectMeta{
                Name:      "test-fe-2",
                Namespace: "default",
                Labels: map[string]string{
                    v1.OwnerReference:    "test-fe",
                    v1.ComponentLabelKey: "fe",
                },
            },
            Spec: corev1.PersistentVolumeClaimSpec{
                AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
                Resources: corev1.VolumeResourceRequirements{
                    Requests: map[corev1.ResourceName]kr.Quantity{
                        "storage": kr.MustParse("500Gi"),
                    },
                },
                Selector: &metav1.LabelSelector{},
            },
        },
    }

    for i, _ := range pvcs {
        if err := mgr.GetClient().Create(context.Background(), &pvcs[i]); err != nil {
            t.Errorf("create pvc failed,err=%s", err.Error())
        }
    }

    fc := New(mgr.GetClient(), mgr.GetEventRecorderFor("test-fe"))
    cluster := &v1.DorisCluster{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test",
            Namespace: "default",
        },
        Spec: v1.DorisClusterSpec{
            FeSpec: &v1.FeSpec{
                BaseSpec: v1.BaseSpec{
                    Replicas: resource.GetInt32Pointer(3),
                    PersistentVolumes: []v1.PersistentVolume{
                        {
                            MountPath: "/opt/apache-doris/fe/doris-meta",
                            PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
                                AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
                                Resources: corev1.VolumeResourceRequirements{
                                    Requests: corev1.ResourceList{
                                        "storage": kr.MustParse("500Gi"),
                                    },
                                },
                            },
                        },
                    },
                },
            },
        },
        Status: v1.DorisClusterStatus{
            FEStatus: &v1.ComponentStatus{
                ComponentCondition: v1.ComponentCondition{
                    Phase: v1.Available,
                },
            },
        },
    }
    fc.ClearResources(context.Background(), cluster)
}

func deferClear() {
    err := testEnv.Stop()
    if err != nil {
        fmt.Println(err)
    }
}
