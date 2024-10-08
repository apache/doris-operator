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

package ms_meta

// vault
const (
	Instance_id string = "instance_id"
	Name        string = "name"
	User_id     string = "user_id"
	Vault       string = "vault"
)

// S3
const (
	Obj_info                   string = "obj_info"
	Obj_info_ak                string = "ak"
	Obj_info_sk                string = "sk"
	Obj_info_bucket            string = "bucket"
	Obj_info_prefix            string = "prefix"
	Obj_info_endpoint          string = "endpoint"
	Obj_info_region            string = "region"
	Obj_info_external_endpoint string = "external_endpoint"
	Obj_info_provider          string = "provider"
	//Ram_user                   string = "ram_user"
	//Ram_user_ak                string = "ak"
	//Ram_user_sk                string = "sk"
)

// HDFS
const (
	Key_hdfs_info                                    string = "hdfs_info"
	Key_hdfs_info_build_conf                         string = "build_conf"
	Key_hdfs_info_build_conf_fs_name                 string = "fs_name"
	Key_hdfs_info_build_conf_user                    string = "user"
	Key_hdfs_info_build_conf_hdfs_kerberos_keytab    string = "hdfs_kerberos_keytab"
	Key_hdfs_info_build_conf_hdfs_kerberos_principal string = "hdfs_kerberos_principal"
	Key_hdfs_info_build_conf_hdfs_confs              string = "hdfs_confs"
	Key_hdfs_info_hdfs_confs_key                     string = "key"
	Key_hdfs_info_hdfs_conf_value                    string = "value"
	Key_hdfs_info_prefix                             string = "prefix"
)
