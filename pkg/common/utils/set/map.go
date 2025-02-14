package set

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"k8s.io/klog/v2"
)

func CompareMaps(map1, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}

	for key, value1 := range map1 {
		value2, exists := map2[key]
		if !exists || value1 != value2 {
			return false
		}
	}

	return true
}

func Map2Hash(m map[string]interface{}) string {
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		klog.Errorf("Map2Hash json Marshal failed, err: %s", err.Error())
		return ""
	}

	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}
