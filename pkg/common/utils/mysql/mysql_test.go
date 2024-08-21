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
	"testing"
)

func TestAPIs(t *testing.T) {

	/*cfg := DBConfig{
		User:     "root",
		Password: "",
		Host:     "127.0.0.1",
		Port:     "9030",
		Database: "mysql",
	}

	db, err := NewDorisSqlDB(cfg)
	if err != nil {
		fmt.Printf("NewDorisSqlDB err : %s\n", err.Error())
	}
	defer db.Close()

	// ShowFrontends
	frontends, err := db.ShowFrontends()
	if err != nil {
		fmt.Printf("ShowFrontends err:%s \n", err.Error())
	}
	fmt.Printf("ShowFrontends :%+v \n", frontends)

	// ShowBackends
	bes, err := db.ShowBackends()
	if err != nil {
		fmt.Printf("ShowBackends err:%s \n", err.Error())
	}
	fmt.Printf("ShowBackends :%+v \n", bes)

	// DropObserver
	arr := []*Frontend{
		&Frontend{Host: "doriscluster-sample-fe-1.doriscluster-sample-fe-internal.doris.svc.cluster.local", EditLogPort: 9010},
		&Frontend{Host: "doriscluster-sample-fe-2.doriscluster-sample-fe-internal.doris.svc.cluster.local", EditLogPort: 9010},
	}

	db.DropObserver(arr)

	bes, err = db.ShowBackends()
	if err != nil {
		fmt.Printf("ShowBackends err:%s \n", err.Error())
	}
	fmt.Printf("ShowBackends after drop %+v \n", bes)

	// DecommissionBE
	arr1 := []*Backend{
		&Backend{Host: "doriscluster-sample-be-3.doriscluster-sample-be-internal.doris.svc.cluster.local", HeartbeatPort: 9050},
		&Backend{Host: "doriscluster-sample-be-4.doriscluster-sample-be-internal.doris.svc.cluster.local", HeartbeatPort: 9050},
	}
	db.DecommissionBE(arr1)

	bes, err = db.ShowBackends()
	if err != nil {
		fmt.Printf("ShowBackends err: %s \n", err.Error())
	}
	fmt.Printf("ShowBackends after decommission%+v \n", bes)*/

}
