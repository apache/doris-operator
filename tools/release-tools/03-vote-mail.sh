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
require_tools gpg || die "GnuPG is required to resolve the signing fingerprint"

SIGNER="$(resolve_signing_key)"
mkdir -p "$WORK_DIR"
subject="[VOTE] Release Apache Doris Operator ${VERSION}"
body_file="${WORK_DIR}/vote-email.txt"
eml_file="${WORK_DIR}/vote-email.eml"

BODY="$(
  cat <<EOF
Hi all,

Please review and vote on the Apache Doris Operator ${VERSION} release.

The Git tag is available here:
${GITHUB_TAG_URL}

Release notes:
${RELEASE_NOTES_URL}

The source release candidate, signature, and checksum are available here:
${DEV_SVN_DIR}/

The source artifact was signed with key ${SIGNER}, associated with ${APACHE_EMAIL}.
The shared Apache Doris KEYS file is available here:
${KEYS_URL}

Verification instructions:
${VERIFY_GUIDE_URL}

The vote will remain open for at least 72 hours.
[ ] +1 Approve the release
[ ] +0 No opinion
[ ] -1 Do not release this package because ...

Best regards,
${SIGNER_NAME} (${APACHE_ID})
EOF
)"

printf '%s\n' "$BODY" > "$body_file"
{
  printf 'To: %s\n' "$VOTE_TO"
  printf 'Subject: %s\n' "$subject"
  printf 'Content-Type: text/plain; charset=UTF-8\n'
  printf '\n%s\n' "$BODY"
} > "$eml_file"

ok "vote draft: ${body_file}"
ok "mail draft: ${eml_file}"
printf 'Subject: %s\n' "$subject"
printf '%s\n' '----------------------------------------------------------------'
printf '%s\n' "$BODY"
printf '%s\n' '----------------------------------------------------------------'
printf 'Review and send the message manually to %s. No email was sent.\n' "$VOTE_TO"
