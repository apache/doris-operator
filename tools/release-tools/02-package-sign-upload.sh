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
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=release.env
source "${HERE}/release.env"
# shellcheck source=lib/release-common.sh
source "${HERE}/lib/release-common.sh"

validate_release_config
require_tools git gpg svn sha512sum gzip || die "install the missing release prerequisites"
export GPG_TTY="$(tty 2>/dev/null || true)"

SIGNER="$(resolve_signing_key)"
ok "signer: ${SIGNER}"
verify_tag_consistency
prepare_source_artifacts "$SIGNER"

stage_and_commit_version_dir \
  "$DEV_SVN_BASE" \
  "$DEV_SVN_DIR" \
  "dev-svn" \
  "Add Apache Doris Operator ${VERSION} release candidate" \
  "${SOURCE_ARTIFACTS[@]}"

if [[ "$SVN_COMMITTED" -eq 1 ]]; then
  ok "candidate uploaded; next run ./03-vote-mail.sh"
fi
