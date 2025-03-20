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
	"strings"
)

// ResolveStorageRootPath transforms a string of storage paths into a slice of StorageRootPathInfo.
func ResolveStorageRootPath(configPath string) []string {
	var res []string

	if configPath == "" {
		return res
	}

	// Separate multiple paths by ';'
	configPathSplit := strings.Split(configPath, ";")

	// Remove empty elements
	for _, c := range configPathSplit {
		if path := parseSinglePath(c); path != "" {
			res = append(res, path)
		}
	}

	return res
}

// Resolving a single storage path
func parseSinglePath(pathConfig string) string {
	if pathConfig == "" {
		return ""
	}
	path := strings.Split(strings.Split(pathConfig, ".")[0], ",")[0]
	path = strings.TrimSpace(path)
	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	return path
}

// GetNameOfEachPath is used to parse a set of paths to obtain unique and concise names for each path.
// If the paths are repeated, the returned names may also be repeated.
// And the order of each name in the array is consistent with the input paths.
// For example:
//
//	["/path1"] >> ["path1"]
//	["/opt/doris/path1"] >> ["path1"]
//	["/path1", "/path2"] >> ["path1", "path2"]
//	["/home/disk1/doris", "/home/disk2/doris"] >> ["disk1-doris", "disk2-doris"]
//	["/home/disk1/doris", "/home/disk1/doris"] >> ["doris", "doris"]
//	["/home/disk1/doris", "/home/disk1/doris", "/home/disk2/doris"] >> ["disk1-doris", "disk1-doris", "disk2-doris"]
func GetNameOfEachPath(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}

	temp := make([][]string, len(paths))
	for i, path := range paths {
		if strings.HasPrefix(path, "/") {
			path = path[1:]
		}
		temp[i] = strings.Split(path, "/")
	}

	if len(paths) == 1 {
		return []string{temp[0][len(temp[0])-1]}
	}

	var res []string
	i := 0
	for true {
		current := temp[0][i]
		for j := range temp {
			if current != temp[j][i] || len(temp[j]) == i+1 {
				goto LOOP
			}
		}
		i++
	}

LOOP:
	for _, path := range temp {
		if i > len(path) {
			res = append(res, "")
			continue
		}
		resPathSplit := path[i:]
		res = append(res, strings.Join(resPathSplit, "-"))
	}

	return res
}
