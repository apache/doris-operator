// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package resource

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"testing"
)

func Test_NewLabels(t *testing.T) {
	larr := []Labels{{
		"test": "test",
	}}

	lres := []string{"test"}
	for i, l := range larr {
		t.Run("newLabels"+strconv.Itoa(i), func(t *testing.T) {
			lnew := NewLabels(l)
			if lnew["test"] != lres[i] {
				t.Errorf("newLabels failed, have not save exist values.")
			}
		})
	}
}

func Test_LabelAddFuncs(t *testing.T) {
	l1 := Labels{
		"test1": "test1",
	}
	l2 := Labels{
		"test2": "test2",
	}
	ltest := Labels{}
	ltest.Add("test1", (map[string]string)(l1)["test1"])
	ltest.AddLabel(l2)
	tKeys := []string{"test1", "test2"}
	for _, key := range tKeys {
		if v := ltest[key]; v == "" {
			t.Errorf("AddFuncs failed, not add value.")
		}
	}
}
func Test_NewAnnotations(t *testing.T) {
	aarr := []Annotations{{
		"test": "test",
	}}

	lres := []string{"test"}
	for i, a := range aarr {
		t.Run("newLabels"+strconv.Itoa(i), func(t *testing.T) {
			lnew := NewAnnotations(a)
			if lnew["test"] != lres[i] {
				t.Errorf("newAnnotations failed, have not save exist values.")
			}
		})
	}
}

func Test_AnnotationAddFuncs(t *testing.T) {
	a1 := Annotations{
		"test1": "test1",
	}
	a2 := Annotations{
		"test2": "test2",
	}
	atest := Annotations{}
	atest.Add("test1", (map[string]string)(a1)["test1"])
	atest.AddAnnotation(a2)
	tKeys := []string{"test1", "test2"}
	for _, key := range tKeys {
		if v := atest[key]; v == "" {
			t.Errorf("Annotations AddFuncs failed, not add value.")
		}
	}
}

func Test_mergeMetadata(t *testing.T) {
	new := metav1.ObjectMeta{
		ResourceVersion: "new_v1",
		Finalizers:      []string{"doris.new.finalizer"},
		Labels: Labels{
			"newtest1": "test1",
			"newtest2": "test2",
		},
		Annotations: Annotations{
			"newAnno1": "newAnno1",
			"newAnno2": "newAnno2",
		},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "DorisCluster",
			Name:       "test1",
		}},
	}
	old := metav1.ObjectMeta{
		ResourceVersion: "old_n1",
		Finalizers:      []string{"doris.old.finalizer"},
		Labels: Labels{
			"oldtest1": "oldtest1",
			"oldtest2": "oldtest2",
		},
		Annotations: Annotations{
			"oldtest1": "oldtest1",
			"oldtest2": "oldtest2",
		},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "DorisCluster",
			Name:       "test2",
		}},
	}

	MergeMetadata(&new, old)
	if new.ResourceVersion != "old_n1" ||
		len(new.Finalizers) != 2 ||
		len(new.Labels) != 4 ||
		len(new.OwnerReferences) != 2 ||
		len(new.Annotations) != 4 {
		t.Errorf("metadata merge failed.")
	}
}
