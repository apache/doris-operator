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
	"net/http"
	"os"
	"strconv"

	v1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	"github.com/spf13/viper"
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

	fmt.Println("start component " + componentType + " for debugging.....")
	listenPort := readConfigListenPort()
	//registerMockApiHealth()
	if err := http.ListenAndServe(":"+listenPort, nil); err != nil {
		fmt.Println("listenAndServe failed," + err.Error())
		os.Exit(1)
	}
}

func envParse() {
	dorisRootPath, _ = os.LookupEnv(resource.DORIS_ROOT_KEY)
	if dorisRootPath == "" {
		dorisRootPath = "/opt/apache-doris"
	}
}

func flagParse() {
	flag.StringVar(&componentType, "component", "fe", "the debug component short name.")
	flag.Parse()
}

func readConfigListenPort() string {

	var listenPortName string
	var configFileName string
	var listenPort string

	switch componentType {
	case "fe":
		configFileName = dorisRootPath + "/fe/conf/fe.conf"
		listenPortName = resource.QUERY_PORT
	case "be":
		configFileName = dorisRootPath + "/be/conf/be.conf"
		listenPortName = resource.HEARTBEAT_SERVICE_PORT
	case "ms":
		configFileName = dorisRootPath + "/ms/conf/doris_cloud.conf"
		listenPortName = resource.BRPC_LISTEN_PORT
	default:
		{
			fmt.Println("the componentType is not supported:" + componentType)
			os.Exit(1)
		}
	}

	_, err := os.Stat(configFileName)
	if err != nil {
		fmt.Println("the config file is not exist, stat error", err.Error())
		os.Exit(1)
	}

	file, _ := os.Open(configFileName)
	viper.SetConfigType("properties")
	viper.ReadConfig(file)
	listenPort = viper.GetString(listenPortName)
	if listenPort == "" {
		listenPort = strconv.Itoa(int(resource.GetDefaultPort(listenPortName)))
	}

	fmt.Println("component listen port " + listenPort)
	return listenPort
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

	valueDoris := viper.GetString(v1.AnnotationDebugDorisKey)

	if valueDoris == "\""+v1.AnnotationDebugValue+"\"" {
		return true
	}

	fmt.Printf("No debug flag matched, flags: [%s:%s],[%s:%s] !", v1.AnnotationDebugDorisKey, valueDoris, v1.AnnotationDebugKey, value)
	return false
}

func registerMockApiHealth() {
	if componentType == "fe" {
		http.HandleFunc("/api/health", mockFEHealth)
		return
	}

	http.HandleFunc("/api/health", mockBEHealth)
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
