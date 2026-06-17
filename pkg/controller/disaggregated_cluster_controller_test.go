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

package controller

import (
	"context"
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeDisaggregatedSubController struct {
	name string
}

func (f fakeDisaggregatedSubController) Sync(ctx context.Context, obj client.Object) error {
	return nil
}

func (f fakeDisaggregatedSubController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	return true, nil
}

func (f fakeDisaggregatedSubController) GetControllerName() string {
	return f.name
}

func (f fakeDisaggregatedSubController) UpdateComponentStatus(obj client.Object) error {
	return nil
}

var _ sc.DisaggregatedSubController = fakeDisaggregatedSubController{}

func TestReorganizeStatusConsidersMetaServiceHealth(t *testing.T) {
	tests := []struct {
		name              string
		metaServiceStatus dv1.MetaServiceStatus
		wantHealth        dv1.Health
	}{
		{
			name: "meta service not fully ready makes cluster yellow",
			metaServiceStatus: dv1.MetaServiceStatus{
				AvailableStatus: dv1.Available,
				Phase:           dv1.Reconciling,
			},
			wantHealth: dv1.Yellow,
		},
		{
			name: "meta service unavailable makes cluster red",
			metaServiceStatus: dv1.MetaServiceStatus{
				AvailableStatus: dv1.UnAvailable,
				Phase:           dv1.Reconciling,
			},
			wantHealth: dv1.Red,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ddc := &dv1.DorisDisaggregatedCluster{}
			ddc.Status.MetaServiceStatus = tt.metaServiceStatus
			ddc.Status.FEStatus.AvailableStatus = dv1.Available
			ddc.Status.FEStatus.Phase = dv1.Ready
			ddc.Status.ClusterHealth.CGCount = 1
			ddc.Status.ClusterHealth.CGAvailableCount = 1

			reconciler := &DisaggregatedClusterReconciler{
				Scs: map[string]sc.DisaggregatedSubController{
					"fake": fakeDisaggregatedSubController{name: "fake"},
				},
			}

			_, err := reconciler.reorganizeStatus(ddc)
			if err != nil {
				t.Fatalf("reorganizeStatus returned error: %v", err)
			}
			if ddc.Status.ClusterHealth.Health != tt.wantHealth {
				t.Fatalf("health = %s, want %s", ddc.Status.ClusterHealth.Health, tt.wantHealth)
			}
		})
	}
}
