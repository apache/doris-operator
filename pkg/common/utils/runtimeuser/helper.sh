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
# This volume is exclusive to the helper and main container. The main process
# may only write requests; it cannot forge helper readiness or replace mounts.
chown 0:0 /doris-helper
chmod 0755 /doris-helper
mkdir -p /doris-helper/request /doris-helper/cgroup
chown "$TARGET_UID:$TARGET_GID" /doris-helper/request
chmod 0700 /doris-helper/request
rm -f /doris-helper/ready
touch /doris-helper/initialized
cleanup() {
 rm -f /doris-helper/ready
 for target in query cgroup.subtree_control cgroup.procs cgroup.type; do
  mountpoint -q "/doris-helper/cgroup/$target" && umount "/doris-helper/cgroup/$target" || true
 done
}
trap cleanup EXIT
trap 'exit 0' TERM INT
self=$(awk -F: '$1=="0"{print $3}' /proc/self/cgroup)
# Support systemd cgroup v2; reject unsupported layouts rather than creating a
# second hierarchy outside the Kubernetes resource limits.
[[ -f /host-cgroup/cgroup.controllers ]] || { echo 'cgroup v2 is required' >&2; exit 1; }
pod_component="pod${POD_UID//-/_}.slice"
last=''
while true; do
 request=$(cat /doris-helper/request/process 2>/dev/null || true)
 if [[ ! "$request" =~ ^([0-9]+):([0-9]+)$ ]]; then sleep 1; continue; fi
 pid=${BASH_REMATCH[1]}; start=${BASH_REMATCH[2]}
 if [[ ! -r /proc/$pid/stat ]]; then cleanup; last=''; sleep 1; continue; fi
 current_start=$(awk '{print $22}' /proc/$pid/stat)
 [[ "$start" == "$current_start" ]] || { cleanup; last=''; sleep 1; continue; }
 if [[ "$request" == "$last" ]]; then sleep 1; continue; fi
 rel=$(awk -F: '$1=="0"{print $3}' /proc/$pid/cgroup)
 uid=$(awk '/^Uid:/{print $2}' /proc/$pid/status)
 # Only the main container shares this control volume with the helper. In
 # particular, the pause container can have the same UID but must not qualify.
 [[ "$(stat -Lc '%d:%i' /proc/$pid/root/doris-helper/request)" == "$(stat -Lc '%d:%i' /doris-helper/request)" ]] || { echo 'Refusing process without the main control mount' >&2; exit 1; }
 [[ "$uid" == "$TARGET_UID" && "$rel" != "$self" ]] || { echo 'Refusing foreign process identity' >&2; exit 1; }
 [[ "$rel" == /kubepods.slice/*"$pod_component"/cri-containerd-*.scope ]] || { echo 'Refusing cgroup outside this Pod' >&2; exit 1; }
 cg="/host-cgroup$rel"
 [[ "$(realpath -e "$cg")" == "$cg" ]] || exit 1
 cleanup
 mkdir -p "$cg/doris"
 echo threaded > "$cg/doris/cgroup.type"
 echo +cpu > "$cg/cgroup.subtree_control"
 # Doris writes its entire process into <home>/query/cgroup.procs before
 # assigning individual worker threads to query/<wg-id>. Make query an alias
 # of the container's memory domain, not a threaded child. The BE leader then
 # stays beside the real memory.max; old images need no memory-reader patch.
 # Delegate directory creation and process/thread membership, never resource
 # limit files such as cpu.max or memory.max, or the domain's cgroup.type.
 chown "$TARGET_UID:$TARGET_GID" "$cg" "$cg/cgroup.procs" "$cg/cgroup.threads" "$cg/cgroup.subtree_control"
 [[ "$(awk '{print $22}' /proc/$pid/stat)" == "$start" ]] || continue
 mkdir -p /doris-helper/cgroup/query
 for file in cgroup.type cgroup.procs cgroup.subtree_control; do
  touch "/doris-helper/cgroup/$file"
  mount --bind "$cg/$file" "/doris-helper/cgroup/$file"
 done
 mount --bind "$cg" /doris-helper/cgroup/query
 # Some images skip CPU quota detection when /proc/self/cgroup is '/'. Supply
 # a default from the actual container quota and the bootstrap's CPU affinity.
 # Explicit positive num_cores in the user's config still takes precedence.
 affinity=$(awk '/^Cpus_allowed_list:/{print $2}' /proc/$pid/status)
 cores=0
 IFS=, read -ra ranges <<< "$affinity"
 for range in "${ranges[@]}"; do
  first=${range%-*}; last_cpu=${range#*-}
  cores=$((cores + last_cpu - first + 1))
 done
 read -r quota period < "$cg/cpu.max"
 if [[ "$quota" != max ]]; then
  quota_cores=$(((quota + period - 1) / period))
  (( quota_cores >= cores )) || cores=$quota_cores
 fi
 (( cores > 0 )) || { echo 'Cannot determine container CPU capacity' >&2; exit 1; }
 printf '%s\n' "$cores" > /doris-helper/cpu-cores
 printf '%s\n' "$request" > /doris-helper/ready
 echo "Delegated $rel as query domain; cores=$cores; cpu.max=$(cat "$cg/cpu.max"); memory.max=$(cat "$cg/memory.max")"
 last=$request
 sleep 1
done
