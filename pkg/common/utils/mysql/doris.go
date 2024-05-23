package mysql

import (
	_ "github.com/go-sql-driver/mysql"
)

type Frontend struct {
	Name               string `json:"name" db:"Name"`
	Host               string `json:"host" db:"Host"`
	EditLogPort        int    `json:"edit_log_port" db:"EditLogPort"`
	HttpPort           int    `json:"http_port" db:"HttpPort"`
	QueryPort          int    `json:"query_port" db:"QueryPort"`
	RpcPort            int    `json:"rpc_port" db:"RpcPort"`
	ArrowFlightSqlPort int    `json:"arrow_flight_sql_port" db:"ArrowFlightSqlPort"`
	Role               string `json:"role" db:"Role"`
	IsMaster           bool   `json:"is_master" db:"IsMaster"`
	ClusterId          string `json:"cluster_id" db:"ClusterId"`
	Join               bool   `json:"join" db:"Join"`
	Alive              bool   `json:"alive" db:"Alive"`
	ReplayedJournalId  string `json:"replayed_journal_id" db:"ReplayedJournalId"`
	LastStartTime      string `json:"last_start_time" db:"LastStartTime"`
	LastHeartbeat      string `json:"last_heartbeat" db:"LastHeartbeat"`
	IsHelper           bool   `json:"is_helper" db:"IsHelper"`
	ErrMsg             string `json:"err_msg" db:"ErrMsg"`
	Version            string `json:"version" db:"Version"`
	CurrentConnected   string `json:"current_connected" db:"CurrentConnected"`
}

type Backend struct {
	BackendID               string `json:"backend_id" db:"BackendId"`
	Host                    string `json:"host" db:"Host"`
	HeartbeatPort           int    `json:"heartbeat_port" db:"HeartbeatPort"`
	BePort                  int    `json:"be_port" db:"BePort"`
	HttpPort                int    `json:"http_port" db:"HttpPort"`
	BrpcPort                int    `json:"brpc_port" db:"BrpcPort"`
	ArrowFlightSqlPort      int    `json:"arrow_flight_sql_port" db:"ArrowFlightSqlPort"`
	LastStartTime           string `json:"last_start_time" db:"LastStartTime"`
	LastHeartbeat           string `json:"last_heartbeat" db:"LastHeartbeat"`
	Alive                   bool   `json:"alive" db:"Alive"`
	SystemDecommissioned    bool   `json:"system_decommissioned" db:"SystemDecommissioned"`
	TabletNum               int64  `json:"tablet_num" db:"TabletNum"`
	DataUsedCapacity        string `json:"data_used_capacity" db:"DataUsedCapacity"`
	TrashUsedCapacity       string `json:"trash_used_capacity" db:"TrashUsedCapacity"`
	TrashUsedCapcacity      string `json:"trash_used_capcacity" db:"TrashUsedCapcacity"`
	AvailCapacity           string `json:"avail_capacity" db:"AvailCapacity"`
	TotalCapacity           string `json:"total_capacity" db:"TotalCapacity"`
	UsedPct                 string `json:"used_pct" db:"UsedPct"`
	MaxDiskUsedPct          string `json:"max_disk_used_pct" db:"MaxDiskUsedPct"`
	RemoteUsedCapacity      string `json:"remote_used_capacity" db:"RemoteUsedCapacity"`
	Tag                     string `json:"tag" db:"Tag"`
	ErrMsg                  string `json:"err_msg" db:"ErrMsg"`
	Version                 string `json:"version" db:"Version"`
	Status                  string `json:"status" db:"Status"`
	HeartbeatFailureCounter int    `json:"heartbeat_failure_counter" db:"HeartbeatFailureCounter"`
	NodeRole                string `json:"node_role" db:"NodeRole"`
}
