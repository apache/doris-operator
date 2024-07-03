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

// the default ResolveKey
const (
	MS_RESOLVEKEY = "selectdb_cloud.conf"
	RC_RESOLVEKEY = "selectdb_cloud.conf"
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
