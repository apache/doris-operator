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

func TestTransformStorage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []StorageRootPathInfo
		wantErr bool
	}{
		// Normal test
		{
			input: "",
			want:  []StorageRootPathInfo{},
		},
		{
			input: "/path1",
			want: []StorageRootPathInfo{
				{MountPath: "/path1", Medium: "HDD"},
			},
		},
		{
			input: "/path1;/path2",
			want: []StorageRootPathInfo{
				{MountPath: "/path1", Medium: "HDD"},
				{MountPath: "/path2", Medium: "HDD"},
			},
		},
		{
			input: "/home/disk1/palo.HDD,50",
			want: []StorageRootPathInfo{
				{MountPath: "/home/disk1/palo", VolumeResource: "50Gi", Medium: "HDD"},
			},
		},
		{
			input: "/home/disk1/palo,medium:ssd,capacity:50",
			want: []StorageRootPathInfo{
				{MountPath: "/home/disk1/palo", VolumeResource: "50Gi", Medium: "SSD"},
			},
		},
		{
			input: "/home/disk1/palo.SSD,100;/home/disk2/palo,medium:hdd,capacity:200",
			want: []StorageRootPathInfo{
				{MountPath: "/home/disk1/palo", VolumeResource: "100Gi", Medium: "SSD"},
				{MountPath: "/home/disk2/palo", VolumeResource: "200Gi", Medium: "HDD"},
			},
		},
		{
			input: "/home/disk1/palo/,capacity:50",
			want: []StorageRootPathInfo{
				{MountPath: "/home/disk1/palo", VolumeResource: "50Gi", Medium: "HDD"},
			},
		},
		{
			input: "/home/disk1/palo.HDD,medium:ssd",
			want: []StorageRootPathInfo{
				{MountPath: "/home/disk1/palo", Medium: "SSD"},
			},
		},
		{
			input: "/home/disk1/palo,capacity:50;",
			want: []StorageRootPathInfo{
				{MountPath: "/home/disk1/palo", VolumeResource: "50Gi", Medium: "HDD"},
			},
		},
		{
			input: " /home/disk1/palo , capacity : 50 ; /home/disk2/palo , medium : ssd ",
			want: []StorageRootPathInfo{
				{MountPath: "/home/disk1/palo", VolumeResource: "50Gi", Medium: "HDD"},
				{MountPath: "/home/disk2/palo", Medium: "SSD"},
			},
		},
		{
			input: "/home/disk1/cache,medium:remote_cache,capacity:50",
			want: []StorageRootPathInfo{
				{MountPath: "/home/disk1/cache", VolumeResource: "50Gi", Medium: "REMOTE_CACHE"},
			},
		},

		// Error Condition Testing
		{
			input:   "home/disk1/palo,capacity:50",
			wantErr: true,
		},
		{
			input:   "/home/disk1/palo,capacity:-10",
			wantErr: true,
		},
		{
			input:   "/home/disk1/palo,capacity:abc",
			wantErr: true,
		},
		{
			input:   "/home/disk1/palo,medium:invalid",
			wantErr: true,
		},
		{
			input:   "/home/disk1/palo,capacity:50;/home/disk1/palo,medium:ssd",
			wantErr: true,
		},
		{
			input:   "/home/disk1/palo,unknown:value",
			wantErr: true,
		},
		{
			input:   ",capacity:50",
			wantErr: true,
		},
		{
			input:   ";",
			want:    []StorageRootPathInfo{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformStorage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TransformStorage() error = %v, Expectation Error = %v", err, tt.wantErr)
				return
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseStorageRootPath() = %v, Expectation = %v", got, tt.want)
			}
		})
	}
}

// 测试单个路径解析
func TestParseSinglePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    StorageRootPathInfo
		wantErr bool
	}{
		{
			input: "/home/data",
			want:  StorageRootPathInfo{MountPath: "/home/data", Medium: "HDD"},
		},
		{
			input: "/home/data.HDD",
			want:  StorageRootPathInfo{MountPath: "/home/data", Medium: "HDD"},
		},
		{
			input: "/home/data.SSD",
			want:  StorageRootPathInfo{MountPath: "/home/data", Medium: "SSD"},
		},
		{
			input: "/home/data,50",
			want:  StorageRootPathInfo{MountPath: "/home/data", VolumeResource: "50Gi", Medium: "HDD"},
		},
		{
			input: "/home/data,medium:ssd,capacity:100",
			want:  StorageRootPathInfo{MountPath: "/home/data", VolumeResource: "100Gi", Medium: "SSD"},
		},
		{
			input:   "relative/path",
			wantErr: true,
		},
		{
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSinglePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSinglePath() error = %v, Expectations Error = %v", err, tt.wantErr)
				return
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSinglePath() = %v, Expectation = %v", got, tt.want)
			}
		})
	}
}
