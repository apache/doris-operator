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
	_ "crypto/tls"
	"database/sql/driver"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"strconv"
	"testing"
)

func Test_ShowFrontends(t *testing.T) {
	mysql_db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("sqlmock new failed %s", err.Error())
	}

	columns := []string{"Name", "Host", "EditLogPort", "HttpPort", "QueryPort", "RpcPort", "ArrowFlightSqlPort", "Role", "IsMaster",
		"ClusterId", "Join", "Alive", "ReplayedJournalId", "LastStartTime", "LastHeartbeat", "IsHelper", "ErrMsg", "Version", "CurrentConnected"}
	values := []driver.Value{"fe_36d7bccc_d358_4dfd_ad4c_6e988f94f12d", "doriscluster-sample-fe-0.doriscluster-sample-fe-internal.default.svc.cluster.local", 9010, 8030, 9030, 9020, -1, "FOLLOWER", true, "1807668748", true, true, "15443", "2024-08-21 10:04:29",
		"2024-08-22 07:29:55", true, "", "doris-2.1.5-rc02-d5a02e095d", "Yes"}
	mock.ExpectQuery("show frontends").WillReturnRows(sqlmock.NewRows(columns).AddRows(values))
	dorisdb := sqlx.NewDb(mysql_db, "mysql")
	db := &DB{
		DB: dorisdb,
	}
	defer db.Close()
	fts, err := db.ShowFrontends()
	if err != nil {
		t.Errorf("show frontends failed, %s", err.Error())
	}
	if len(fts) != 1 {
		t.Errorf("show frontends failed, not retun one frontend.")
	}
}

func Test_ShowBackends(t *testing.T) {
	columns := []string{"BackendId", "Host", "HeartbeatPort", "BePort", "HttpPort", "BrpcPort", "ArrowFlightSqlPort", "LastStartTime",
		"LastHeartbeat", "Alive", "SystemDecommissioned", "TabletNum", "DataUsedCapacity", "TrashUsedCapacity", "AvailCapacity", "TotalCapacity", "UsedPct", "MaxDiskUsedPct",
		"RemoteUsedCapacity", "Tag", "ErrMsg", "Version", "Status", "HeartbeatFailureCounter", "NodeRole"}
	values := []driver.Value{"10009", "doriscluster-sample-be-0.doriscluster-sample-be-internal.default.svc.cluster.local", 9050, 9060, 8040, 8060, -1, "2024-08-21 10:05:37",
		"2024-08-22 08:29:46", true, false, 24, "0.000", "0.000", "74.619 GB", "439.037 GB", "83.00 %", "83.00 %", "0.000",
		"{\"location\" : \"default\"}", "", "doris-2.1.5-rc02-d5a02e095d", "{\"lastSuccessReportTabletsTime\":\"2024-08-22 08:29:09\",\"lastStreamLoadTime\":-1,\"isQueryDisabled\":false,\"isLoadDisabled\":false}",
		0, "mix"}
	mysql_db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("sqlmock new failed %s", err.Error())
	}
	mock.ExpectQuery("show backends").WillReturnRows(sqlmock.NewRows(columns).AddRows(values))
	dorisdb := sqlx.NewDb(mysql_db, "mysql")
	db := &DB{
		DB: dorisdb,
	}
	defer db.Close()

	bds, err := db.ShowBackends()
	if err != nil {
		t.Errorf("show backends failed, %s", err.Error())
	}
	if len(bds) != 1 {
		t.Errorf("show backends failed, not return one backend.")
	}
}

func TestDB_GetBackendsByCGName(t *testing.T) {
	columns := []string{"BackendId", "Host", "HeartbeatPort", "BePort", "HttpPort", "BrpcPort", "ArrowFlightSqlPort", "LastStartTime",
		"LastHeartbeat", "Alive", "SystemDecommissioned", "TabletNum", "DataUsedCapacity", "TrashUsedCapacity", "AvailCapacity", "TotalCapacity", "UsedPct", "MaxDiskUsedPct",
		"RemoteUsedCapacity", "Tag", "ErrMsg", "Version", "Status", "HeartbeatFailureCounter", "NodeRole"}
	values := []driver.Value{"10009", "doriscluster-sample-be-0.doriscluster-sample-be-internal.default.svc.cluster.local", 9050, 9060, 8040, 8060, -1, "2024-08-21 10:05:37",
		"2024-08-22 08:29:46", true, false, 24, "0.000", "0.000", "74.619 GB", "439.037 GB", "83.00 %", "83.00 %", "0.000",
		"{\"location\" : \"default\",\"compute_group_name\":\"test\"}", "", "doris-2.1.5-rc02-d5a02e095d", "{\"lastSuccessReportTabletsTime\":\"2024-08-22 08:29:09\",\"lastStreamLoadTime\":-1,\"isQueryDisabled\":false,\"isLoadDisabled\":false}",
		0, "mix"}
	mysql_db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("sqlmock new failed %s", err.Error())
	}
	mock.ExpectQuery("show backends").WillReturnRows(sqlmock.NewRows(columns).AddRows(values))
	dorisdb := sqlx.NewDb(mysql_db, "mysql")
	db := &DB{
		DB: dorisdb,
	}
	defer db.Close()

	db.GetBackendsByCGName("test")
}

func Test_DecommissionBE(t *testing.T) {
	version := "doris-2.1.5-rc02-d5a02e095d"
	startTime := "2024-08-21 10:05:37"
	heartbeat := "2024-08-22 08:29:46"
	values := []*Backend{{BackendID: "10009", Host: "doriscluster-sample-be-0.doriscluster-sample-be-internal.default.svc.cluster.local", HeartbeatPort: 9050, BePort: 9060, HttpPort: 8040, BrpcPort: 8060, ArrowFlightSqlPort: -1, LastStartTime: &startTime,
		LastHeartbeat: &heartbeat, Alive: true, TabletNum: 24, DataUsedCapacity: "0.000", TrashUsedCapacity: "0.000", AvailCapacity: "74.619 GB", TotalCapacity: "439.037 GB", UsedPct: "83.00 %", MaxDiskUsedPct: "83.00 %",
		RemoteUsedCapacity: "0.000", ErrMsg: "", Version: &version, Status: "{\"lastSuccessReportTabletsTime\":\"2024-08-22 08:29:09\",\"lastStreamLoadTime\":-1,\"isQueryDisabled\":false,\"isLoadDisabled\":false}", HeartbeatFailureCounter: 0, NodeRole: "mix"}}
	values2 := []*Backend{{BackendID: "10009", Host: "doriscluster-sample-be-0.doriscluster-sample-be-internal.default.svc.cluster.local", HeartbeatPort: 9050, BePort: 9060, HttpPort: 8040, BrpcPort: 8060, ArrowFlightSqlPort: -1, LastStartTime: &startTime,
		LastHeartbeat: &heartbeat, Alive: true, TabletNum: 24, DataUsedCapacity: "0.000", TrashUsedCapacity: "0.000", AvailCapacity: "74.619 GB", TotalCapacity: "439.037 GB", UsedPct: "83.00 %", MaxDiskUsedPct: "83.00 %",
		RemoteUsedCapacity: "0.000", ErrMsg: "", Version: &version, Status: "{\"lastSuccessReportTabletsTime\":\"2024-08-22 08:29:09\",\"lastStreamLoadTime\":-1,\"isQueryDisabled\":false,\"isLoadDisabled\":false}", HeartbeatFailureCounter: 0, NodeRole: "mix"},
		{BackendID: "10010", Host: "doriscluster-sample-be-1.doriscluster-sample-be-internal.default.svc.cluster.local", HeartbeatPort: 9050, BePort: 9060, HttpPort: 8040, BrpcPort: 8060, ArrowFlightSqlPort: -1, LastStartTime: &startTime,
			LastHeartbeat: &heartbeat, Alive: true, TabletNum: 24, DataUsedCapacity: "0.000", TrashUsedCapacity: "0.000", AvailCapacity: "74.619 GB", TotalCapacity: "439.037 GB", UsedPct: "83.00 %", MaxDiskUsedPct: "83.00 %",
			RemoteUsedCapacity: "0.000", ErrMsg: "", Version: &version, Status: "{\"lastSuccessReportTabletsTime\":\"2024-08-22 08:29:09\",\"lastStreamLoadTime\":-1,\"isQueryDisabled\":false,\"isLoadDisabled\":false}", HeartbeatFailureCounter: 0, NodeRole: "mix"}}
	tests := [][]*Backend{
		{},
		values,
		values2,
	}
	mysql_db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("sqlmock new failed %s", err.Error())
	}
	mock.ExpectExec("ALTER SYSTEM DECOMMISSION BACKEND").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`ALTER SYSTEM DECOMMISSION BACKEND "doriscluster-sample-be-0.doriscluster-sample-be-internal.default.svc.cluster.local:9050","doriscluster-sample-be-1.doriscluster-sample-be-internal.default.svc.cluster.local:9050"`).WillReturnResult(sqlmock.NewResult(1, 2))
	dorisdb := sqlx.NewDb(mysql_db, "mysql")
	db := &DB{
		DB: dorisdb,
	}
	defer db.Close()
	for i, test := range tests {

		t.Run("test"+strconv.Itoa(i), func(t *testing.T) {
			err = db.DecommissionBE(test)
			if err != nil {
				t.Errorf("test decommission failed, err=%s", err.Error())
			}
		})
	}
}

func Test_DropObserver(t *testing.T) {
	version := "doris-2.1.5-rc02-d5a02e095d"
	startTime := "2024-08-21 10:04:29"
	heartbeat := "2024-08-22 07:29:55"
	values := []*Frontend{{"fe_36d7bccc_d358_4dfd_ad4c_6e988f94f12d", "doriscluster-sample-fe-0.doriscluster-sample-fe-internal.default.svc.cluster.local", 9010, 8030, 9030, 9020, -1, "FOLLOWER", true, "1807668748", true, true, "15443", &startTime,
		&heartbeat, true, "", &version, "Yes"}}

	tests := [][]*Frontend{
		{},
		values,
	}

	mysql_db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("sqlmock new failed %s", err.Error())
	}
	mock.ExpectExec("ALTER SYSTEM DROP OBSERVER").WillReturnResult(sqlmock.NewResult(1, 1))
	dorisdb := sqlx.NewDb(mysql_db, "mysql")
	db := &DB{
		DB: dorisdb,
	}
	defer db.Close()

	for i, test := range tests {
		t.Run("test"+strconv.Itoa(i), func(t *testing.T) {
			err = db.DropObserver(test)
			if err != nil {
				t.Errorf("test decommission failed, err=%s", err.Error())
			}
		})
	}
}

func Test_GetObservers(t *testing.T) {
	mysql_db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("sqlmock new failed %s", err.Error())
	}

	columns := []string{"Name", "Host", "EditLogPort", "HttpPort", "QueryPort", "RpcPort", "ArrowFlightSqlPort", "Role", "IsMaster",
		"ClusterId", "Join", "Alive", "ReplayedJournalId", "LastStartTime", "LastHeartbeat", "IsHelper", "ErrMsg", "Version", "CurrentConnected"}
	values := []driver.Value{"fe_36d7bccc_d358_4dfd_ad4c_6e988f94f12d", "doriscluster-sample-fe-0.doriscluster-sample-fe-internal.default.svc.cluster.local", 9010, 8030, 9030, 9020, -1, "OBSERVER", true, "1807668748", true, true, "15443", "2024-08-21 10:04:29",
		"2024-08-22 07:29:55", true, "", "doris-2.1.5-rc02-d5a02e095d", "Yes"}
	mock.ExpectQuery("show frontends").WillReturnRows(sqlmock.NewRows(columns).AddRows(values))
	dorisdb := sqlx.NewDb(mysql_db, "mysql")
	db := &DB{
		DB: dorisdb,
	}
	defer db.Close()

	fts, err := db.GetObservers()
	if err != nil {
		t.Errorf("get observers failed, err=%s", err.Error())
	}
	if len(fts) != 1 {
		t.Errorf("get observers failed, not observer")
	}
}

func Test_GetFollowers(t *testing.T) {
	mysql_db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("sqlmock new failed %s", err.Error())
	}

	columns := []string{"Name", "Host", "EditLogPort", "HttpPort", "QueryPort", "RpcPort", "ArrowFlightSqlPort", "Role", "IsMaster",
		"ClusterId", "Join", "Alive", "ReplayedJournalId", "LastStartTime", "LastHeartbeat", "IsHelper", "ErrMsg", "Version", "CurrentConnected"}
	values := []driver.Value{"fe_36d7bccc_d358_4dfd_ad4c_6e988f94f12d", "doriscluster-sample-fe-0.doriscluster-sample-fe-internal.default.svc.cluster.local", 9010, 8030, 9030, 9020, -1, "FOLLOWER", true, "1807668748", true, true, "15443", "2024-08-21 10:04:29",
		"2024-08-22 07:29:55", true, "", "doris-2.1.5-rc02-d5a02e095d", "Yes"}
	mock.ExpectQuery("show frontends").WillReturnRows(sqlmock.NewRows(columns).AddRows(values))
	dorisdb := sqlx.NewDb(mysql_db, "mysql")
	db := &DB{
		DB: dorisdb,
	}
	defer db.Close()

	_, fts, err := db.GetFollowers()
	if err != nil {
		t.Errorf("get observers failed, err=%s", err.Error())
	}
	if len(fts) != 1 {
		t.Errorf("get observers failed, not observer")
	}
}

func Test_DropBE(t *testing.T) {
	tests := [][]*Backend{
		{
			{
				Host:          "test",
				HeartbeatPort: 9050,
			}, {
				Host:          "test1",
				HeartbeatPort: 9050,
			},
		},
		{},
	}

	mysql_db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("sqlmock new failed %s", err.Error())
	}
	mock.ExpectExec("ALTER SYSTEM DROPP BACKEND").WillReturnResult(sqlmock.NewResult(1, 1))
	dorisdb := sqlx.NewDb(mysql_db, "mysql")
	db := &DB{
		DB: dorisdb,
	}
	defer db.Close()

	for i, test := range tests {
		t.Run("test"+strconv.Itoa(i), func(t *testing.T) {
			err = db.DropBE(test)
			if err != nil {
				t.Errorf("test decommission failed, err=%s", err.Error())
			}
		})
	}
}
