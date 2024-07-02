package resource

import (
	"strconv"
	"strings"
)

// GetPort retrieves the port number associated with the given key from the provided configuration map.
//
// Parameters:
//
//	config map[string]interface{}: A map containing configuration information, where the keys are strings and the values are of interface type.
//	key string: The key name for which the port number is to be retrieved.
//
// Returns:
//
//	int32: The port number corresponding to the specified key, or a default value if not found.
//
// Notes:
//  1. If the key name contains BRPC_LISTEN_PORT, it will be used as the key name for lookup.
//  2. If the specified key does not exist in the configuration map, or if the value associated with the key cannot be parsed as a 32-bit integer, a default value will be returned.
//  3. The default value is retrieved from either the defMap or the defDMSMap. If the key does not exist in defMap, it will be retrieved from defDMSMap.
func GetPort(config map[string]interface{}, key string) int32 {
	queryKey := key
	if strings.Contains(queryKey, BRPC_LISTEN_PORT) {
		queryKey = BRPC_LISTEN_PORT
	}
	if v, ok := config[queryKey]; ok {
		if port, err := strconv.ParseInt(v.(string), 10, 32); err == nil {
			return int32(port)
		}
	}
	if i, ok := defMap[queryKey]; ok {
		return i
	}
	return defDMSMap[key]
}

// GetTerminationGracePeriodSeconds get grace_shutdown_wait_seconds from config file.
func GetTerminationGracePeriodSeconds(config map[string]interface{}) int64 {
	if v, ok := config[GRACE_SHUTDOWN_WAIT_SECONDS]; ok {
		if seconds, err := strconv.ParseInt(v.(string), 10, 64); err == nil {
			return int64(seconds)
		}
	}

	return 0
}
