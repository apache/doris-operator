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

package computeclusters

import (
	"regexp"
	"testing"
)

func Test_Regex(t *testing.T) {
	tns := []string{"test", "test_name", "test_", "test1", "testNa", "1test"}
	rns := []bool{true, true, false, true, true, false}
	for i, n := range tns {
		res, err := regexp.Match(compute_cluster_name_regex, []byte(n))
		if err != nil && res != rns[i] {
			t.Errorf("name %s not match regex %s, err=%s, match result %t", n, compute_cluster_name_regex, err.Error(), res)
		}
	}
}
