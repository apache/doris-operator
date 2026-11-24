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

package mysql

import "testing"

func Test_BuildSeqNumberToFrontendMap_FQDN(t *testing.T) {
	frontends := []*Frontend{
		{Host: "doris-fe-0.doris-fe-internal.default.svc.cluster.local", Role: FE_FOLLOWER_ROLE},
		{Host: "doris-fe-2.doris-fe-internal.default.svc.cluster.local", Role: FE_OBSERVE_ROLE},
	}
	result, err := BuildSeqNumberToFrontendMap(frontends, nil, "doris-fe")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[0] == nil || result[2] == nil {
		t.Fatalf("unexpected frontend map: %#v", result)
	}
}

func Test_BuildSeqNumberToFrontendMap_IP(t *testing.T) {
	frontends := []*Frontend{
		{Host: "10.0.0.1", Role: FE_FOLLOWER_ROLE},
		{Host: "10.0.0.3", Role: FE_FOLLOWER_ROLE},
	}
	podMap := map[string]string{
		"10.0.0.1": "doris-fe-1",
		"10.0.0.3": "doris-fe-3",
	}
	result, err := BuildSeqNumberToFrontendMap(frontends, podMap, "doris-fe")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[1] == nil || result[3] == nil {
		t.Fatalf("unexpected frontend map: %#v", result)
	}
}

func Test_BuildSeqNumberToFrontendMap_UnmatchedHostReturnsError(t *testing.T) {
	frontends := []*Frontend{{Host: "10.0.0.9", Role: FE_FOLLOWER_ROLE}}
	if _, err := BuildSeqNumberToFrontendMap(frontends, nil, "doris-fe"); err == nil {
		t.Fatal("expected an error for a host that does not belong to the statefulset")
	}
}

func Test_SeqNumberFromPodSignName(t *testing.T) {
	tests := []struct {
		podSignName string
		expected    int
		expectErr   bool
	}{
		{podSignName: "doris-fe-12", expected: 12},
		{podSignName: "doris-fe-12.doris-fe-internal.default.svc.cluster.local", expected: 12},
		{podSignName: "doris-fe", expectErr: true},
		{podSignName: "", expectErr: true},
	}
	for _, test := range tests {
		result, err := seqNumberFromPodSignName(test.podSignName, "doris-fe")
		if test.expectErr {
			if err == nil {
				t.Fatalf("expected error for %q", test.podSignName)
			}
			continue
		}
		if err != nil || result != test.expected {
			t.Fatalf("seqNumberFromPodSignName(%q)=%d, err=%v, expected %d", test.podSignName, result, err, test.expected)
		}
	}
}
