#!/bin/bash

FE_QUERY_PORT=${FE_QUERY_PORT:-9030}
PROBE_TIMEOUT=60
PROBE_INTERVAL=2
HEARTBEAT_PORT=9050
MY_SELF=
MY_IP=`hostname -i`
MY_HOSTNAME=`hostname -f`
DORIS_ROOT=${DORIS_ROOT:-"/opt/apache-doris"}
DORIS_HOME=${DORIS_ROOT}/be
BE_CONFIG=$DORIS_HOME/conf/be.conf
REGISTERED=false


log_stderr()
{
    echo "[`date`] $@" >&2
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

show_backends(){
    local svc=$1
    timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u root --skip-column-names --batch -e 'SHOW BACKENDS;'
}

# get all registered fe in cluster.
function show_frontends()
{
    local addr=$1
    echo ""
    timeout 15 mysql  --connect-timeout 2 -h $addr -P $FE_QUERY_PORT -u root --skip-column-names --batch -e 'show frontends;'
}

parse_confval_from_cn_conf()
{
    # a naive script to grep given confkey from cn conf file
    # assume conf format: ^\s*<key>\s*=\s*<value>\s*$
    local confkey=$1
    local confvalue=`grep "\<$confkey\>" $BE_CONFIG | grep -v '^\s*#' | sed 's|^\s*'$confkey'\s*=\s*\(.*\)\s*$|\1|g'`
    echo "$confvalue"
}

collect_env_info()
{
    # heartbeat_port from conf file
    local heartbeat_port=`parse_confval_from_cn_conf "heartbeat_service_port"`
    if [[ "x$heartbeat_port" != "x" ]] ; then
        HEARTBEAT_PORT=$heartbeat_port
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
        memlist=`show_backends $svc`
        if echo "$memlist" | grep -q -w "$MY_SELF" &>/dev/null ; then
            log_stderr "Check myself ($MY_SELF:$HEARTBEAT_PORT)  exist in FE start be ..."
            break;
        fi

        fe_memlist=`show_frontends $svc`
        local leader=`echo "$fe_memlist" | grep '\<FOLLOWER\>' | awk -F '\t' '{if ($8=="true") print $2}'`
        if [[ "x$leader" != "x" ]]; then
            log_stderr "Check myself ($MY_SELF:$HEARTBEAT_PORT)  not exist in FE and fe have leader register myself..."
            timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u root --skip-column-names --batch -e "ALTER SYSTEM ADD BACKEND \"$MY_SELF:$HEARTBEAT_PORT\";"
            let "expire=start+timeout"
            now=`date +%s`
            if [[ $expire -le $now ]] ; then
                log_stderr "Time out, abort!"
                return 0
            fi
        else
            log_stderr "not have leader wait fe cluster elect a master, sleep 2s..."
            sleep $PROBE_INTERVAL
        fi
    done
}

function check_and_register()
{
    $addrs=$1
    addrArr=(${addrs//,/ })
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

fe_addr=$1
if [[ "x$fe_addr" == "x" ]]; then
    echo "need fe address as paramter!"
    echo "  Example $0 <fe_addr>"
    exit 1
fi

update_conf_from_configmap
collect_env_info
#add_self $fe_addr || exit $?
check_and_register $fe_addr
log_stderr "run start_be.sh"
$DORIS_HOME/bin/start_be.sh
