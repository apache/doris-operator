package resource

import (
	"bytes"
	"errors"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// the ports key

const (
	MS_BRPC_LISTEN_PORT = "ms_brpc_listen_port"
	RC_BRPC_LISTEN_PORT = "rc_brpc_listen_port"
	BRPC_LISTEN_PORT    = "brpc_listen_port"
)

// the default ResolveKey
const (
	MS_RESOLVEKEY = "selectdb_cloud.conf"
	RC_RESOLVEKEY = "selectdb_cloud.conf"
)

// defMap the default port about abilities.
var defDMSMap = map[string]int32{
	MS_BRPC_LISTEN_PORT: 5000,
	RC_BRPC_LISTEN_PORT: 5001,
}

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
	err := errors.New("not fund configmap ResolveKey: " + key)
	return nil, err
}
