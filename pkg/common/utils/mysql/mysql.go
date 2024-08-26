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
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"k8s.io/klog/v2"
)

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
}

type DB struct {
	*sqlx.DB
}

func NewDorisSqlDB(cfg DBConfig) (*DB, error) {
	if cfg.Host == "mock" {
		return newFakeDB()
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sqlx.Open("mysql", dsn)
	if err != nil {
		klog.Errorf("NewDorisSqlDB sqlx.Open failed open doris sql client connection, err: %s \n", err.Error())
		return nil, err
	}

	if err = db.Ping(); err != nil {
		klog.Errorf("NewDorisSqlDB sqlx.Open.Ping failed ping doris sql client connection, err: %s \n", err.Error())
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}

func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(query, args...)
}

func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	return db.DB.Select(dest, query, args...)
}

func (db *DB) ShowFrontends() ([]*Frontend, error) {
	var fes []*Frontend
	err := db.Select(&fes, "show frontends")
	return fes, err
}

func (db *DB) ShowBackends() ([]*Backend, error) {
	var bes []*Backend
	err := db.Select(&bes, "show backends")
	return bes, err
}

func (db *DB) DecommissionBE(nodes []*Backend) error {
	if len(nodes) == 0 {
		klog.Infoln("mysql DecommissionBE BE node is empty")
		return nil
	}
	nodesString := fmt.Sprintf(`"%s:%d"`, nodes[0].Host, nodes[0].HeartbeatPort)
	for _, node := range nodes[1:] {
		nodesString = nodesString + fmt.Sprintf(`,"%s:%d"`, node.Host, node.HeartbeatPort)
	}

	alter := fmt.Sprintf("ALTER SYSTEM DECOMMISSION BACKEND %s;", nodesString)
	_, err := db.Exec(alter)
	return err
}

func (db *DB) DropObserver(nodes []*Frontend) error {
	if len(nodes) == 0 {
		klog.Infoln("DropObserver observer node is empty")
		return nil
	}
	var alter string
	for _, node := range nodes {
		alter = alter + fmt.Sprintf(`ALTER SYSTEM DROP OBSERVER "%s:%d";`, node.Host, node.EditLogPort)
	}
	_, err := db.Exec(alter)
	return err
}

func (db *DB) GetObservers() ([]*Frontend, error) {
	frontends, err := db.ShowFrontends()
	if err != nil {
		klog.Errorf("GetObservers show frontends failed, err: %s\n", err.Error())
		return nil, err
	}
	var res []*Frontend
	for _, fe := range frontends {
		if fe.Role == FE_OBSERVE_ROLE {
			res = append(res, fe)
		}
	}
	return res, nil
}

// GetFollowers return fe master,all followers(including master) and err
func (db *DB) GetFollowers() (*Frontend, []*Frontend, error) {
	frontends, err := db.ShowFrontends()
	if err != nil {
		klog.Errorf("GetFollowers show frontends failed, err: %s\n", err.Error())
		return nil, nil, err
	}
	var res []*Frontend
	var master *Frontend
	for _, fe := range frontends {
		if fe.Role == FE_FOLLOWER_ROLE {
			res = append(res, fe)
			if fe.IsMaster {
				master = fe
			}
		}
	}
	return master, res, nil
}
