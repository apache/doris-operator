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
package cn

import (
	"context"
	"fmt"
	v1 "github.com/apache/doris-operator/api/doris/v1"
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
		fmt.Println("Test_ClearResources NewManager failed, err=" + err.Error())
	}
	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
	}()
}

// Sync only calls ClearResources when CnSpec is nil, so a cluster whose cnSpec was removed while
// status.cnStatus is still set must not dereference the spec.
func Test_ClearResources(t *testing.T) {
	defer deferClear()
	cc := New(mgr.GetClient(), mgr.GetEventRecorderFor("test-cn"))
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1.DorisClusterSpec{
			CnSpec: nil,
		},
		Status: v1.DorisClusterStatus{
			CnStatus: &v1.CnStatus{
				ComponentStatus: v1.ComponentStatus{
					ComponentCondition: v1.ComponentCondition{
						Phase: v1.Reconciling,
					},
				},
			},
		},
	}

	cleared, err := cc.ClearResources(context.Background(), cluster)
	if err != nil {
		t.Errorf("Test_ClearResources clear resources failed, err=%s", err.Error())
	}
	if !cleared {
		t.Error("Test_ClearResources expected resources to be cleared")
	}
}

// A cluster that never had cn returns early on the nil status, before reaching the spec.
func Test_ClearResources_NoCnStatus(t *testing.T) {
	cc := New(mgr.GetClient(), mgr.GetEventRecorderFor("test-cn"))
	cluster := &v1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-no-cn",
			Namespace: "default",
		},
	}

	cleared, err := cc.ClearResources(context.Background(), cluster)
	if err != nil {
		t.Errorf("Test_ClearResources_NoCnStatus failed, err=%s", err.Error())
	}
	if !cleared {
		t.Error("Test_ClearResources_NoCnStatus expected resources to be cleared")
	}
}

func deferClear() {
	err := testEnv.Stop()
	if err != nil {
		fmt.Println(err)
	}
}
