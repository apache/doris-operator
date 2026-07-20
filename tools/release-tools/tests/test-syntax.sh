#!/usr/bin/env bash
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
# shellcheck source=testlib.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testlib.sh"

for script in "${TOOLS_ROOT}"/0*.sh "${TOOLS_ROOT}"/tests/run.sh "${TOOLS_ROOT}"/tests/test-*.sh; do
  [[ -x "$script" ]] || fail "script is not executable: $script"
done

bash -n \
  "${TOOLS_ROOT}"/release.env \
  "${TOOLS_ROOT}"/0*.sh \
  "${TOOLS_ROOT}"/lib/*.sh \
  "${TOOLS_ROOT}"/tests/*.sh

readme="${TOOLS_ROOT}/README.md"
for text in \
  './01-check-env.sh' \
  './02-package-sign-upload.sh' \
  './03-vote-mail.sh' \
  './04-release-complete.sh' \
  '--mail-only' \
  'refuses to overwrite' \
  'does not inspect, compare, promote, move, or delete anything under dev SVN' \
  'prints the subject and body' \
  'independently from dev SVN' \
  'freshly packages the selected Git tag' \
  'No email was sent' \
  './tests/run.sh'; do
  assert_file_contains "$readme" "$text"
done

assert_file_not_contains "$readme" 'initial `26.0.0` dev directory'

pass
