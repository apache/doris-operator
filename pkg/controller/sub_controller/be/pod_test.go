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
)

func Test_buildBEPodTemplateSpec(t *testing.T) {
	dcrJsonStr := `{
    "apiVersion": "doris.selectdb.com/v1",
        "kind": "DorisCluster",
        "metadata": {
        "annotations": {
            "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"doris.selectdb.com/v1\",\"kind\":\"DorisCluster\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/instance\":\"doriscluster-sample\",\"app.kubernetes.io/name\":\"doriscluster\"},\"name\":\"doriscluster-sample\",\"namespace\":\"default\"},\"spec\":{\"beSpec\":{\"image\":\"selectdb/doris.be-ubuntu:2.1.6\",\"replicas\":3},\"feSpec\":{\"image\":\"selectdb/doris.fe-ubuntu:2.1.6\",\"replicas\":3,\"service\":{\"type\":\"NodePort\"}}}}\n"
        },
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
    }
}`

	dcr := &v1.DorisCluster{}
	if err := json.Unmarshal([]byte(dcrJsonStr), dcr); err != nil {
		t.Errorf("the buildBEPodTemplateSpec unmarshal failed, err=%s", err.Error())
	}

	be := &Controller{}
	be.buildBEPodTemplateSpec(dcr, map[string]interface{}{})
}

func Test_buildBEPodTemplateSpecWithFEAffinity(t *testing.T) {
	dcrJsonStr := `{
    "apiVersion": "doris.selectdb.com/v1",
        "kind": "DorisCluster",
        "metadata": {
        "annotations": {
            "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"doris.selectdb.com/v1\",\"kind\":\"DorisCluster\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/instance\":\"doriscluster-sample\",\"app.kubernetes.io/name\":\"doriscluster\"},\"name\":\"doriscluster-sample\",\"namespace\":\"default\"},\"spec\":{\"beSpec\":{\"image\":\"selectdb/doris.be-ubuntu:2.1.6\",\"replicas\":3},\"feSpec\":{\"image\":\"selectdb/doris.fe-ubuntu:2.1.6\",\"replicas\":3,\"service\":{\"type\":\"NodePort\"}}}}\n"
        },
        "name": "doriscluster-sample",
        "namespace": "default"
    },
    "spec": {
        "beSpec": {
			"enableFeAffinity": true,
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
    }
}`

	dcr := &v1.DorisCluster{}
	if err := json.Unmarshal([]byte(dcrJsonStr), dcr); err != nil {
		t.Errorf("the buildBEPodTemplateSpec unmarshal failed, err=%s", err.Error())
	}

	be := &Controller{}
	be.buildBEPodTemplateSpec(dcr, map[string]interface{}{})
}
