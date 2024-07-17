#!/bin/bash
#

PROBE_TYPE=$1
DORIS_HOME=${DORIS_HOME:="/opt/apache-doris"}
CONFIG_FILE="$DORIS_HOME/be/conf/be.conf"
DEFAULT_HEARTBEAT_SERVICE_PORT=9050
DEFAULT_WEBSERVER_PORT=8040

function parse_config_file_with_key()
{
    local key=$1
    local value=`grep "^\s*$key\s*=" $CONFIG_FILE | sed "s|^\s*$key\s*=\s*\(.*\)\s*$|\1|g"`
    echo $value
}

function alive_probe()
{
    local heartbeat_service_port=$(parse_config_file_with_key "heartbeat_service_port")
    heartbeat_service_port=${heartbeat_service_port:=$DEFAULT_HEARTBEAT_SERVICE_PORT}
    if netstat -lntp | grep ":$heartbeat_service_port" > /dev/null ; then
        exit 0
    else
        exit 1
    fi
}

function ready_probe()
{
    local webserver_port=$(parse_config_file_with_key "webserver_port")
    webserver_port=${webserver_port:=$DEFAULT_WEBSERVER_PORT}
    local ip=`hostname -i | awk '{print $1}'`
    local url="http://${ip}:${webserver_port}/api/health"
    local res=$(curl -s $url)
    local status=$(jq -r ".status" <<< $res)
    if [[ "x$status" == "xOK" ]]; then
        exit 0
    else
        exit 1
    fi
}

if [[ "$PROBE_TYPE" == "ready" ]]; then
    ready_probe
else
    alive_probe
fi
