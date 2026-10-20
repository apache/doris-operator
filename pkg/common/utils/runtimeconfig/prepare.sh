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
store="/runtime-store/.doris-runtime-config/$POD_NAME"
mkdir -p "$store"
ordinal="${POD_NAME##*-}"
if (( ordinal < RUNTIME_MIGRATED_REPLICAS )) && [[ ! -f "$store/.migration-complete" ]]; then
    echo 'Runtime configuration migration is missing; refusing to start with empty overrides.' >&2
    exit 1
fi
if [[ -d /runtime-source ]]; then
    cp -RL /runtime-source/. /runtime-prepared/
fi
if [[ ! -f "/runtime-prepared/$RUNTIME_BASE" ]]; then
    cp "$RUNTIME_HOME/conf/$RUNTIME_BASE" "/runtime-prepared/$RUNTIME_BASE"
fi
custom="${RUNTIME_COMPONENT}_custom.conf"
key=custom_config_dir
value=/var/lib/doris/runtime-config
if [[ "$RUNTIME_COMPONENT" == ms ]]; then
    key=custom_conf_path
    value="$value/$custom"
fi
# ConfigMaps may contain initial overrides. Seed once, then remove the source
# entry so the image's entrypoint cannot symlink the mutable file to a ConfigMap.
if [[ ! -e "$store/$custom" ]]; then
    if [[ -f "/runtime-prepared/$custom" ]]; then
        cp "/runtime-prepared/$custom" "$store/$custom"
    else
        : > "$store/$custom"
    fi
fi
rm -f "/runtime-prepared/$custom"
# Remove prior assignments to avoid parser-specific duplicate-key behavior.
awk -v key="$key" '$0 !~ "^[[:space:]]*" key "[[:space:]]*=" {print}' "/runtime-prepared/$RUNTIME_BASE" > /runtime-prepared/base.tmp
printf '\n%s = %s\n' "$key" "$value" >> /runtime-prepared/base.tmp
mv /runtime-prepared/base.tmp "/runtime-prepared/$RUNTIME_BASE"
