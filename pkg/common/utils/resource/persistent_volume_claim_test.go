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

package resource

import (
	"fmt"

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
)

func Test_BuildPVCAnnotations(t *testing.T) {
	test := dorisv1.PersistentVolume{
		Name:      "test",
		MountPath: "/etc/doris",
		Annotations: NewAnnotations(Annotations{
			"test": "test",
		}),
		PVCProvisioner: "Operator",
	}

	anno := buildPVCAnnotations(test)
	if _, ok := anno[pvc_manager_annotation]; !ok {
		t.Errorf("buildPVCAnnotations failed, not \"pvc_manager_annotation\" annotation.")
	}
}

func TestBuildPVCDoesNotAddOperatorFinalizer(t *testing.T) {
	pvc := BuildPVC(
		dorisv1.PersistentVolume{}, map[string]string{"app": "doris"}, "default", "doris-fe", "0")
	if len(pvc.Finalizers) != 0 {
		t.Fatalf("BuildPVC finalizers = %v, want none", pvc.Finalizers)
	}

	pvc = BuildDisaggregatedPVC(
		corev1.PersistentVolumeClaim{}, map[string]string{"app": "doris"}, "default", "doris-cg", "0")
	if len(pvc.Finalizers) != 0 {
		t.Fatalf("BuildDisaggregatedPVC finalizers = %v, want none", pvc.Finalizers)
	}
}

func TestRemoveOperatorPVCFinalizersPreservesKubernetesFinalizers(t *testing.T) {
	got := RemoveOperatorPVCFinalizers(
		[]string{pvc_finalizer, pvcFinalizerApache, "kubernetes.io/pvc-protection"})
	if len(got) != 1 || got[0] != "kubernetes.io/pvc-protection" {
		t.Fatalf("RemoveOperatorPVCFinalizers = %v", got)
	}
}

func Test_Result(t *testing.T) {
	res := ctrl.Result{}
	if res.IsZero() {
		fmt.Println("test true")
	}
}
