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

package metaservice

import (
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestNewPodTemplateSpec_KeepsPodInfoMount(t *testing.T) {
	ddc := &dv1.DorisDisaggregatedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ddc",
			Namespace: "default",
		},
		Spec: dv1.DorisDisaggregatedClusterSpec{
			MetaService: dv1.MetaService{
				CommonSpec: dv1.CommonSpec{
					Replicas: pointer.Int32(1),
					Image:    "selectdb/doris.ms-ubuntu:latest",
				},
				FDB: dv1.FDB{
					Address: "127.0.0.1:4500",
				},
			},
		},
	}

	dms := &DisaggregatedMSController{}
	pts := dms.NewPodTemplateSpec(ddc, map[string]string{}, map[string]interface{}{})

	foundPodInfoMount := false
	for _, c := range pts.Spec.Containers {
		if c.Name != resource.DISAGGREGATED_MS_MAIN_CONTAINER_NAME {
			continue
		}
		for _, vm := range c.VolumeMounts {
			if vm.Name == resource.POD_INFO_VOLUME_NAME && vm.MountPath == resource.POD_INFO_PATH {
				foundPodInfoMount = true
				break
			}
		}
	}
	if !foundPodInfoMount {
		t.Fatalf("expected metaservice container to keep podinfo mount %q at %q", resource.POD_INFO_VOLUME_NAME, resource.POD_INFO_PATH)
	}
}
