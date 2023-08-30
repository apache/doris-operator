#!/bin/bash
DORIS_ROOT=${DORIS_ROOT:-"/opt/apache-doris"}
# fe location
DORIS_HOME=${DORIS_ROOT}/fe
# participant election number of fe.
ELECT_NUMBER=${ELECT_NUMBER:=3}
# query port for mysql connection.
QUERY_PORT=${FE_QUERY_PORT:-9030}
# location of fe config store.
FE_CONFFILE=$DORIS_HOME/conf/fe.conf
# represents the type for fe communication: domain or IP.
START_TYPE=
# the master node in fe cluster.
FE_MASTER=
# pod number
POD_INDEX=
# probe interval: 2 seconds
PROBE_INTERVAL=2
# timeout for probe master: 120 seconds
PROBE_MASTER_POD0_TIMEOUT=60 # at most 30 attempts, no less than the times needed for an election
PROBE_MASTER_PODX_TIMEOUT=180 # at most 60 attempts
# administrator for administrate the cluster.
DB_ADMIN_USER=${USER:-"root"}

DB_ADMIN_PASSWD=$PASSWD
# myself as IP or FQDN
MYSELF=


function log_stderr()
{
  echo "[`date`] $@" >& 2
}

#parse the `$FE_CONFFILE` file, passing the key need resolve as parameter.
parse_confval_from_fe_conf()
{
    # a naive script to grep given confkey from fe conf file
    # assume conf format: ^\s*<key>\s*=\s*<value>\s*$
    local confkey=$1
    local confvalue=`grep "\<$confkey\>" $FE_CONFFILE | grep -v '^\s*#' | sed 's|^\s*'$confkey'\s*=\s*\(.*\)\s*$|\1|g'`
    echo "$confvalue"
}

# when image exist int doris-meta, use exist meta to start.
function start_fe_with_meta()
{
    log_stderr "start with meta run start_fe.sh"
    $DORIS_HOME/fe/bin/start_fe.sh
}

collect_env_info()
{
    # set POD_IP, POD_FQDN, POD_INDEX, EDIT_LOG_PORT, QUERY_PORT
    if [[ "x$POD_IP" == "x" ]] ; then
        POD_IP=`hostname -i | awk '{print $1}'`
    fi

    if [[ "x$POD_FQDN" == "x" ]] ; then
        POD_FQDN=`hostname -f`
    fi

    # example: fe-sr-deploy-1.fe-svc.kc-sr.svc.cluster.local
    POD_INDEX=`echo $POD_FQDN | awk -F'.' '{print $1}' | awk -F'-' '{print $NF}'`

    START_TYPE=`parse_confval_from_fe_conf "enable_fqdn_mode"`

    if [[ "x$START_TYPE" == "xtrue" ]]; then
        MYSELF=$POD_FQDN
    else
        MYSELF=$POD_IP
    fi

    # edit_log_port from conf file
    local edit_log_port=`parse_confval_from_fe_conf "edit_log_port"`
    if [[ "x$edit_log_port" != "x" ]] ; then
        EDIT_LOG_PORT=$edit_log_port
    fi

    # query_port from conf file
    local query_port=`parse_confval_from_fe_conf "query_port"`
    if [[ "x$query_port" != "x" ]] ; then
        QUERY_PORT=$query_port
    fi
}

# get all registered fe in cluster.
function show_frontends()
{
    local addr=$1
    if [[ "x$DB_ADMIN_PASSWD" != "x" ]]; then
        timeout 15 mysql --connect-timeout 2 -h $addr -P $QUERY_PORT -u$DB_ADMIN_USER -p$DB_ADMIN_PASSWD --skip-column-names --batch -e 'show frontends;'
    else
        timeout 15 mysql --connect-timeout 2 -h $addr -P $QUERY_PORT -u$DB_ADMIN_USER --skip-column-names --batch -e 'show frontends;'
    fi
}

# add myself in cluster for FOLLOWER.
function add_self_follower()
{
    if [[ "x$DB_ADMIN_PASSWD" != "x" ]]; then
        mysql --connect-timeout 2 -h $FE_MASTER -P $QUERY_PORT -u$DB_ADMIN_USER -p$DB_ADMIN_PASSWD --skip-column-names --batch -e "ALTER SYSTEM ADD FOLLOWER \"$MYSELF:$EDIT_LOG_PORT\";"
    else
        mysql --connect-timeout 2 -h $FE_MASTER -P $QUERY_PORT -u$DB_ADMIN_USER --skip-column-names --batch -e "ALTER SYSTEM ADD FOLLOWER \"$MYSELF:$EDIT_LOG_PORT\";"
    fi
}

# add myself in cluster for OBSERVER.
function add_self_observer()
{
    if [[ "x$DB_ADMIN_PASSWD" != "x" ]]; then
        mysql --connect-timeout 2 -h $FE_MASTER -P $QUERY_PORT -u$DB_ADMIN_USER -p$DB_ADMIN_PASSWD --skip-column-names --batch -e "ALTER SYSTEM ADD OBSERVER \"$MYSELF:$EDIT_LOG_PORT\";"
    else
        mysql --connect-timeout 2 -h $FE_MASTER -P $QUERY_PORT -u$DB_ADMIN_USER --skip-column-names --batch -e "ALTER SYSTEM ADD OBSERVER \"$MYSELF:$EDIT_LOG_PORT\";"
    fi
}

# `dori-meta/image` not exist start as first time.
function start_fe_no_meta()
{
    local opts=""
    local start=`date +%s`
    local has_member=false
    local member_list=
    if [[ "x$FE_MASTER" != "x" ]] ; then
        opts+=" --helper $FE_MASTER:$EDIT_LOG_PORT"
        local start=`date +%s`
        while true
        do
            if [[ ELECT_NUMBER -gt $POD_INDEX ]]; then
                log_stderr "Add myself($MYSELF:$EDIT_LOG_PORT) to master as follower ..."
                add_self_follower
            else
                log_stderr "Add myself($MYSELF:$EDIT_LOG_PORT) to master as observer ..."
                add_self_observer
            fi
               # check if added successfully.
            if show_frontends $addr | grep -q -w "$MYSELF" &>/dev/null ; then
                break;
            fi

            local now=`date +%s`
            let "expire=start+30" # 30s timeout
            if [[ $expire -le $now ]] ; then
                log_stderr "Timed out, abort!"
                exit 1
            fi

            log_stderr "Sleep a while and retry adding ..."
            sleep $PROBE_INTERVAL
        done
    fi
    log_stderr "first start with no meta run start_fe.sh with additional options: '$opts'"
    $DORIS_HOME/bin/start_fe.sh $opts
}

# the ordinal is 0, probe timeout as 60s, when have not meta and not `MASTER` in fe cluster, 0 start as master.
probe_master_for_pod0()
{
    # possible to have no result at all, because myself is the first FE instance in the cluster
    local svc=$1
    local start=`date +%s`
    local has_member=false
    local memlist=
    while true
    do
        memlist=`show_frontends $svc`
	    local master=`echo "$memlist" | grep '\<FOLLOWER\>' | awk -F '\t' '{if ($8=="true") print $2}'`
        if [[ "x$master" != "x" ]] ; then
            # has master, done
            log_stderr "Find master: $master!"
            FE_MASTER=$master
            return 0
        fi

        if [[ "x$memlist" != "x" ]] ; then
            # has memberlist ever before
            has_member=true
        fi

        # no master yet, check if needs timeout and quit
        log_stderr "No master yet, has_member: $has_member ..."
        local timeout=$PROBE_MASTER_POD0_TIMEOUT
        if $has_member ; then
            # set timeout to the same as PODX since there are other members
            timeout=$PROBE_MASTER_PODX_TIMEOUT
        fi

        local now=`date +%s`
        let "expire=start+timeout"
        if [[ $expire -le $now ]] ; then
            if $has_member ; then
                log_stderr "Timed out ${timeout}s, has members but not master abort!"
                exit 1
            else
                log_stderr "Timed out, no members detected ever, assume myself is the first node .."
                # empty FE_MASTER
                FE_MASTER=""
                return 0
            fi
        fi
        sleep $PROBE_INTERVAL
    done
}

# ordinal greater than 0, start as `FOLLOWER` or `OBSERVER`
probe_master_for_podX()
{
    # wait until find a master or timeout
    local svc=$1
    local start=`date +%s`
    while true
    do
        memlist=`show_frontends $svc`
	    local master=`echo "$memlist" | grep '\<FOLLOWER\>' | awk -F '\t' '{if ($8=="true") print $2}'`
        if [[ "x$master" != "x" ]] ; then
            # has master done
            log_stderr "Find master: $master!"
            FE_MASTER=$master
            return 0
        fi
        # no master yet, check if needs timeout and quit
        log_stderr "No master wait ${PROBE_INTERVAL}s..."

        local now=`date +%s`
        let "expire=start+PROBE_MASTER_PODX_TIMEOUT"
        if [[ $expire -le $now ]] ; then
            log_stderr "Probe master timeout, abort!"
            return 0
        fi

        sleep $PROBE_INTERVAL
    done
}

# when not meta exist, fe start should probe
probe_master()
{
    local svc=$1
    # resolve svc as array.
    local addArr=${svc//,/ }
    for addr in ${addArr[@]}
    do
        # if have master break for register or check.
        if [[ "x$FE_MASTER" != "x" ]]; then
            break
        fi

        # find master under current service and set to FE_MASTER
        if [[ "$POD_INDEX" -eq 0 ]] ; then
            probe_master_for_pod0 $addr
        else
            probe_master_for_podX $addr
    fi
    done

    # if first pod assume first start should as master. others first start have not master exit.
    if [[ "x$FE_MASTER" == "x" ]]; then
        if [[ "$POD_INDEX" -eq 0 ]]; then
            return 0
        else
            exit 1
        fi
    fi
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

start_fe_with_meta()
{
    local opts=""
    log_stderr "start with meta run start_fe.sh with additional options: '$opts'"
    $DORIS_HOME/bin/start_fe.sh $opts
}

fe_addrs=$1
if [[ "x$fe_addrs" == "x" ]]; then
    echo "need fe address as parameter!"
    exit
fi

update_conf_from_configmap
if [[ -f "/opt/apache-doris/fe/doris-meta/image/ROLE" ]]; then
    log_stderr "start fe with exist meta."
    start_fe_with_meta
else
    log_stderr "first start fe with meta not exist."
    collect_env_info
    probe_master $fe_addrs
    start_fe_no_meta
fi