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

package v1

import (
	"context"
	"testing"
)

func TestDorisDisaggregatedClusterValidateFEReplicas(t *testing.T) {
	replicas := int32(1)
	electionNumber := int32(2)
	ddc := &DorisDisaggregatedCluster{
		Spec: DorisDisaggregatedClusterSpec{
			FeSpec: FeSpec{
				ElectionNumber: &electionNumber,
				CommonSpec: CommonSpec{
					Replicas: &replicas,
				},
			},
		},
	}

	if _, err := ddc.ValidateCreate(context.Background(), ddc); err == nil {
		t.Fatal("expected create validation to reject replicas smaller than electionNumber")
	}
	if _, err := ddc.ValidateUpdate(context.Background(), ddc, ddc); err == nil {
		t.Fatal("expected update validation to reject replicas smaller than electionNumber")
	}

	replicas = 2
	if _, err := ddc.ValidateCreate(context.Background(), ddc); err != nil {
		t.Fatalf("expected valid create to pass, got %v", err)
	}
	if _, err := ddc.ValidateUpdate(context.Background(), ddc, ddc); err != nil {
		t.Fatalf("expected valid update to pass, got %v", err)
	}
}
