package ms

import "github.com/selectdb/doris-operator/pkg/common/utils/resource"

// defConfMap the default port for MS when not use configmap.
var defConfMap = map[string]int32{
	resource.BRPC_LISTEN_PORT: 5000,
}
