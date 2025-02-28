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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"k8s.io/klog/v2"
)

func Map2Hash(m map[string]interface{}) string {
	//convert to json for the order in map is not fixed. but the json is sequential as alphabetic order.
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		klog.Errorf("Map2Hash json Marshal failed, err: %s", err.Error())
		return ""
	}

	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}
