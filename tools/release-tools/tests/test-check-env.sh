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

script="${TOOLS_ROOT}/01-check-env.sh"
common="${TOOLS_ROOT}/lib/release-common.sh"

for tool in git gpg svn svnmucc sha512sum curl gzip; do
  grep -qw "$tool" "$script" || fail "environment check omits required tool: $tool"
done

assert_file_contains "$script" 'export GPG_TTY='
assert_file_contains "$script" 'Append the recommended SHA-512 settings?'
assert_file_contains "$script" 'require_configured_signing_key'
assert_file_not_contains "$script" 'Import a secret key into this GnuPG keyring?'
assert_file_not_contains "$script" 'Generate a new RSA-4096 signing key with no expiry?'
assert_file_contains "$script" 'Append this key to both Doris dev and release KEYS files?'
assert_file_contains "$common" '"dev=${DEV_KEYS_SVN_BASE}" "release=${RELEASE_KEYS_SVN_BASE}"'
assert_file_contains "$common" 'test_signing_key()'

error_file="$(mktemp)"
trap 'rm -f "$error_file"' EXIT
if (SIGNING_KEY=""; require_configured_signing_key) 2>"$error_file"; then
  fail "empty SIGNING_KEY was accepted"
fi
assert_file_contains "$error_file" 'SIGNING_KEY is required in release.env'
assert_file_contains "$error_file" 'gpg --list-secret-keys --keyid-format=long --with-fingerprint'
assert_file_contains "$error_file" 'SIGNING_KEY="<full fingerprint>"'

(SIGNING_KEY="0123456789ABCDEF"; require_configured_signing_key)

if grep -Eq 'printf.*ASF_PASSWORD|echo.*ASF_PASSWORD' "$script"; then
  fail "environment check may print the SVN password"
fi

pass
