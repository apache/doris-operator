package resource

import (
	"strconv"
)

// getPort get ports from config file.
func GetPort(config map[string]interface{}, key string) int32 {
	if v, ok := config[key]; ok {
		if port, err := strconv.ParseInt(v.(string), 10, 32); err == nil {
			return int32(port)
		}
	}
	return defMap[key]
}
