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

package fdb

import (
	"errors"
	"fmt"
	"strings"
)

// use ":" as IPS to split image as baseimage and version.
func imageSplit(image string) (baseImage, version string, err error) {
	isa := strings.Split(image, ":")
	if len(isa) == 0 {
		err = errors.New(fmt.Sprintf("the image = %s format is not provided. please reference docker format.", image))
		return
	}

	baseImage = isa[0]
	version = isa[1]
	return
}
