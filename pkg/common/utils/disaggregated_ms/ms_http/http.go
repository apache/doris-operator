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
	"bytes"
	"fmt"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_meta"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
)

const (
	CREATE_INSTANCE_PREFIX_TEMPLATE = `http://%s/MetaService/http/create_instance?token=%s`
	DELETE_INSTANCE_PREFIX_TEMPLATE = `http://%s/MetaService/http/drop_instance?token=%s`
	GET_INSTANCE_PREFIX_TEMPLATE    = `http://%s/MetaService/http/get_instance?token=%s&instance_id=%s`
	DROP_NODE_PREFIX_TEMPLATE       = `http://%s/MetaService/http/drop_node?token=%s`
	GET_CLUSTER_PREFIX_TEMPLATE     = `http://%s/MetaService/http/get_cluster?token=%s`
)

//realize the metaservice interface
//https://doris.apache.org/zh-CN/docs/dev/separation-of-storage-and-compute/meta-service-resource-http-api#%E5%88%9B%E5%BB%BA-instance

func GetInstance(endpoint, token, instanceId string) (*MSResponse, error) {
	addr := fmt.Sprintf(GET_INSTANCE_PREFIX_TEMPLATE, endpoint, token, instanceId)
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	mr := &MSResponse{}
	if err := json.Unmarshal(body, mr); err != nil {
		return nil, err
	}
	return mr, nil
}

func DeleteInstance(endpoint, token, instanceId string) (*MSResponse, error) {
	addr := fmt.Sprintf(DELETE_INSTANCE_PREFIX_TEMPLATE, endpoint, token)
	delReq := map[string]string{}
	delReq[ms_meta.Instance_id] = instanceId
	delReqBytes, err := json.Marshal(delReq)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(delReqBytes)
	req, err := http.NewRequest("DELETE", addr, r)
	if err != nil {
		return nil, err
	}
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	mr := &MSResponse{}
	if err = json.Unmarshal(body, mr); err != nil {
		return mr, err
	}

	return mr, nil
}

func putRequest(requestAddress string, reqBody []byte) (*MSResponse, error) {
	req, err := http.NewRequest("PUT", requestAddress, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("putRequest NewRequest error: %w", err)
	}

	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("putRequest sending HTTP request error: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("putRequest reading HTTP response body error: %w", err)
	}

	mr := &MSResponse{}
	if err := json.Unmarshal(body, mr); err != nil {
		return nil, fmt.Errorf("putRequest unmarshalling JSON response error: %w", err)
	}

	return mr, nil
}

func CreateInstance(endpoint, token string, instanceInfo []byte) (*MSResponse, error) {
	addr := fmt.Sprintf(CREATE_INSTANCE_PREFIX_TEMPLATE, endpoint, token)
	mr, err := putRequest(addr, instanceInfo)
	if err != nil {
		return nil, fmt.Errorf("CreateInstance putRequest failed: %w", err)
	}
	return mr, nil
}

func GetFECluster(endpoint, token, instanceId, cloudUniqueId string) ([]*NodeInfo, error) {
	param := map[string]interface{}{
		"instance_id":     instanceId,
		"cloud_unique_id": cloudUniqueId,
		"cluster_name":    "RESERVED_CLUSTER_NAME_FOR_SQL_SERVER",
		"cluster_id":      "RESERVED_CLUSTER_ID_FOR_SQL_SERVER",
	}
	str, _ := json.Marshal(param)
	addr := fmt.Sprintf(GET_CLUSTER_PREFIX_TEMPLATE, endpoint, token)

	mr, err := putRequest(addr, str)
	if err != nil {
		return nil, fmt.Errorf("GetFECluster putRequest failed: %w", err)
	}

	return mr.MSResponseResultNodesToNodeInfos()
}

func DropFENodes(endpoint, token, instanceID string, nodes []*NodeInfo) (*MSResponse, error) {
	cluster := Cluster{
		ClusterName: FeClusterName,
		ClusterID:   FeClusterId,
		Type:        FeNodeType,
		Nodes:       nodes,
	}
	mr, err := dropNodesFromSpecifyCluster(endpoint, token, instanceID, cluster)
	if err != nil {
		return mr, fmt.Errorf("DropFENodes dropNodesFromSpecifyCluster failed: %w", err)
	}
	return mr, nil
}

// dropNodesFromSpecifyCluster drop all nodes of specify cluster from ms
func dropNodesFromSpecifyCluster(endpoint, token, instanceID string, cluster Cluster) (*MSResponse, error) {
	addr := fmt.Sprintf(DROP_NODE_PREFIX_TEMPLATE, endpoint, token)
	param := MSRequest{
		InstanceID: instanceID,
		Cluster:    cluster,
	}
	jsonData, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("dropNodesFromSpecifyCluster failed to marshal request data: %w", err)
	}

	mr, err := putRequest(addr, jsonData)
	if err != nil {
		return nil, fmt.Errorf("dropNodesFromSpecifyCluster putRequest failed: %w", err)
	}
	return mr, nil

}

// suspend cluster
func SuspendComputeCluster() (*MSResponse, error) {
	//TODO: suspend compute cluster
	return nil, nil
}

func DropComputeCluster() (*MSResponse, error) {
	//TODO: drop compute cluster
	return nil, nil
}
