// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package ms_http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_CreateInstance(t *testing.T) {
	tests := map[string]string{"disaggregated-test-cluster3": ` {
      "instance_id": "disaggregated-test-cluster3",
      "name": "instance-name",
      "user_id": "test_user",
      "vault": {
        "obj_info": {
          "ak": "test_ak",
          "sk": "test_sk",
          "bucket": "test_bucket",
          "prefix": "test-prefix",
          "endpoint": "cos.ap-beijing.myqcloud.com",
          "external_endpoint": "cos.ap-beijing.myqcloud.com",
          "region": "ap-beijing",
          "provider": "COS",
          "user_id": "test_cluster_user_id"
        }
      }
    }`}

	testResults := map[string]string{
		"disaggregated-test-cluster3": `{"code":"OK","msg":"","result":{}}`,
	}

	for instanceId, instanceInfo := range tests {
		t.Run(instanceId, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

				rw.Write([]byte(testResults[instanceId]))
			}))
			defer server.Close()

			ms, err := CreateInstance(server.Listener.Addr().String(), "greedisgood9999", []byte(instanceInfo))
			if err != nil {
				t.Errorf("%s create failed, err=%s", instanceId, err.Error())
				return
			}

			if ms.Code != "OK" {
				t.Errorf("%s create failed, code not ok.", instanceId)
			}
		})
	}
}

func Test_GetInstance(t *testing.T) {
	tests := map[string]string{
		"disaggregated-test-cluster3": `{
    "code": "OK",
    "msg": "",
    "result": {
        "user_id": "test_user",
        "instance_id": "disaggregated-test-cluster3",
        "name": "instance-name",
        "clusters": [
            {
                "cluster_id": "RESERVED_CLUSTER_ID_FOR_SQL_SERVER",
                "cluster_name": "RESERVED_CLUSTER_NAME_FOR_SQL_SERVER",
                "type": "SQL",
                "nodes": [
                    {
                        "cloud_unique_id": "1:disaggregated-test-cluster3:test-disaggregated-cluster-fe-0",
                        "ip": "test-disaggregated-cluster-fe-0.test-disaggregated-cluster-fe.test.svc.cluster.local",
                        "ctime": "1723706571",
                        "mtime": "1723706571",
                        "edit_log_port": 9010,
                        "node_type": "FE_MASTER",
                        "host": "test-disaggregated-cluster-fe-0.test-disaggregated-cluster-fe.test.svc.cluster.local"
                    },
                    {
                        "cloud_unique_id": "1:disaggregated-test-cluster3:test-disaggregated-cluster-fe-1",
                        "ip": "test-disaggregated-cluster-fe-1.test-disaggregated-cluster-fe.test.svc.cluster.local",
                        "ctime": "1723706571",
                        "mtime": "1723706571",
                        "status": "NODE_STATUS_RUNNING",
                        "edit_log_port": 9010,
                        "node_type": "FE_OBSERVER",
                        "host": "test-disaggregated-cluster-fe-1.test-disaggregated-cluster-fe.test.svc.cluster.local"
                    }
                ]
            },
            {
                "cluster_id": "test_test_disaggregated_cluster_test1",
                "cluster_name": "test1",
                "type": "COMPUTE",
                "nodes": [
                    {
                        "cloud_unique_id": "1:disaggregated-test-cluster3:test-disaggregated-cluster-test1-1",
                        "ip": "test-disaggregated-cluster-test1-1.test-disaggregated-cluster-test1.test.svc.cluster.local",
                        "ctime": "1723706753",
                        "mtime": "1723706753",
                        "heartbeat_port": 9050,
                        "host": "test-disaggregated-cluster-test1-1.test-disaggregated-cluster-test1.test.svc.cluster.local"
                    },
                    {
                        "cloud_unique_id": "1:disaggregated-test-cluster3:test-disaggregated-cluster-test1-0",
                        "ip": "test-disaggregated-cluster-test1-0.test-disaggregated-cluster-test1.test.svc.cluster.local",
                        "ctime": "1723706755",
                        "mtime": "1723706755",
                        "status": "NODE_STATUS_RUNNING",
                        "heartbeat_port": 9050,
                        "host": "test-disaggregated-cluster-test1-0.test-disaggregated-cluster-test1.test.svc.cluster.local"
                    },
                    {
                        "cloud_unique_id": "1:disaggregated-test-cluster3:test-disaggregated-cluster-test1-2",
                        "ip": "test-disaggregated-cluster-test1-2.test-disaggregated-cluster-test1.test.svc.cluster.local",
                        "ctime": "1723706768",
                        "mtime": "1723706768",
                        "status": "NODE_STATUS_RUNNING",
                        "heartbeat_port": 9050,
                        "host": "test-disaggregated-cluster-test1-2.test-disaggregated-cluster-test1.test.svc.cluster.local"
                    }
                ],
                "cluster_status": "NORMAL"
            },
            {
                "cluster_id": "test_test_disaggregated_cluster_test2",
                "cluster_name": "test2",
                "type": "COMPUTE",
                "nodes": [
                    {
                        "cloud_unique_id": "1:disaggregated-test-cluster3:test-disaggregated-cluster-test2-1",
                        "ip": "test-disaggregated-cluster-test2-1.test-disaggregated-cluster-test2.test.svc.cluster.local",
                        "ctime": "1723706756",
                        "mtime": "1723706756",
                        "heartbeat_port": 9050,
                        "host": "test-disaggregated-cluster-test2-1.test-disaggregated-cluster-test2.test.svc.cluster.local"
                    },
                    {
                        "cloud_unique_id": "1:disaggregated-test-cluster3:test-disaggregated-cluster-test2-0",
                        "ip": "test-disaggregated-cluster-test2-0.test-disaggregated-cluster-test2.test.svc.cluster.local",
                        "ctime": "1723706770",
                        "mtime": "1723706770",
                        "status": "NODE_STATUS_RUNNING",
                        "heartbeat_port": 9050,
                        "host": "test-disaggregated-cluster-test2-0.test-disaggregated-cluster-test2.test.svc.cluster.local"
                    }
                ],
                "cluster_status": "NORMAL"
            }
        ],
        "status": "NORMAL",
        "iam_user": {
            "user_id": "",
            "ak": "",
            "sk": "",
            "external_id": "disaggregated-test-cluster3"
        },
        "sse_enabled": false,
        "resource_ids": [
            "1"
        ],
        "storage_vault_names": [
            "built_in_storage_vault"
        ],
        "enable_storage_vault": true
    }}`}

	for instanceId, _ := range tests {
		t.Run(instanceId, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.Write([]byte(tests[instanceId]))
			}))
			defer server.Close()
			mr, err := GetInstance(server.Listener.Addr().String(), "greedisgood9999", instanceId)
			if err != nil {
				t.Errorf("%s get instance failed, err=%s", instanceId, err.Error())
			}
			if mr.Code != "OK" {
				t.Errorf("%s get instance response code not OK.", instanceId)
			}
		})
	}
}

func Test_DeleteInstance(t *testing.T) {
	tests := map[string]string{
		"disaggregated-test-cluster3": `{"code":"OK","msg":"","result":{}}`,
	}
	for instanceId, _ := range tests {
		t.Run(instanceId, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.Write([]byte(tests[instanceId]))
			}))
			defer server.Close()

			mr, err := DeleteInstance(server.Listener.Addr().String(), "greedisgood9999", instanceId)
			if err != nil {
				t.Errorf("%s get instance failed, err=%s", instanceId, err.Error())
			}

			if mr.Code != "OK" {
				t.Errorf("%s get instance response code not OK.", instanceId)
			}
		})
	}
}
