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

func Test_Result(t *testing.T) {
	res := ctrl.Result{}
	if res.IsZero() {
		fmt.Println("test true")
	}
}
