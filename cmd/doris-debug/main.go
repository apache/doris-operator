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

package main

import (
	"flag"
	"fmt"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/spf13/viper"
	"net/http"
	"os"
	"strconv"
)

var (
	componentType string
	dorisRootPath string
)

func main() {
	flagParse()
	envParse()
	if !kickOffDebug() {
		return
	}

	fmt.Println("start component " + componentType + "for debugging.....")
	listenPort := readConfigListenPort()
	//registerMockApiHealth()
	if err := http.ListenAndServe(":"+listenPort, nil); err != nil {
		fmt.Println("listenAndServe failed," + err.Error())
		os.Exit(1)
	}
}

func envParse() {
	dorisRootPath, _ = os.LookupEnv(resource.DORIS_ROOT)
	if dorisRootPath == "" {
		dorisRootPath = "/opt/apache-doris"
	}
}

func flagParse() {
	flag.StringVar(&componentType, "component", "fe", "the debug component short name.")
	flag.Parse()
}

func readConfigListenPort() string {
	configFileName := dorisRootPath + "/" + componentType + "/conf/" + componentType + ".conf"
	_, err := os.Stat(configFileName)
	if err != nil {
		fmt.Println("the config file is not exist, stat error", err.Error())
		os.Exit(1)
	}

	file, _ := os.Open(configFileName)
	viper.SetConfigType("properties")
	viper.ReadConfig(file)

	var listenPort string
	if componentType == "fe" {
		configQueryPort := viper.GetString(resource.QUERY_PORT)
		if configQueryPort == "" {
			listenPort = strconv.Itoa(int(resource.GetDefaultPort(resource.QUERY_PORT)))
		}
	} else if componentType == "be" {
		configHeartbeatPort := viper.GetString(resource.HEARTBEAT_SERVICE_PORT)
		fmt.Println("component be" + configHeartbeatPort)
		if configHeartbeatPort == "" {
			listenPort = strconv.Itoa(int(resource.GetDefaultPort(resource.HEARTBEAT_SERVICE_PORT)))
		}
	}
	fmt.Println("component listen port " + listenPort)

	return listenPort
}

func registerMockApiHealth() {
	if componentType == "fe" {
		http.HandleFunc("/api/health", mockFEHealth)
		return
	}

	http.HandleFunc("/api/health", mockBEHealth)
}

func kickOffDebug() bool {
	annotationFileName := resource.POD_INFO_PATH + "/annotations"
	if _, err := os.Stat(annotationFileName); os.IsNotExist(err) {
		fmt.Println(annotationFileName + "is not exists.")
		return false
	}

	file, err := os.Open(annotationFileName)
	if err != nil {
		fmt.Println(annotationFileName + "can't open" + err.Error())
	}
	viper.Reset()
	viper.SetConfigType("properties")
	viper.ReadConfig(file)
	value := viper.GetString(v1.AnnotationDebugKey)
	fmt.Println("the annotations value:" + value)

	if value == "\""+v1.AnnotationDebugValue+"\"" {
		return true
	}

	fmt.Println("the value not equal!", value, v1.AnnotationDebugValue)
	return false
}

func mockFEHealth(w http.ResponseWriter, r *http.Request) {
	//{"msg":"success","code":0,"data":{"online_backend_num":3,"total_backend_num":3},"count":0}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{\"msg\":\"success\",\"code\":0,\"data\":{\"online_backend_num\":3,\"total_backend_num\":3},\"count\":0}"))
}

func mockBEHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{\"status\": \"OK\",\"msg\": \"To Be Added\"}"))
}
