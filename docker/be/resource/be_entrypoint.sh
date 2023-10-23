#!/bin/bash

# the fe query port for mysql.
FE_QUERY_PORT=${FE_QUERY_PORT:-9030}
# timeout for probe fe master.
PROBE_TIMEOUT=60
# interval time to probe fe.
PROBE_INTERVAL=2
# rpc port for fe communicate with be.
HEARTBEAT_PORT=9050
# fqdn or ip
MY_SELF=
MY_IP=`hostname -i`
MY_HOSTNAME=`hostname -f`
DORIS_ROOT=${DORIS_ROOT:-"/opt/apache-doris"}
DORIS_HOME=${DORIS_ROOT}/be
BE_CONFIG=$DORIS_HOME/conf/be.conf
# represents self in fe meta or not.
REGISTERED=false

DB_ADMIN_USER=${USER:-"root"}

DB_ADMIN_PASSWD=$PASSWD

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

# get all backends info to check self exist or not.
show_backends(){
    local svc=$1
    if [[ "x$DB_ADMIN_PASSWD" != "x" ]]; then
       timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u$DB_ADMIN_USER -p$DB_ADMIN_PASSWD --skip-column-names --batch -e 'SHOW BACKENDS;'
    else
       timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u$DB_ADMIN_USER --skip-column-names --batch -e 'SHOW BACKENDS;'
    fi
}

# get all registered fe in cluster, for check the fe have `MASTER`.
function show_frontends()
{
    local addr=$1
    if [[ "x$DB_ADMIN_PASSWD" != "x" ]]; then
        timeout 15 mysql --connect-timeout 2 -h $addr -P $FE_QUERY_PORT -u$DB_ADMIN_USER -p$DB_ADMIN_PASSWD --skip-column-names --batch -e 'show frontends;'
    else
        timeout 15 mysql --connect-timeout 2 -h $addr -P $FE_QUERY_PORT -u$DB_ADMIN_USER --skip-column-names --batch -e 'show frontends;'
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

        # check fe cluster have master, if fe have not master wait.
        fe_memlist=`show_frontends $svc`
        local leader=`echo "$fe_memlist" | grep '\<FOLLOWER\>' | awk -F '\t' '{if ($8=="true") print $2}'`
        if [[ "x$leader" != "x" ]]; then
            log_stderr "Check myself ($MY_SELF:$HEARTBEAT_PORT)  not exist in FE and fe have leader register myself..."

            if [[ "x$DB_ADMIN_PASSWD" != "x" ]]; then
                timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u$DB_ADMIN_USER -p$DB_ADMIN_PASSWD --skip-column-names --batch -e "ALTER SYSTEM ADD BACKEND \"$MY_SELF:$HEARTBEAT_PORT\";"
            else
                timeout 15 mysql --connect-timeout 2 -h $svc -P $FE_QUERY_PORT -u$DB_ADMIN_USER --skip-column-names --batch -e "ALTER SYSTEM ADD BACKEND \"$MY_SELF:$HEARTBEAT_PORT\";"
            fi

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

# check be exist or not, if exist return 0, or register self in fe cluster. when all fe address failed exit script.
# `xxx1:port,xxx2:port` as parameter to function.
function check_and_register()
{
    addrs=$1
    local addrArr=(${addrs//,/ })
    for addr in ${addrArr[@]}
    do
        add_self $addr

        if [[ $REGISTERED ]]; then
            break;
        fi
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
./doris-debug --component be
log_stderr "run start_be.sh"
# the server will start in the current terminal session, and the log output and console interaction will be printed to that terminal
# befor doris 2.0.2 ,doris start with : start_xx.sh
# sine doris 2.0.2 ,doris start with : start_xx.sh --console  doc: https://doris.apache.org/docs/dev/install/standard-deployment/#version--202
$DORIS_HOME/bin/start_be.sh --console

