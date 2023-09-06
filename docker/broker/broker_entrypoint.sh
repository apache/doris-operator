#!/bin/bash

# the fe query port for mysql.
FE_QUERY_PORT=${FE_QUERY_PORT:-9030}
# timeout for probe fe master.
PROBE_TIMEOUT=60
# interval time to probe fe.
PROBE_INTERVAL=2
# ipc port for fe/be communicate with broker.
IPC_PORT=8000
# fqdn or ip
MY_SELF=
MY_IP=`hostname -i`
MY_HOSTNAME=`hostname -f`
DORIS_ROOT=${DORIS_ROOT:-"/opt/apache-doris"}
DORIS_HOME=${DORIS_ROOT}/apache_hdfs_broker
BK_CONFIG=$DORIS_HOME/conf/apache_hdfs_broker.conf
BK_NAME=broker001
# represents self in fe meta or not.
REGISTERED=false


log_stderr() #11
{
    echo "[`date`] $@" >&2
}

parse_confval_from_bk_conf()
{
    # a naive script to grep given confkey from broker conf file
    # assume conf format: ^\s*<key>\s*=\s*<value>\s*$
    local confkey=$1
    local confvalue=`grep "\<$confkey\>" $BK_CONFIG | grep -v '^\s*#' | sed 's|^\s*'$confkey'\s*=\s*\(.*\)\s*$|\1|g'`
    echo "$confvalue"
}

update_conf_from_configmap()
{
    if [[ "x$CONFIGMAP_MOUNT_PATH" == "x" ]] ; then
        log_stderr 'Empty $CONFIGMAP_MOUNT_PATH env var, skip it!'
        return 0
    fi
    if ! test -d $CONFIGMAP_MOUNT_PATH ; then
        log_stderr "$CONFIGMAP_MOUNT_PATH not exist or not a directory, ignore ..."
        return 0
    fi
    local tgtconfdir=$DORIS_HOME/conf
    for conffile in `ls $CONFIGMAP_MOUNT_PATH`
    do
        log_stderr "Process conf file $conffile ..."
        local tgt=$tgtconfdir/$conffile
        if test -e $tgt ; then
            # make a backup
            mv -f $tgt ${tgt}.bak
        fi
        ln -sfT $CONFIGMAP_MOUNT_PATH/$conffile $tgt
    done
}



# get all brokers info to check self exist or not.
show_brokers(){ #11
    local svc=$1
    timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u root --skip-column-names --batch -e 'SHOW BROKER;'
}


 ## ALTER SYSTEM ADD BROKER broker_name "broker_host1:broker_ipc_port1","broker_host2:broker_ipc_port2",...;

# get all registered fe in cluster, for check the fe have `MASTER`.
function show_frontends() #11
{
    local addr=$1
    echo ""
    timeout 15 mysql  --connect-timeout 2 -h $addr -P $FE_QUERY_PORT -u root --skip-column-names --batch -e 'show frontends;'
}

collect_env_info()
{
    # IPC_PORT from conf file
    local ipc_port=`parse_confval_from_bk_conf "broker_ipc_port"`
    if [[ "x$ipc_port" != "x" ]] ; then
        IPC_PORT=$ipc_port
    fi

    if [[ "x$HOST_TYPE" == "xIP" ]] ; then
        MY_SELF=$MY_IP
    else
        MY_SELF=$MY_HOSTNAME
    fi
}

add_self()
{
    local svc=$1
    start=`date +%s`
    local timeout=$PROBE_TIMEOUT

    while true
    do
        memlist=`show_brokers $svc`
        if echo "$memlist" | grep -q -w "$MY_SELF" &>/dev/null ; then
            log_stderr "Check myself ($MY_SELF:$IPC_PORT)  exist in FE start broker ..."
            break;
        fi

        # check fe cluster have master, if fe have not master wait.
        fe_memlist=`show_frontends $svc`
        local leader=`echo "$fe_memlist" | grep '\<FOLLOWER\>' | awk -F '\t' '{if ($8=="true") print $2}'`
        if [[ "x$leader" != "x" ]]; then
            log_stderr "Check myself ($MY_SELF:$IPC_PORT)  not exist in FE and fe have leader register myself..."
            timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u root --skip-column-names --batch -e "ALTER SYSTEM ADD BROKER $BK_NAME \"$MY_SELF:$IPC_PORT\";"
            let "expire=start+timeout"
            now=`date +%s`
            if [[ $expire -le $now ]] ; then
                log_stderr "Time out, abort!"
                return 0
            fi
        else
            log_stderr "not have leader wait fe cluster select a master, sleep 2s..."
            sleep $PROBE_INTERVAL
        fi
    done
}

# check be exist or not, if exist return 0, or register self in fe cluster. when all fe address failed exit script.
# `xxx1:port,xxx2:port` as parameter to function.
function check_and_register()
{
    addrs=$1
    local addrArr=(${addrs//,/ })
    for addr in ${addrArr[@]}
    do
        add_self $addr
    done

    if [[ $REGISTERED ]]; then
        return 0
    else
        exit 1
    fi
}

fe_addrs=$1
if [[ "x$fe_addrs" == "x" ]]; then
    echo "need fe address as paramter!"
    echo "  Example $0 <fe_addr>"
    exit 1
fi

update_conf_from_configmap
collect_env_info
#add_self $fe_addr || exit $?
check_and_register $fe_addrs
log_stderr "run start_broker.sh"
$DORIS_HOME/bin/start_broker.sh
