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

package resource

import (
	"strconv"
)

// getPort get ports from config file.
func GetPort(config map[string]interface{}, key string) int32 {
	if v, ok := config[key]; ok {
		if port, err := strconv.ParseInt(v.(string), 10, 32); err == nil {
			return int32(port)
		}
	}
	return defMap[key]
}

// GetTerminationGracePeriodSeconds get grace_shutdown_wait_seconds from config file.
func GetTerminationGracePeriodSeconds(config map[string]interface{}) int64 {
	if v, ok := config[GRACE_SHUTDOWN_WAIT_SECONDS]; ok {
		if seconds, err := strconv.ParseInt(v.(string), 10, 64); err == nil {
			return int64(seconds)
		}
	}

	return 0
}
