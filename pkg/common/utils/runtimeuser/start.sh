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

set -euo pipefail
start=$(awk '{print $22}' /proc/$$/stat)
request="$$:$start"
printf '%s\n' "$request" > /doris-helper/request/process
ready=false
for ((i=0;i<120;i++)); do
 if [[ "$(cat /doris-helper/ready 2>/dev/null || true)" == "$request" ]] && mountpoint -q /doris-helper/cgroup; then ready=true; break; fi
 sleep 1
done
$ready || { echo 'Privileged helper did not delegate the CPU cgroup within 120s' >&2; exit 1; }
exec "$@"
