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

package set

import (
	"testing"
)

func TestArrayContains(t *testing.T) {
	type args struct {
		arr    []string
		target string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty nil array",
			args: args{
				arr:    nil,
				target: "1",
			},
			want: false,
		},
		{
			name: "string array contains",
			args: args{
				arr:    []string{"a", "b", "c"},
				target: "b",
			},
			want: true,
		},
		{
			name: "string array does not contain",
			args: args{
				arr:    []string{"a", "b", "c"},
				target: "d",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArrayContains(tt.args.arr, tt.args.target); got != tt.want {
				t.Errorf("ArrayContains() = %v, want %v", got, tt.want)
			}
		})
	}
}
