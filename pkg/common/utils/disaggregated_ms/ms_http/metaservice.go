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
	"encoding/json"
	"errors"
)

type MSResponse struct {
	Code   string                 `json:"code,omitempty"`
	Msg    string                 `json:"msg,omitempty"`
	Result map[string]interface{} `json:"result,omitempty"`
}

const (
	SuccessCode    string = "OK"
	ALREADY_EXIST  string = "ALREADY_EXISTED"
	NotFound       string = "NOT_FOUND"
	INTERNAL_ERROR string = "INTERNAL_ERROR"
	FeClusterId           = "RESERVED_CLUSTER_ID_FOR_SQL_SERVER"
	FeClusterName         = "RESERVED_CLUSTER_NAME_FOR_SQL_SERVER"
	FeNodeType            = "SQL"
)

type NodeInfo struct {
	CloudUniqueID string `json:"cloud_unique_id"`
	IP            string `json:"ip"`
	Ctime         string `json:"-"`
	Mtime         string `json:"-"`
	Status        string `json:"-"`
	NodeType      string `json:"node_type,omitempty"`
	EditLogPort   int    `json:"edit_log_port,omitempty"`
	HeartbeatPort string `json:"heartbeat_port,omitempty"`
	Host          string `json:"-"`
}

type Cluster struct {
	ClusterName string      `json:"cluster_name"`
	ClusterID   string      `json:"cluster_id"`
	Type        string      `json:"type"`
	Nodes       []*NodeInfo `json:"nodes"`
}

type MSRequest struct {
	InstanceID string  `json:"instance_id"`
	Cluster    Cluster `json:"cluster"`
}

func (mr *MSResponse) MSResponseResultNodesToNodeInfos() ([]*NodeInfo, error) {

	nodes, ok := mr.Result["nodes"]
	if !ok {
		return nil, errors.New("MSResponseResultNodes is not exist")
	}

	jsonStr, err := json.Marshal(nodes)
	if err != nil {
		return nil, errors.New("MSResponseResultNodesToNodeInfos error marshaling map to JSON: " + err.Error())
	}

	var res []*NodeInfo
	err = json.Unmarshal(jsonStr, &res)
	if err != nil {
		return nil, errors.New("MSResponseResultNodesToNodeInfos Error unmarshaling JSON to struct: " + err.Error())
	}
	return res, nil
}
