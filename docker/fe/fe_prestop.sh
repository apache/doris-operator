#!/bin/bash

DORIS_ROOT=${DORIS_ROOT:-"/opt/doris"}
DORIS_HOME=${DORIS_ROOT}/fe
$STARROCKS_HOME/bin/stop_fe.sh