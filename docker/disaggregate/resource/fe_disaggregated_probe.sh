#!/bin/bash
#

DORIS_HOEM=${DORIS_HOME:="/opt/apache-doris"}
CONFIG_FILE="$DORIS_HOME/fe/conf/fe.conf"
DEFAULT_HTTP_PORT=8030
DEFAULT_QUERY_PORT=9030
PROBE_TYPE=$1

function parse_config_file_with_key()
{
    local key=$1
    local value=`grep "^\s*$key\s*=" $CONFIG_FILE | sed "s|^\s*$key\s*=\s*\(.*\)\s*$|\1|g"`
    echo $value
}

function alive_probe()
{
    local query_port=$(parse_config_file_with_key "query_port")
    query_port=${query_port:=$DEFAULT_QUERY_PORT}
    if netstat -lntp | grep ":$query_port" > /dev/null ; then
        exit 0
    else
        exit 1
    fi
}

function ready_probe()
{
    local http_port=$(parse_config_file_with_key "http_port")
    http_port=${http_port:=$DEFAULT_HTTP_PORT}
    local ip=`hostname -i | awk '{print $1}'`
    local url="http://${ip}:${http_port}/api/health"
    local res=$(curl -s $url)
    local code=$(jq -r ".code" <<< $res)
    if [[ "x$code" == "x0" ]]; then
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
