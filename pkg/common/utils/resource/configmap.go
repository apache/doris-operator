package resource

import (
	"bytes"
	"errors"
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
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
)

const BROKER_IPC_PORT = "broker_ipc_port"
const GRACE_SHUTDOWN_WAIT_SECONDS = "grace_shutdown_wait_seconds"

const ENABLE_FQDN = "enable_fqdn_mode"

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
}

func IsFQDN(config map[string]interface{}) bool {
	// not use configmap
	if len(config) == 0 {
		return true
	}

	// use configmap
	if v, ok := config[ENABLE_FQDN]; ok {
		return v.(string) == "true"
	} else {
		return false
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
