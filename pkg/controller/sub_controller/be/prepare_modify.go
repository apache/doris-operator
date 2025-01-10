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

	v1 "github.com/apache/doris-operator/api/doris/v1"
)

// prepareStatefulsetApply means Pre-operation and status control on the client side
func (be *Controller) prepareStatefulsetApply(ctx context.Context, dcr *v1.DorisCluster, oldStatus v1.ComponentStatus) error {

	// be rolling restart
	// check 1: be Phase is Available
	// check 2: be RestartTime is not empty and useful
	// check 3: be RestartTime different from old(This condition does not need to be checked here. If it is allowed to pass, it will be processed idempotent when applying sts.)
	if oldStatus.ComponentCondition.Phase == v1.Available && be.CheckRestartTimeAndInject(dcr, v1.Component_BE) {
		dcr.Status.BEStatus.ComponentCondition.Phase = v1.Restarting
	}

	//TODO check upgrade

	return nil
}
