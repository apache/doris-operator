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
package fe

import (
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
)

func Test_safeScaleDown(t *testing.T) {

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Errorf("Test_safeScaleDown NewManager failed, err=%s", err.Error())
	}

	fc := New(mgr.GetClient(), mgr.GetEventRecorderFor("test-fe"))

	dcrs := []*dorisv1.DorisCluster{
		{
			Spec: dorisv1.DorisClusterSpec{
				FeSpec: &dorisv1.FeSpec{
					BaseSpec: dorisv1.BaseSpec{
						Replicas: resource.GetInt32Pointer(3),
					},
				},
			},
		},
		{
			Spec: dorisv1.DorisClusterSpec{
				FeSpec: &dorisv1.FeSpec{
					BaseSpec: dorisv1.BaseSpec{
						Replicas: resource.GetInt32Pointer(2),
					},
				},
			},
		},
		{
			Spec: dorisv1.DorisClusterSpec{
				FeSpec: &dorisv1.FeSpec{
					BaseSpec: dorisv1.BaseSpec{
						Replicas: resource.GetInt32Pointer(4),
					},
				},
			},
		},
		{
			Spec: dorisv1.DorisClusterSpec{
				FeSpec: &dorisv1.FeSpec{
					BaseSpec: dorisv1.BaseSpec{
						Replicas: resource.GetInt32Pointer(1),
					},
				},
			},
		},
	}

	osts := []*appv1.StatefulSet{
		{
			Spec: appv1.StatefulSetSpec{
				Replicas: resource.GetInt32Pointer(3),
			},
		},
		{
			Spec: appv1.StatefulSetSpec{
				Replicas: resource.GetInt32Pointer(3),
			},
		},
		{
			Spec: appv1.StatefulSetSpec{
				Replicas: resource.GetInt32Pointer(5),
			},
		},
		{
			Spec: appv1.StatefulSetSpec{
				Replicas: resource.GetInt32Pointer(2),
			},
		},
	}

	res := []int32{3, 3, 4,2}
	for i := 0; i < len(dcrs); i++ {
		fc.safeScaleDown(dcrs[i], osts[i])
		if *dcrs[i].Spec.FeSpec.Replicas != res[i] {
			t.Errorf("Test_safeScaleDown failed, cluster fe replicas %d, expect result %d", *dcrs[i].Spec.FeSpec.Replicas, res[i])
		}
	}
}
