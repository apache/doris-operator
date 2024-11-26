package hash

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_SetHashLabel(t *testing.T) {
	test := metav1.ObjectMeta{}
	labelName := "test.hash.label"
	labels := map[string]string{}
	labels = setHashLabel(labelName, labels, test)
	if _, ok := labels[labelName]; !ok {
		t.Errorf("setHashLabel not effect.")
	}
}
