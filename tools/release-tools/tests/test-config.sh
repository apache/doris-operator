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
# shellcheck source=../release.env
source "${TOOLS_ROOT}/release.env"
# shellcheck source=../lib/release-common.sh
source "${TOOLS_ROOT}/lib/release-common.sh"

assert_eq "26.0.0" "$VERSION"
assert_eq "$VERSION" "$TAG"
assert_eq "upstream-apache" "$GIT_REMOTE"
assert_eq "apache/doris" "$DOCKER_IMAGE_REPOSITORY"
assert_eq "linux/amd64,linux/arm64" "$DOCKER_PLATFORMS"
assert_eq "apache-doris-operator-26.0.0-src" "$PKG_BASE"
assert_eq "${PKG_BASE}/" "$ARCHIVE_PREFIX"
assert_eq "https://dist.apache.org/repos/dist/dev/doris/doris-operator/26.0.0" "$DEV_SVN_DIR"
assert_eq "https://dist.apache.org/repos/dist/release/doris/doris-operator/26.0.0" "$RELEASE_SVN_DIR"
assert_eq "https://downloads.apache.org/doris/KEYS" "$KEYS_URL"
assert_eq "https://dist.apache.org/repos/dist/dev/doris" "$DEV_KEYS_SVN_BASE"
assert_eq "https://dist.apache.org/repos/dist/release/doris" "$RELEASE_KEYS_SVN_BASE"

validate_release_config

error_file="$(mktemp)"
trap 'rm -f "$error_file"' EXIT
if (TAG="not-${VERSION}"; validate_release_config) 2>"$error_file"; then
  fail "configuration validation accepted a tag that differs from VERSION"
fi
assert_file_contains "$error_file" "TAG must equal VERSION"

if grep -Eq '^[[:space:]]*ASF_(USERNAME|PASSWORD)=' "${TOOLS_ROOT}/release.env"; then
  fail "release.env stores SVN credentials"
fi

pass
