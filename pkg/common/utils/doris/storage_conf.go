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
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// StorageRootPathInfo represents the parsed storage path information.
type StorageRootPathInfo struct {
	MountPath      string
	VolumeResource string
	Medium         string
}

// TransformStorage transforms a string of storage paths into a slice of StorageRootPathInfo.
// Currently, both of three following formats are supported(as doris`s be.conf), remote cache is the
// local path :
//
//	format 1:   /home/disk1/palo.HDD,50
//	format 2:   /home/disk1/palo,medium:ssd,capacity:50
//
// remote storage :
//
//	format 1:   /home/disk/palo/cache,medium:remote_cache,capacity:50
func TransformStorage(configPath string) ([]StorageRootPathInfo, error) {
	if configPath == "" {
		return []StorageRootPathInfo{}, nil
	}

	// Separate multiple paths by ';'
	pathVec := strings.Split(configPath, ";")

	// Remove empty elements
	var cleanPaths []string
	for _, p := range pathVec {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			cleanPaths = append(cleanPaths, trimmed)
		}
	}

	if len(cleanPaths) == 0 {
		return []StorageRootPathInfo{}, nil
	}

	// Check path uniqueness
	uniquePaths := make(map[string]bool)
	result := make([]StorageRootPathInfo, 0, len(cleanPaths))

	for _, item := range cleanPaths {
		info, err := parseSinglePath(item)
		if err != nil {
			return nil, fmt.Errorf("TransformStorage error parsing path %s: %w", item, err)
		}

		// Check path uniqueness
		if uniquePaths[info.MountPath] {
			return nil, fmt.Errorf("TransformStorage duplicate paths found : %s", info.MountPath)
		}
		uniquePaths[info.MountPath] = true
		result = append(result, info)
	}

	return result, nil
}

// Resolving a single storage path
func parseSinglePath(pathConfig string) (StorageRootPathInfo, error) {
	info := StorageRootPathInfo{
		Medium: "HDD", // default Medium is HDD
	}

	// split by ','
	parts := strings.Split(pathConfig, ",")
	if len(parts) == 0 || parts[0] == "" {
		return info, fmt.Errorf("invalid storage path format: %s", pathConfig)
	}

	// path
	rawPath := strings.TrimSpace(parts[0])
	// remove suffix '/' , if exists
	cleanPath := strings.TrimRight(rawPath, "/")
	if cleanPath == "" || cleanPath[0] != '/' {
		return info, fmt.Errorf("the path must start with '/', but got an err path: %s", rawPath)
	}

	// Check path suffix as storage medium
	extension := filepath.Ext(cleanPath)
	if extension != "" {
		// Remove the leading '.' and uppercase
		mediumType := strings.ToUpper(extension[1:])
		// Only set if this is a valid storage type
		if mediumType == "HDD" || mediumType == "SSD" || mediumType == "REMOTE_CACHE" {
			info.Medium = mediumType
			cleanPath = cleanPath[:len(cleanPath)-len(extension)]
		}
	}

	info.MountPath = cleanPath

	// Handling other attributes (e.g., capacity, medium)
	for i := 1; i < len(parts); i++ {
		propStr := strings.TrimSpace(parts[i])
		if propStr == "" {
			continue
		}

		// Parsing property format: 'property:value' or directly the value (indicating 'capacity')
		var property, value string
		if colonPos := strings.Index(propStr, ":"); colonPos != -1 {
			property = strings.ToUpper(strings.TrimSpace(propStr[:colonPos]))
			value = strings.TrimSpace(propStr[colonPos+1:])
		} else {
			property = "CAPACITY"
			value = strings.TrimSpace(propStr)
		}

		// Storage properties configuration
		switch property {
		case "CAPACITY":
			// Verify that the capacity is a non-negative integer
			if value == "" {
				return info, fmt.Errorf("parseSinglePathcapacity value cannot be empty")
			}
			num, err := strconv.ParseInt(value, 10, 64)
			if err != nil || num < 0 {
				return info, fmt.Errorf("parseSinglePath invalid capacity value: %s", value)
			}
			// Convert to Kubernetes quantity format (e.g., "10Gi")
			info.VolumeResource = fmt.Sprintf("%dGi", num)
		case "MEDIUM":
			mediumType := strings.ToUpper(value)
			if mediumType != "HDD" && mediumType != "SSD" && mediumType != "REMOTE_CACHE" {
				return info, fmt.Errorf("parseSinglePath invalid storage media type: %s", value)
			}
			// The medium parameter takes precedence over path extension.
			info.Medium = mediumType
		default:
			return info, fmt.Errorf("parseSinglePath Invalid attribute: %s", property)
		}
	}

	return info, nil
}
