package resource

import (
	"strconv"
)

// getPort get ports from config file.
func GetPort(config map[string]interface{}, key string) int32 {
	return GetPortFromMap(config, defMap, key)
}

func GetPortFromMap(config map[string]interface{}, defaultConfig map[string]int32, key string) int32 {
	if v, ok := config[key]; ok {
		if port, err := strconv.ParseInt(v.(string), 10, 32); err == nil {
			return int32(port)
		}
	}
	return defaultConfig[key]
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
