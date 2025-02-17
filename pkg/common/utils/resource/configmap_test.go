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

package resource

import (
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"strconv"
	"testing"
)

func Test_GetStartMode(t *testing.T) {
	tests := []map[string]interface{}{
		{
			"enable_fqdn_mode": "true",
		},
		{},
		{
			"enable_fqdn_mode": "IP",
		}}
	ress := []string{"FQDN", "FQDN", "IP"}

	for i, test := range tests {
		t.Run("test"+strconv.Itoa(i), func(t *testing.T) {
			res := ress[i]
			mode := GetStartMode(test)
			if res != mode {
				t.Errorf("mode %s not equal res %s", mode, res)
			}
		})
	}
}

func Test_ResolveConfigMpas(t *testing.T) {
	tests := []*corev1.ConfigMap{
		&corev1.ConfigMap{
			Data: map[string]string{
				"fe.conf": `
    # the output dir of stderr and stdout
    LOG_DIR = ${DORIS_HOME}/log

    JAVA_OPTS="-Djavax.security.auth.useSubjectCredsOnly=false -Xss4m -Xmx8192m -XX:+UseMembar -XX:SurvivorRatio=8 -XX:MaxTenuringThreshold=7 -XX:+PrintGCDateStamps -XX:+PrintGCDetails -XX:+UseConcMarkSweepGC -XX:+UseParNewGC -XX:+CMSClassUnloadingEnabled -XX:-CMSParallelRemarkEnabled -XX:CMSInitiatingOccupancyFraction=80 -XX:SoftRefLRUPolicyMSPerMB=0 -Xloggc:$DORIS_HOME/log/fe.gc.log.$CUR_DATE"

    # For jdk 9+, this JAVA_OPTS will be used as default JVM options
    JAVA_OPTS_FOR_JDK_9="-Djavax.security.auth.useSubjectCredsOnly=false -Xss4m -Xmx8192m -XX:SurvivorRatio=8 -XX:MaxTenuringThreshold=7 -XX:+CMSClassUnloadingEnabled -XX:-CMSParallelRemarkEnabled -XX:CMSInitiatingOccupancyFraction=80 -XX:SoftRefLRUPolicyMSPerMB=0 -Xlog:gc*:$DORIS_HOME/log/fe.gc.log.$CUR_DATE:time"

    # INFO, WARN, ERROR, FATAL
    sys_log_level = INFO

    # NORMAL, BRIEF, ASYNC
    sys_log_mode = NORMAL

    # Default dirs to put jdbc drivers,default value is ${DORIS_HOME}/jdbc_drivers
    # jdbc_drivers_dir = ${DORIS_HOME}/jdbc_drivers

    http_port = 8030
    rpc_port = 9020
    query_port = 9030
    edit_log_port = 9010
    enable_fqdn_mode=true
	`}},
		{
			Data: map[string]string{
				"be.conf": `
    PPROF_TMPDIR="$DORIS_HOME/log/"

    JAVA_OPTS="-Xmx1024m -DlogPath=$DORIS_HOME/log/jni.log -Xloggc:$DORIS_HOME/log/be.gc.log.$CUR_DATE -Djavax.security.auth.useSubjectCredsOnly=false -Dsun.java.command=DorisBE -XX:-CriticalJNINatives -DJDBC_MIN_POOL=1 -DJDBC_MAX_POOL=100 -DJDBC_MAX_IDLE_TIME=300000 -DJDBC_MAX_WAIT_TIME=5000"

    # For jdk 9+, this JAVA_OPTS will be used as default JVM options
    JAVA_OPTS_FOR_JDK_9="-Xmx1024m -DlogPath=$DORIS_HOME/log/jni.log -Xlog:gc:$DORIS_HOME/log/be.gc.log.$CUR_DATE -Djavax.security.auth.useSubjectCredsOnly=false -Dsun.java.command=DorisBE -XX:-CriticalJNINatives -DJDBC_MIN_POOL=1 -DJDBC_MAX_POOL=100 -DJDBC_MAX_IDLE_TIME=300000 -DJDBC_MAX_WAIT_TIME=5000"

    # since 1.2, the JAVA_HOME need to be set to run BE process.
    # JAVA_HOME=/path/to/jdk/

    # https://github.com/apache/doris/blob/master/docs/zh-CN/community/developer-guide/debug-tool.md#jemalloc-heap-profile
    # https://jemalloc.net/jemalloc.3.html
    JEMALLOC_CONF="percpu_arena:percpu,background_thread:true,metadata_thp:auto,muzzy_decay_ms:15000,dirty_decay_ms:15000,oversize_threshold:0,lg_tcache_max:20,prof:false,lg_prof_interval:32,lg_prof_sample:19,prof_gdump:false,prof_accum:false,prof_leak:false,prof_final:false"
    JEMALLOC_PROF_PRFIX=""

    # INFO, WARNING, ERROR, FATAL
    sys_log_level = INFO

    # ports for admin, web, heartbeat service
    be_port = 9060
    webserver_port = 8040
    heartbeat_service_port = 9050
    brpc_port = 8060
    doris_cgroup_cpu_path=/sys/fs/cgroup/cpu/doris
    enable_java_support=false`,
			},
		},
		nil,
	}

	m, err := ResolveConfigMaps(tests, dorisv1.Component_FE)
	if err != nil || len(m) == 0 {
		t.Errorf("resolve configmaps faild, len=%d, err=%s", len(m), err.Error())
	}
}

func Test_GetMountConfigMapInfo(t *testing.T) {
	c := dorisv1.ConfigMapInfo{
		ConfigMapName: "test",
		ConfigMaps:    []dorisv1.MountConfigMapInfo{},
	}

	fc := GetMountConfigMapInfo(c)
	if len(fc) != 1 {
		t.Errorf("get mountConfigMapInfo failed, len not equal 1")
	}
}

func Test_getCoreCmName(t *testing.T) {
	type args struct {
		dcr           *dorisv1.DorisCluster
		componentType dorisv1.ComponentType
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{
				dcr: &dorisv1.DorisCluster{
					Spec: dorisv1.DorisClusterSpec{
						FeSpec: &dorisv1.FeSpec{
							BaseSpec: dorisv1.BaseSpec{
								ConfigMapInfo: dorisv1.ConfigMapInfo{
									ConfigMapName: "fe-config",
								},
							},
						},
					},
				},
				componentType: dorisv1.Component_FE,
			},
			want: "fe-config",
		},
		{
			name: "test2",
			args: args{dcr: &dorisv1.DorisCluster{
				Spec: dorisv1.DorisClusterSpec{
					FeSpec: &dorisv1.FeSpec{
						BaseSpec: dorisv1.BaseSpec{
							ConfigMapInfo: dorisv1.ConfigMapInfo{
								ConfigMapName: "fe-config",
								ConfigMaps: []dorisv1.MountConfigMapInfo{
									{
										ConfigMapName: "fe-config-1",
										MountPath:     "config",
									},
								},
							},
						},
					},
				},
			},
				componentType: dorisv1.Component_FE,
			},
			want: "fe-config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDorisCoreConfigMapName(tt.args.dcr, tt.args.componentType); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDorisCoreConfigMapName() = %v, want %v", got, tt.want)
			}
		})
	}
}
