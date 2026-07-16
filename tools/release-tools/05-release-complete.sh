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

usage() {
  printf 'Usage: %s [--mail-only]\n' "$0"
  printf '  --mail-only  regenerate announcement drafts without SVN promotion\n'
}

mail_only=0
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --mail-only) mail_only=1 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown argument: $1" ;;
  esac
  shift
done

write_announce_email() {
  local subject body_file eml_file body
  mkdir -p "$WORK_DIR"
  subject="[ANNOUNCE] Apache Doris Operator ${VERSION} release"
  body_file="${WORK_DIR}/announce-email.txt"
  eml_file="${WORK_DIR}/announce-email.eml"

  body="$(
    cat <<EOF
Hi all,

We are pleased to announce the release of Apache Doris Operator ${VERSION}.

Apache Doris Operator automates the deployment and management of Apache Doris clusters on Kubernetes.

Release downloads:
${DOWNLOAD_PAGE_URL}

Formal source artifact:
${RELEASE_SVN_DIR}/${PKG_BASE}.tar.gz

Release notes:
${RELEASE_NOTES_URL}

Thank you to everyone who contributed to this release.

Best regards,
${SIGNER_NAME}
EOF
  )"

  printf '%s\n' "$body" > "$body_file"
  {
    printf 'To: %s\n' "$ANNOUNCE_TO"
    printf 'Subject: %s\n' "$subject"
    printf 'Content-Type: text/plain; charset=UTF-8\n'
    printf '\n%s\n' "$body"
  } > "$eml_file"

  ok "announcement draft: ${body_file}"
  ok "mail draft: ${eml_file}"
  printf '%s\n' '----------------------------------------------------------------'
  printf '%s\n' "$body"
  printf '%s\n' '----------------------------------------------------------------'
  printf 'Review and send the message manually to %s. No email was sent.\n' "$ANNOUNCE_TO"
}

if [[ "$mail_only" -eq 1 ]]; then
  validate_release_config mail
  ok "mail-only mode: skipping the SVN promotion"
  write_announce_email
  exit 0
fi

validate_release_config release
require_tools svn svnmucc || die "install the missing release prerequisites"
move_svn_version_dir \
  "$DEV_SVN_BASE" "$DEV_SVN_DIR" \
  "$RELEASE_SVN_BASE" "$RELEASE_SVN_DIR" \
  "Release Apache Doris Operator ${VERSION}"

if [[ "$SVN_COMMITTED" -eq 1 ]]; then
  write_announce_email
fi
