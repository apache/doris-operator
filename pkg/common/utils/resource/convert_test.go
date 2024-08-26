package resource

import "testing"

func Test_GetPort(t *testing.T) {
	config := map[string]interface{}{}
	config["http_port"] = 80030
	config["rpc_port"] = "test"
	tks := []string{"http_port", "rpc_port"}
	rk := map[string]int32{
		"http_port": 80030,
		"rpc_port":  9020,
	}
	for _, key := range tks {
		res := GetPort(config, key)
		if res != rk[key] {
			t.Errorf("")
		}
	}
}

func Test_GetTerminationGracePeriodSeconds(t *testing.T) {
	tests := []map[string]interface{}{
		{
			"grace_shutdown_wait_seconds": "60",
		},
		{
			"test_shutdown": "10",
		},
	}

	ress := []int64{60, 0}
	for i, test := range tests {
		res := GetTerminationGracePeriodSeconds(test)
		if res != ress[i] {
			t.Errorf("test TerminationGracePeriodSeconds failed, intput not equal expected")
		}
	}
}
