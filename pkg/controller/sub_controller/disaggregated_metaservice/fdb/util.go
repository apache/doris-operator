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
	"github.com/FoundationDB/fdb-kubernetes-operator/api/v1beta2"
	"k8s.io/klog/v2"
	"strings"
)

const (
	DefaultFDBImage        = "selectdb/foundationdb:7.1.38"
	DefaultFDBSidecarImage = "selectdb/foundationdb-kubernetes-sidecar:7.1.36-1"
)

// use ":" as IPS to split image as baseimage and version.
func imageSplit(image string) (baseImage, tag string, err error) {
	isa := strings.Split(image, ":")
	if len(isa) == 0 {
		err = errors.New(fmt.Sprintf("the image = %s format is not provided. please reference docker format.", image))
		return
	}

	baseImage = isa[0]
	tag = isa[1]
	return
}

func fdbImageOverride(image string) (v1beta2.ContainerOverrides, error) {
	if image == "" {
		image = DefaultFDBImage
	}

	return newContainerOverride(image)
}

func newContainerOverride(image string) (v1beta2.ContainerOverrides, error) {
	if image == "" {
		return v1beta2.ContainerOverrides{}, errors.New("image is empty")
	}

	bi, tag, err := imageSplit(image)
	if err != nil {
		klog.Infof("disaggregatedFDBController split config image error, err=%s", err.Error())
		return v1beta2.ContainerOverrides{}, err

	}

	return v1beta2.ContainerOverrides{
		ImageConfigs: []v1beta2.ImageConfig{
			v1beta2.ImageConfig{
				BaseImage: bi,
				Tag:       tag,
			},
		},
	}, nil
}

func fdbSidecarImageOverride(image string) (v1beta2.ContainerOverrides, error) {
	if image == "" {
		image = DefaultFDBSidecarImage
	}

	return newContainerOverride(image)
}
