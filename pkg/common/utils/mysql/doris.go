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

package mysql

import (
	_ "github.com/go-sql-driver/mysql"
	"k8s.io/klog/v2"
	"sort"
	"strconv"
	"strings"
)

const (
	FE_FOLLOWER_ROLE = "FOLLOWER"
	FE_OBSERVE_ROLE  = "OBSERVER"
)

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

// BuildSeqNumberToFrontendMap
// input ipMap key is podIP,value is fe.podName(from 'kubectl get pods -owide')
// return frontendMap key is fe pod index ,value is frontend
func BuildSeqNumberToFrontendMap(frontends []*Frontend, ipMap map[string]string, podTemplateName string) (map[int]*Frontend, error) {
	frontendMap := make(map[int]*Frontend)
	for _, fe := range frontends {
		var podSignName string
		if strings.HasPrefix(fe.Host, podTemplateName) {
			// use fqdn, not need ipMap
			// podSignName like: doriscluster-sample-fe-0.doriscluster-sample-fe-internal.doris.svc.cluster.local
			podSignName = fe.Host
		} else {
			// use ip
			// podSignName like: doriscluster-sample-fe-0
			podSignName = ipMap[fe.Host]
		}
		split := strings.Split(strings.Split(strings.Split(podSignName, podTemplateName)[1], ".")[0], "-")
		num, err := strconv.Atoi(split[len(split)-1])
		if err != nil {
			klog.Errorf("buildSeqNumberToFrontend can not split pod name,pod name: %s,err:%s", podSignName, err.Error())
			return nil, err
		}
		frontendMap[num] = fe
	}
	return frontendMap, nil
}

// FindNeedDeletedFrontends means descending sort fe by index and return top needRemovedAmount
func FindNeedDeletedObservers(frontendMap map[int]*Frontend, needRemovedAmount int32) []*Frontend {
	var topFrontends []*Frontend
	if int(needRemovedAmount) <= len(frontendMap) {
		keys := make([]int, 0, len(frontendMap))
		for k := range frontendMap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] > keys[j]
		})

		for i := 0; i < int(needRemovedAmount); i++ {
			topFrontends = append(topFrontends, frontendMap[keys[i]])
		}
	} else {
		klog.Errorf("findNeedDeletedFrontends frontendMap size(%d) not larger than needRemovedAmount(%d)", len(frontendMap), needRemovedAmount)
	}
	return topFrontends
}
