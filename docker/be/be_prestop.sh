#!/bin/bash

DORIS_ROOT=${DORIS_ROOT:-"/opt/apache-doris"}
# the fe query port for mysql.
FE_QUERY_PORT=${FE_QUERY_PORT:-9030}
# rpc port for fe communicate with be.
HEARTBEAT_PORT=9050
DORIS_HOME=${DORIS_ROOT}/be
# if pod as compute node, the COMPONENT_TYPE=COMPUTE
COMPONENT_TYPE=${COMPONENT_TYPE:-"storage"}
# retry 30 times to drop myself, interval is 2s, timeout=60s
DROP_TIMEOUT=60
# drop myself interval=2w
DROP_INTERVAL=2
MY_SELF=
MY_IP=`hostname -i`
MY_HOSTNAME=`hostname -f`
BE_CONFIG=$DORIS_HOME/conf/be.conf
# represent the node self have removed from fe cluster or not.
REMOVED=false

log_stderr()
{
    echo "[`date`] $@" >&2
}

show_backends(){
    local svc=$1
    timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u root --skip-column-names --batch -e 'SHOW BACKENDS;'
}

collect_env_info()
{
    # heartbeat_port from conf file
    local heartbeat_port=`parse_confval_from_conf "heartbeat_service_port"`
    if [[ "x$heartbeat_port" != "x" ]] ; then
        HEARTBEAT_PORT=$heartbeat_port
    fi

    if [[ "x$HOST_TYPE" == "xIP" ]] ; then
        MY_SELF=$MY_IP
    else
        MY_SELF=$MY_HOSTNAME
    fi
}

#parse the `$BE_CONFIG` file, passing the key need resolve as parameter.
parse_confval_from_conf()
{
    # a naive script to grep given confkey from fe conf file
    # assume conf format: ^\s*<key>\s*=\s*<value>\s*$
    local confkey=$1
    local confvalue=`grep "\<$confkey\>" $BE_CONFIG | grep -v '^\s*#' | sed 's|^\s*'$confkey'\s*=\s*\(.*\)\s*$|\1|g'`
    echo "$confvalue"
}

function drop_my_self()
{
    local fe_addr=$1
    while true
    local timeout=$DROP_TIMEOUT
    start=`date +%s`

    do
        memlist=`show_backends $fe_addr`
        if echo "$memlist" | grep -q -w "$MY_SELF" &> /dev/null; then
            log_stderr "myself in fe cluster start drop myself."
              timeout 15 mysql --connect-timeout 2 -h $fe_addr -P $FE_QUERY_PORT -u root --skip-column-names --batch -e "ALTER SYSTEM DROPP BACKEND \"$MY_SELF:$HEARTBEAT_PORT\";"
        else
            REMOVED=true
            break
        fi

        let "expire=start+timeout"
        now=`date +%s`
        if [[ $expire -le $now ]]; then
            log_stderr "timeout, drop myself from $fe_addr failed."
            break
        else
            log_stderr "drop myself failed waiting for ${DROP_INTERVAL}s to retry."
            sleep $DROP_INTERVAL
        fi
    done
}

function drop_my_self_fe_array()
{
    addrs=$1
    local addr_array=(${addrs//,/ })
    for addr in ${addr_array[@]}
    do
        drop_my_self $addr
        if [[ $REMOVED ]]; then
            break
        fi
    done
}

fe_addrs=$ENV_FE_ADDR
collect_env_info
if [[ "x$COMPONENT_TYPE" == "xCOMPUTE" ]]; then
    drop_my_self_fe_array $fe_addrs
fi
# stop myself
$DORIS_HOME/bin/stop_be.sh
