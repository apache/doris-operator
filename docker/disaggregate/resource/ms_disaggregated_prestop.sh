#!/bin/bash

#get from env
DORIS_HOME=${DORIS_HOME:="/opt/apache-doris"}

$DORIS_HOME/ms/bin/stop.sh --$1

