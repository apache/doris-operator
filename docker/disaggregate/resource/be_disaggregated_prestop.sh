#!/bin/bash
#

DORIS_HOME=${DORIS_HOME:="/opt/apache-doris"}

$DORIS_HOME/be/bin/stop_be.sh --grace
