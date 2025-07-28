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
package cmdutil

import (
    "errors"
    "fmt"
    "github.com/apache/doris-operator/pkg/common/cmd/types"
    _ "github.com/go-sql-driver/mysql"
    "github.com/jmoiron/sqlx"
)

//Client provides abstractions that access doris cluster methods.
type Client interface {
    ShowFrontends() ([]*cmdtypes.Frontend, error)
    ShowBackends() ([]*cmdtypes.Backend, error)
}

var _ Client = &DorisClient{}
type DorisClient struct {
    db *sqlx.DB
}

func NewDorisClient(user, password, host, queryPort string) (*DorisClient, error) {
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, queryPort, "mysql")
    db, err := sqlx.Open("mysql", dsn)
    if err != nil {
        return nil, errors.New("NewDorisSqlDB sqlx.Open failed open doris sql client connection, err: "+ err.Error())
    }

    return &DorisClient{
        db:db,
    }, nil
}

func(dc *DorisClient) ShowFrontends()([]*cmdtypes.Frontend, error) {
    if err := dc.db.Ping(); err != nil {
        return nil, err
    }

    var fs []*cmdtypes.Frontend
    if err := dc.db.Select(&fs, "show frontends"); err != nil {
        return fs, err
    }
    return fs, nil
}

func (dc *DorisClient) ShowBackends()([]*cmdtypes.Backend, error) {
    if err := dc.db.Ping(); err != nil {
        return nil, err
    }

    var bs []*cmdtypes.Backend
    if err := dc.db.Select(&bs, "show backends"); err != nil {
        return bs, err
    }
    return bs, nil
}
