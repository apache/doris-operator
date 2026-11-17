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
if [[ -d "$FE_HEAP_SOURCE" ]]; then
    cp -RL "$FE_HEAP_SOURCE/." "$FE_HEAP_OUTPUT/"
fi
image_conf="$FE_HEAP_HOME/conf/fe.conf"
conf="$FE_HEAP_OUTPUT/fe.conf"
if [[ ! -f "$conf" ]]; then
    cp "$image_conf" "$conf"
fi

# Read only image defaults, without evaluating their shell expressions. User
# JAVA_OPTS entries are preserved as whole settings, including percentage heaps.
adapt_heap() {
    local key="$1" heap="$2" preserve="$3" replacement
    if [[ "$preserve" == true ]] || { [[ -f "$FE_HEAP_SOURCE/fe.conf" ]] &&
        grep -Eq "^[[:space:]]*${key}[[:space:]]*=" "$FE_HEAP_SOURCE/fe.conf"; }; then
        return
    fi
    replacement=$(awk -v key="$key" -v heap="$heap" '
        $0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
            value = $0
            sub(/^[^=]*=[[:space:]]*/, "", value)
            sub(/[[:space:]]*$/, "", value)
            quote = substr(value, 1, 1)
            if (quote == "\"" || quote == "\047") {
                value = substr(value, 2, length(value) - 2)
            } else {
                quote = "\""
            }
            gsub(/(^|[[:space:]])-Xm[sx][^[:space:]]*/, "", value)
            gsub(/(^|[[:space:]])-XX:(InitialRAMPercentage|MaxRAMPercentage|InitialHeapSize|MaxHeapSize)=[^[:space:]]*/, "", value)
            print key "=" quote value " " heap quote
            exit
        }
    ' "$image_conf")
    # Do not manufacture JVM options for JDKs unsupported by this image.
    if [[ -z "$replacement" ]]; then
        return
    fi
    awk -v key="$key" '$0 !~ "^[[:space:]]*" key "[[:space:]]*=" { print }' "$conf" > "$conf.tmp"
    printf '\n%s\n' "$replacement" >> "$conf.tmp"
    mv "$conf.tmp" "$conf"
}

adapt_heap JAVA_OPTS "-Xmx${FE_HEAP_JDK8_MIB}m" "$FE_HEAP_PRESERVE_JDK8"
adapt_heap JAVA_OPTS_FOR_JDK_17 '-XX:InitialRAMPercentage=25 -XX:MaxRAMPercentage=50' "$FE_HEAP_PRESERVE_JDK17"
