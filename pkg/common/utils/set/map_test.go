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

func Test_Map2Hash(t *testing.T) {
	tests := []struct {
		name     string
		mapInput map[string]interface{}
		expected string
	}{
		{
			name:     "empty map",
			mapInput: map[string]interface{}{},
			expected: "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a", // sha256 hash of an empty map
		},
		{
			name:     "map with one key-value",
			mapInput: map[string]interface{}{"key1": "value1"},
			expected: "9874854240b45b4bdbf43fca6110bafce8525aedbeca5babaee0cb137d9a7868", // sha256 hash of {"key1":"value1"}
		},
		{
			name:     "map with multiple key-value pairs",
			mapInput: map[string]interface{}{"key1": "value1", "key2": "value2"},
			expected: "b734413c644ec49f6a7c07d88b267244582d6422d89eee955511f6b3c0dcb0f2", // sha256 hash of {"key1":"value1", "key2":"value2"}
		},
		{
			name:     "map with different key order",
			mapInput: map[string]interface{}{"key2": "value2", "key1": "value1"},
			expected: "b734413c644ec49f6a7c07d88b267244582d6422d89eee955511f6b3c0dcb0f2", // sha256 hash of {"key2":"value2", "key1":"value1"} should match previous test case
		},
		{
			name:     "map with different values",
			mapInput: map[string]interface{}{"key1": "value1", "key2": "value3"},
			expected: "d28ff006564cb178d72850c11e58a8131f40d66418ef4acb6a579cb3c6d1d379", // sha256 hash of {"key1":"value1", "key2":"value3"}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Map2Hash(tt.mapInput)
			if got != tt.expected {
				t.Errorf("Map2Hash() = %v, want %v", got, tt.expected)
			}
		})
	}
}
