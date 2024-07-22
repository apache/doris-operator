#!/bin/bash
#

DORIS_HOME=${DORIS_HOME:="/opt/apache-doris"}

$DORIS_HOME/fe/bin/stop_fe.sh --grace
