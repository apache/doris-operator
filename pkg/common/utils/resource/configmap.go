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
	"errors"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// the fe ports key
const (
	HTTP_PORT     = "http_port"
	RPC_PORT      = "rpc_port"
	QUERY_PORT    = "query_port"
	EDIT_LOG_PORT = "edit_log_port"
)

// the cn or be ports key
const (
	THRIFT_PORT            = "thrift_port"
	BE_PORT                = "be_port"
	WEBSERVER_PORT         = "webserver_port"
	HEARTBEAT_SERVICE_PORT = "heartbeat_service_port"
	BRPC_PORT              = "brpc_port"
)

// the default ResolveKey
const (
	FE_RESOLVEKEY     = "fe.conf"
	BE_RESOLVEKEY     = "be.conf"
	CN_RESOLVEKEY     = "be.conf"
	BROKER_RESOLVEKEY = "apache_hdfs_broker.conf"
	MS_RESOLVEKEY     = "doris_cloud.conf"
	DefaultMsToken    = "greedisgood9999"
	DefaultMsTokenKey = "http_token"
)

const ARROW_FLIGHT_SQL_PORT = "arrow_flight_sql_port"
const BRPC_LISTEN_PORT = "brpc_listen_port"

const BROKER_IPC_PORT = "broker_ipc_port"
const GRACE_SHUTDOWN_WAIT_SECONDS = "grace_shutdown_wait_seconds"

const ENABLE_FQDN = "enable_fqdn_mode"
const START_MODEL_FQDN = "FQDN"
const START_MODEL_IP = "IP"

// defMap the default port about abilities.
var defMap = map[string]int32{
	HTTP_PORT:              8030,
	RPC_PORT:               9020,
	QUERY_PORT:             9030,
	EDIT_LOG_PORT:          9010,
	THRIFT_PORT:            9060,
	BE_PORT:                9060,
	WEBSERVER_PORT:         8040,
	HEARTBEAT_SERVICE_PORT: 9050,
	BRPC_PORT:              8060,
	BROKER_IPC_PORT:        8000,
	BRPC_LISTEN_PORT:       5000,
	ARROW_FLIGHT_SQL_PORT:  -1,
}

// GetStartMode return fe host type, fqdn(host) or ip, from 'fe.conf' enable_fqdn_mode
func GetStartMode(config map[string]interface{}) string {
	// not use configmap
	if len(config) == 0 {
		return START_MODEL_FQDN
	}

	// use configmap
	v, ok := config[ENABLE_FQDN]
	if ok && v.(string) == "true" {
		return START_MODEL_FQDN
	} else {
		return START_MODEL_IP
	}

}

func GetDefaultPort(key string) int32 {
	return defMap[key]
}

func getDefaultResolveKey(componentType dorisv1.ComponentType) string {
	switch componentType {
	case dorisv1.Component_FE:
		return FE_RESOLVEKEY
	case dorisv1.Component_BE:
		return BE_RESOLVEKEY
	case dorisv1.Component_CN:
		return CN_RESOLVEKEY
	case dorisv1.Component_Broker:
		return BROKER_RESOLVEKEY
	default:
		klog.Infof("the componentType: %s have not default ResolveKey", componentType)
	}
	return ""
}

func ResolveConfigMaps(configMaps []*corev1.ConfigMap, componentType dorisv1.ComponentType) (map[string]interface{}, error) {
	key := getDefaultResolveKey(componentType)
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
	err := errors.New("not fund configmap ResolveKey: " + key)
	return nil, err
}

func GetMountConfigMapInfo(c dorisv1.ConfigMapInfo) (finalConfigMaps []dorisv1.MountConfigMapInfo) {

	if c.ConfigMapName != "" {
		finalConfigMaps = append(
			finalConfigMaps,
			dorisv1.MountConfigMapInfo{
				ConfigMapName: c.ConfigMapName,
				MountPath:     "",
			},
		)
	}
	finalConfigMaps = append(finalConfigMaps, c.ConfigMaps...)

	return finalConfigMaps
}

// getDorisCoreConfigMapName return a configmap`s name include doris configurations such as fe.conf/be.conf
func getDorisCoreConfigMapName(dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType) string {
	var cmInfo dorisv1.ConfigMapInfo
	switch componentType {
	case dorisv1.Component_FE:
		cmInfo = dcr.Spec.FeSpec.ConfigMapInfo
	case dorisv1.Component_BE:
		cmInfo = dcr.Spec.BeSpec.ConfigMapInfo
	case dorisv1.Component_CN:
		cmInfo = dcr.Spec.CnSpec.ConfigMapInfo
	case dorisv1.Component_Broker:
		cmInfo = dcr.Spec.BrokerSpec.ConfigMapInfo
	default:
		klog.Infof("getCoreCmName: the componentType: %s have not default ResolveKey", componentType)
	}

	maps := GetMountConfigMapInfo(cmInfo)
	for i := range maps {
		if maps[i].MountPath == "" || maps[i].MountPath == ConfigEnvPath {
			return maps[i].ConfigMapName
		}
	}
	return ""
}

func GetDorisCoreConfigMapNames(dcr *dorisv1.DorisCluster) map[dorisv1.ComponentType]string {
	dorisCoreConfigMaps := map[dorisv1.ComponentType]string{}
	if dcr.Spec.FeSpec != nil {
		if cm := getDorisCoreConfigMapName(dcr, dorisv1.Component_FE); cm != "" {
			dorisCoreConfigMaps[dorisv1.Component_FE] = cm
		}
	}

	if dcr.Spec.BeSpec != nil {
		if cm := getDorisCoreConfigMapName(dcr, dorisv1.Component_BE); cm != "" {
			dorisCoreConfigMaps[dorisv1.Component_BE] = cm
		}
	}

	if dcr.Spec.CnSpec != nil {
		if cm := getDorisCoreConfigMapName(dcr, dorisv1.Component_CN); cm != "" {
			dorisCoreConfigMaps[dorisv1.Component_CN] = cm
		}
	}

	if dcr.Spec.BrokerSpec != nil {
		if cm := getDorisCoreConfigMapName(dcr, dorisv1.Component_Broker); cm != "" {
			dorisCoreConfigMaps[dorisv1.Component_Broker] = cm
		}
	}

	return dorisCoreConfigMaps
}
