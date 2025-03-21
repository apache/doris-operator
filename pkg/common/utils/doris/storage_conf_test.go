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

package doris

import (
	"reflect"
	"testing"
)

func TestResolveStorageRootPath(t *testing.T) {
	var empty []string
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		// Normal test
		{
			input: "",
			want:  empty,
		},
		{
			input: "/path1",
			want:  []string{"/path1"},
		},
		{
			input: "/path1;/path2",
			want:  []string{"/path1", "/path2"},
		},
		{
			input: "/home/disk1/doris.HDD,50",
			want:  []string{"/home/disk1/doris"},
		},
		{
			input: "/home/disk1/doris,medium:ssd,capacity:50",
			want:  []string{"/home/disk1/doris"},
		},
		{
			input: "/home/disk1/doris.SSD,100;/home/disk2/doris,medium:hdd,capacity:200",
			want:  []string{"/home/disk1/doris", "/home/disk2/doris"},
		},
		{
			input: "/home/disk1/doris/,capacity:50",
			want:  []string{"/home/disk1/doris"},
		},
		{
			input: "/home/disk1/doris.HDD,medium:ssd",
			want:  []string{"/home/disk1/doris"},
		},
		{
			input: "/home/disk1/doris,capacity:50;",
			want:  []string{"/home/disk1/doris"},
		},
		{
			input: " /home/disk1/doris , capacity : 50 ; /home/disk2/doris , medium : ssd ",
			want:  []string{"/home/disk1/doris", "/home/disk2/doris"},
		},

		{
			input: "/home/disk1/doris,capacity:50;/home/disk1/doris,medium:ssd",
			want:  []string{"/home/disk1/doris", "/home/disk1/doris"},
		},

		{
			input: ",capacity:50",
			want:  empty,
		},

		{
			input: "/home/disk1/doris/,unknown:value",
			want:  []string{"/home/disk1/doris"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStorageRootPath(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("input: %s, got %v, Expectation %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetNameOfEachPath(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			input: []string{},
			want:  []string{},
		},
		{
			input: []string{"", "", "", ""},
			want:  []string{"", "", "", ""},
		},
		{
			input: []string{"", ""},
			want:  []string{"", ""},
		},
		{
			input: []string{"/path1"},
			want:  []string{"path1"},
		},
		{
			input: []string{"/opt/doris/path1"},
			want:  []string{"path1"},
		},
		{
			input: []string{"/path1", "/path2"},
			want:  []string{"path1", "path2"},
		},
		{
			input: []string{"/home/disk1/doris", "/home/disk2/doris"},
			want:  []string{"doris", "disk2-doris"},
		},
		{
			input: []string{"/home/disk1/doris", "/home/disk1/doris"},
			want:  []string{"disk1-doris", "disk1-doris"},
		},
		{
			input: []string{"/home/disk1/doris", "/home/disk1/doris", "/home/disk2/doris"},
			want:  []string{"disk1-doris", "disk1-doris", "disk2-doris"},
		},
		{
			input: []string{"/home/disk1/doris", "/home/disk2/doris", "/home/disk3/doris"},
			want:  []string{"doris", "disk2-doris", "disk3-doris"},
		},
		{
			input: []string{"/home/disk1/doris", "/home/disk1/doris/subdir"},
			want:  []string{"doris", "subdir"},
		},
		{
			input: []string{"/home/disk1/doris", "/home/disk1/doris/subdir", "/home/disk1/doris/subdir/subsubdir"},
			want:  []string{"doris", "subdir", "subsubdir"},
		},
		{
			input: []string{"/home/disk1/doris", "/home/disk1/doris/subdir", "/home/disk1/doris/subdir/subsubdir", "/home/disk1/doris/subdir/subsubdir"},
			want:  []string{"doris", "subdir", "subdir-subsubdir", "subdir-subsubdir"},
		},
		{
			input: []string{"/home/disk1/doris", "/home/disk1/doris", "/home/disk2/doris"},
			want:  []string{"disk1-doris", "disk1-doris", "disk2-doris"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetNameOfEachPath(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("input: %v, got %v, Expectation %v", tt.input, got, tt.want)
			}
		})
	}
}
