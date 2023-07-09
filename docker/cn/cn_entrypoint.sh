#!/bin/bash

HOST_TYPE=${HOST_TYPE:-"IP"}
FE_QUERY_PORT=${FE_QUERY_PORT:-9030}
HEARTBEAT_PORT=9050
MY_SELF=
MY_IP=`hostname -i`
MY_HOSTNAME=`hostname -f`
DORIS_ROOT=${DORIS_ROOT:-"/opt/doris"}
#  If Doris has its own CN profile, it will be split in the future
DORIS_CN_HOME=${DORIS_ROOT}/be
CN_CONFIG=${DORIS_CN_HOME}/conf/be.conf
# time out
PROBE_TIMEOUT=60
# sleep interval
PROBE_INTERVAL=2

# log module
doris_log() {
  local type="$1"
  shift
  # accept argument string or stdin
  local text="$*"
  if [ "$#" -eq 0 ]; then text="$(cat)"; fi
  local dt="$(date -Iseconds)"
  printf '%s [%s] [Entrypoint]: %s\n' "$dt" "$type" "$text"
}
doris_note() {
  doris_log Note "$@"
}
doris_warn() {
  doris_log Warn "$@" >&2
}
doris_error() {
  doris_log ERROR "$@" >&2
}

#从环境变量获取变量ENV_FE_ADDR，变量的形态为数组[ip:port,domain:port]
#get_feaddr(){
#
#}

show_backends(){
  timeout 15 mysql --connect-timeout 2 -h $fe_host -P $FE_QUERY_PORT -u root --skip-column-names --batch -e "SHOW BACKENDS;"
}

#从fe中获取be的信息，根据注册的形态是FQDN还是IP检查本节点是否已注册成功。没有成功注册自己到fe
check_self_status(){
  # get heartbeat port
    local heartbeat_port=`get_configuration_from_config "heartbeat_service_port"`
    if [[ "x$heartbeat_port" != "x" ]]; then
        HEARTBEAT_PORT=$$heartbeat_port
    fi
    if [[ "x$HOST_TYPE" == "xIP" ]]; then
        MY_SELF=$MY_IP
    else
        MY_SELF=$MY_HOSTNAME
    fi
  # check self status and add self
    while true
    do
      doris_note "Add myself ($MY_SELF:$HEARTBEAT_PORT) into FE"
      timeout 15 mysql --connect-timeout 2 -h $fe_host -P $FE_QUERY_PORT -u root --skip-column-names --batch -e "ALTER SYSTEM ADD BACKEND \"$MY_SELF:$HEARTBEAT_PORT\";"
      be_list=`show_backends $fe_host`
      if echo "$be_list" | grep -q -w "$MY_SELF" &>/dev/null ; then
        doris_note "Add myself success"
        break;
      fi
      let "expire=start+timeout"
      now=`date +%s`
      if [[ $expire -le $now ]] ; then
          log_stderr "Time out, abort!"
          exit 1
      fi

      sleep $PROBE_INTERVAL
    done
}


update_config(){
  doris_note "add configuration to be.conf"
  echo "be_node_role=computation" >>$CN_CONFIG
  echo "priority_networks = ${MY_SELF}" >>$CN_CONFIG
}

# get conf value by conf key
get_configuration_from_config(){
   local confkey=$1
   local confvalue=`grep "\<$confkey\>" $CN_CONFIG | grep -v '^\s*#' | sed 's|^\s*'$confkey'\s*=\s*\(.*\)\s*$|\1|g'`
   echo "$confvalue"
}




back_conf_from_configmap(){
  if [[ "x$CONFIGMAP_MOUNT_PATH" == "x" ]]; then
      doris_error "Env var $CONFIGMAP_MOUNT_PATH is empty, skip it!"
  fi
  if ! test -d $$CONFIGMAP_MOUNT_PATH ; then
      doris_error "$CONFIGMAP_MOUNT_PATH not exists or not a dir,ignore ..."
  fi
  local confdir=$DORIS_CN_HOME/conf
   # shellcheck disable=SC2045
  for configfile in `ls $CONFIGMAP_MOUNT_PATH`
  do
      doris_note "config file $configfile ..."
      local conf=$confdir/$configfile
      if test -e $conf ; then
           # back up
          mv -f $conf ${conf}.bak
      fi
      ln -sfT $CONFIGMAP_MOUNT_PATH/$configfile $conf
  done
}



add_config_to_cn_conf(){
    doris_note "Start add cn config to be.conf!"
    echo "priority_networks = ${1}" >>$CN_CONFIG
    echo "be_node_role = computation" >>$CN_CONFIG
}


back_conf_from_configmap(){

  if [[ "x$CONFIGMAP_MOUNT_PATH" == "x" ]]; then
      doris_error "Env var $CONFIGMAP_MOUNT_PATH is empty, skip it!"
  fi
  if ! test -d $$CONFIGMAP_MOUNT_PATH ; then
      doris_error "$CONFIGMAP_MOUNT_PATH not exists or not a dir,ignore ..."
  fi
  # /opt/doris/be/conf
  local confdir=$DORIS_CN_HOME/conf
  # shellcheck disable=SC2045
  # /etc/doris
  for configfile in `ls $CONFIGMAP_MOUNT_PATH`
  do
      doris_note "config file $configfile ..."
      local conf=$confdir/$configfile
      # if /opt/doris/conf has configfile , do back up
      if test -e $conf ; then
          # back up
          mv -f $conf ${conf}.bak
      fi
      # ln /etc/doris/xx.conf to   /opt/doris/be/conf/xx.conf
      ln -sfT $CONFIGMAP_MOUNT_PATH/$configfile $conf
  done
}
doris_note "Start cn!"
$DORIS_CN_HOME/bin/start_be.sh




