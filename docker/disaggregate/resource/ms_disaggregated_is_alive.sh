#!/bin/bash
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.


log_stderr()
{
    echo "[`date`] $@" >&2
}

brpc_listen_port=$1
if [[ "x$brpc_listen_port" == "x" ]]; then
    echo "need disaggregated doris meta-service(or recycler) brpc_listen_port as paramter!"
    exit 1
fi

netstat -nltu | grep ":$brpc_listen_port " > /dev/null

if [ $? -eq 0 ]; then
#  log_stderr "disaggregated doris meta-service(or recycler) ($brpc_listen_port) alive, ProbeHandler ExecAction get exit 0"
  exit 0
else
#  log_stderr "disaggregated doris meta-service(or recycler) ($brpc_listen_port) not alive，ProbeHandler ExecAction get exit 1"
  exit 1
fi
