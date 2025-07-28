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
package cmdtypes

//Frontend describe the details of frontend node.
type Frontend struct {
    Name               string  `json:"name" db:"Name"`
    Host               string  `json:"host" db:"Host"`
    EditLogPort        int     `json:"edit_log_port" db:"EditLogPort"`
    HttpPort           int     `json:"http_port" db:"HttpPort"`
    QueryPort          int     `json:"query_port" db:"QueryPort"`
    RpcPort            int     `json:"rpc_port" db:"RpcPort"`
    ArrowFlightSqlPort int     `json:"arrow_flight_sql_port" db:"ArrowFlightSqlPort"`
    Role               string  `json:"role" db:"Role"`
    IsMaster           bool    `json:"is_master" db:"IsMaster"`
    ClusterId          string  `json:"cluster_id" db:"ClusterId"`
    Join               bool    `json:"join" db:"Join"`
    Alive              bool    `json:"alive" db:"Alive"`
    ReplayedJournalId  string  `json:"replayed_journal_id" db:"ReplayedJournalId"`
    LastStartTime      *string `json:"last_start_time" db:"LastStartTime"`
    LastHeartbeat      *string `json:"last_heartbeat" db:"LastHeartbeat"`
    IsHelper           bool    `json:"is_helper" db:"IsHelper"`
    ErrMsg             string  `json:"err_msg" db:"ErrMsg"`
    Version            *string `json:"version" db:"Version"`
    CurrentConnected   string  `json:"current_connected" db:"CurrentConnected"`
}

//Backend describe the details of backend node.
type Backend struct {
    BackendID               string  `json:"backend_id" db:"BackendId"`
    Host                    string  `json:"host" db:"Host"`
    HeartbeatPort           int     `json:"heartbeat_port" db:"HeartbeatPort"`
    BePort                  int     `json:"be_port" db:"BePort"`
    HttpPort                int     `json:"http_port" db:"HttpPort"`
    BrpcPort                int     `json:"brpc_port" db:"BrpcPort"`
    ArrowFlightSqlPort      int     `json:"arrow_flight_sql_port" db:"ArrowFlightSqlPort"`
    LastStartTime           *string `json:"last_start_time" db:"LastStartTime"`
    LastHeartbeat           *string `json:"last_heartbeat" db:"LastHeartbeat"`
    Alive                   bool    `json:"alive" db:"Alive"`
    SystemDecommissioned    bool    `json:"system_decommissioned" db:"SystemDecommissioned"`
    TabletNum               int64   `json:"tablet_num" db:"TabletNum"`
    DataUsedCapacity        string  `json:"data_used_capacity" db:"DataUsedCapacity"`
    TrashUsedCapacity       string  `json:"trash_used_capacity" db:"TrashUsedCapacity"`
    TrashUsedCapcacity      string  `json:"trash_used_capcacity" db:"TrashUsedCapcacity"`
    AvailCapacity           string  `json:"avail_capacity" db:"AvailCapacity"`
    TotalCapacity           string  `json:"total_capacity" db:"TotalCapacity"`
    UsedPct                 string  `json:"used_pct" db:"UsedPct"`
    MaxDiskUsedPct          string  `json:"max_disk_used_pct" db:"MaxDiskUsedPct"`
    RemoteUsedCapacity      string  `json:"remote_used_capacity" db:"RemoteUsedCapacity"`
    Tag                     string  `json:"tag" db:"Tag"`
    ErrMsg                  string  `json:"err_msg" db:"ErrMsg"`
    Version                 *string `json:"version" db:"Version"`
    Status                  string  `json:"status" db:"Status"`
    HeartbeatFailureCounter int     `json:"heartbeat_failure_counter" db:"HeartbeatFailureCounter"`
    NodeRole                string  `json:"node_role" db:"NodeRole"`
    CpuCores                string  `json:"cpu_cores" db:"CpuCores"`
    Memory                  string  `json:"memory" db:"Memory"`
}

type Tag struct {
    CloudUniqueId string  `json:"cloud_unique_id"`
    ComputeGroupStatus string `json:"compute_group_status"`
    PrivateEndpoint string `json:"private_endpoint"`
    ComputeGroupName string `json:"compute_group_name"`
    Location string `json:"location"`
    PublicEndpoint string `json:"public_endpoint"`
    ComputeGroupId string `json:"compute_group_id"`
}
