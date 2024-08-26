package resource

import (
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
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
