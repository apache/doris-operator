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
package be

import (
	"encoding/json"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	"testing"
	"time"
)

func Test_prepareStatefulsetApply(t *testing.T) {
	dcrJsonStr := `{
    "apiVersion": "doris.selectdb.com/v1",
        "kind": "DorisCluster",
        "metadata": {
        "name": "doriscluster-sample",
        "namespace": "default"
    },
    "spec": {
        "beSpec": {
			"baseSpec": {
            	"image": "selectdb/doris.be-ubuntu:2.1.6",
				"replicas": 3
			},
			"enableWorkloadGroup": true
        },
        "feSpec": {
            "image": "selectdb/doris.fe-ubuntu:2.1.6",
                "replicas": 3,
                "service": {
                "type": "NodePort"
            }
        }
    },
    "status":{
        "feStatus": {
            "componentCondition": {
				"phase": "Available"
			}
        },
        "beStatus": {
            "componentCondition": {
				"phase": "Available"
			}
        }
    }
}`
	bc := &Controller{}
	ntime := time.Now().Format(time.RFC3339)
	dcr := &v1.DorisCluster{}
	if err := json.Unmarshal([]byte(dcrJsonStr), dcr); err != nil {
		t.Errorf("test prepareStatefulsetApply unmarshal doriscluster failed, err=%s", err.Error())
	}
	dcr.Annotations = map[string]string{
		v1.BERestartAt: ntime,
	}
	oldStatus := v1.ComponentStatus{
		ComponentCondition: v1.ComponentCondition{
			Phase: v1.Available,
		},
	}
	bc.prepareStatefulsetApply(dcr, oldStatus)
}
