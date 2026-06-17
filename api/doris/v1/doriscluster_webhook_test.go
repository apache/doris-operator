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

func TestDorisClusterRejectsAdminManagementUser(t *testing.T) {
	cluster := &DorisCluster{
		Spec: DorisClusterSpec{
			AdminUser: &AdminUser{Name: "admin"},
		},
	}

	if _, err := cluster.ValidateCreate(context.Background(), cluster); err == nil {
		t.Fatal("expected admin management user to be rejected on create")
	}

	replicas := int32(3)
	cluster.Spec.FeSpec = &FeSpec{BaseSpec: BaseSpec{Replicas: &replicas}}
	if _, err := cluster.ValidateUpdate(context.Background(), cluster, cluster); err == nil {
		t.Fatal("expected admin management user to be rejected on update")
	}
}

func TestDorisClusterAllowsNonAdminManagementUser(t *testing.T) {
	cluster := &DorisCluster{
		Spec: DorisClusterSpec{
			AdminUser: &AdminUser{Name: "doris_operator"},
		},
	}

	if _, err := cluster.ValidateCreate(context.Background(), cluster); err != nil {
		t.Fatalf("expected non-admin management user to be allowed on create: %v", err)
	}

	replicas := int32(3)
	cluster.Spec.FeSpec = &FeSpec{BaseSpec: BaseSpec{Replicas: &replicas}}
	if _, err := cluster.ValidateUpdate(context.Background(), cluster, cluster); err != nil {
		t.Fatalf("expected non-admin management user to be allowed on update: %v", err)
	}
}
