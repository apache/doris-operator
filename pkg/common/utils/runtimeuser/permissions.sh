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
mode=$1; shift
if [[ "$mode" == repair ]]; then
 uid=$1; gid=$2; shift 2
 for dir in "$@"; do
  [[ -d "$dir" && ! -L "$dir" ]] || { echo "Invalid migration directory: $dir" >&2; exit 1; }
  # No success marker in writable data: a non-root process could forge it.
  # The controller removes this initializer on the next same-user image upgrade.
  # GNU find does not follow symlinks; -xdev prevents crossing mount boundaries.
  echo "Preparing ownership for $dir ($uid:$gid)"
  find -P "$dir" -xdev ! -type l \( ! -uid "$uid" -o ! -gid "$gid" \) -exec chown --no-dereference "$uid:$gid" {} +
  find -P "$dir" -xdev -type d ! -perm -u=rwx -exec chmod u+rwx {} +
  find -P "$dir" -xdev -type f ! -perm -u=rw -exec chmod u+rw {} +
 done
else
 for dir in "$@"; do
  [[ -d "$dir" && -r "$dir" && -w "$dir" && -x "$dir" ]] || { echo "Volume is not accessible by $(id): $dir" >&2; exit 1; }
  if [[ "$mode" == verify-deep ]]; then
   # Root-to-non-root is the only full-tree check. In particular, external/shared
   # PVCs are checked without changing permissions under concurrent writers.
   bad=$(find -P "$dir" -xdev \( -type d \( ! -readable -o ! -writable -o ! -executable \) -o -type f \( ! -readable -o ! -writable \) \) -print -quit)
   [[ -z "$bad" ]] || { echo "Storage administrator must grant $(id) access to: $bad" >&2; exit 1; }
  fi
  probe=$(mktemp -d "$dir/.doris-permission-check.XXXXXXXX")
  touch "$probe/a"; mv "$probe/a" "$probe/b"; rm "$probe/b"; rmdir "$probe"
 done
fi
