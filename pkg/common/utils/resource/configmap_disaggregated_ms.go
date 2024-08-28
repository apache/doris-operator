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
	"bytes"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// the ports key

const (
	BRPC_LISTEN_PORT = "brpc_listen_port"
)

func getDefaultDMSResolveKey(componentType mv1.ComponentType) string {
	switch componentType {
	case mv1.Component_MS:
		return MS_RESOLVEKEY
	case mv1.Component_RC:
		return RC_RESOLVEKEY
	default:
		klog.Infof("the componentType: %s have not default ResolveKey", componentType)
	}
	return ""
}

func ResolveDMSConfigMaps(configMaps []*corev1.ConfigMap, componentType mv1.ComponentType) (map[string]interface{}, error) {
	key := getDefaultDMSResolveKey(componentType)
	for _, configMap := range configMaps {
		if configMap == nil {
			continue
		}
		if value, ok := configMap.Data[key]; ok {
			viper.SetConfigType("properties")
			viper.ReadConfig(bytes.NewBuffer([]byte(value)))
			return viper.AllSettings(), nil
		}
	}
	return make(map[string]interface{}), nil
}
