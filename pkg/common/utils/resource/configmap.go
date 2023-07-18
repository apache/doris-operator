package resource

import (
	"bytes"
	v1 "github.com/selectdb/doris-operator/api/v1"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
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
}

func ResolveConfigMap(configMap *corev1.ConfigMap, key string) (map[string]interface{}, error) {
	res := make(map[string]interface{})
	data := configMap.Data
	if _, ok := data[key]; !ok {
		return res, nil
	}

	value, _ := data[key]

	viper.SetConfigType("properties")
	viper.ReadConfig(bytes.NewBuffer([]byte(value)))

	return viper.AllSettings(), nil
}

func MountConfigMap(cmInfo v1.ConfigMapInfo) (corev1.Volume, corev1.VolumeMount) {
	var volume corev1.Volume
	var volumeMount corev1.VolumeMount

	if cmInfo.ConfigMapName != "" && cmInfo.ResolveKey != "" {
		volume = corev1.Volume{
			Name: cmInfo.ConfigMapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cmInfo.ConfigMapName,
					},
				},
			},
		}
		volumeMount = corev1.VolumeMount{
			Name:      cmInfo.ConfigMapName,
			MountPath: "/etc/doris",
		}
	}

	return volume, volumeMount
}
