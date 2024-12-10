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

package set

type SetString struct {
	m map[string]struct{}
}

func NewSetString(strs ...string) *SetString {
	ss := &SetString{
		m: make(map[string]struct{}),
	}

	for _, str := range strs {
		ss.Add(str)
	}

	return ss
}
func (ss *SetString) Add(str string) {
	ss.m[str] = struct{}{}
}

func (ss *SetString) Del(str string) {
	delete(ss.m, str)
}

func (ss *SetString) Find(str string) bool {
	_, ok := ss.m[str]
	return ok
}

func (ss *SetString) Get(str string) bool {
	if _, ok := ss.m[str]; ok {
		return true
	}
	return false
}
