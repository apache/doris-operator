package computegroups

import (
	"regexp"
	"testing"
)

func Test_Regex(t *testing.T) {
	tns := []string{"test", "test_name", "test_", "test1", "testNa", "1test"}
	rns := []bool{true, true, false, true, true, false}
	for i, n := range tns {
		res, err := regexp.Match(compute_group_name_regex, []byte(n))
		if err != nil && res != rns[i] {
			t.Errorf("name %s not match regex %s, err=%s, match result %t", n, compute_group_name_regex, err.Error(), res)
		}
	}
}
