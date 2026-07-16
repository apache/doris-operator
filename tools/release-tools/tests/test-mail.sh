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

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
tool_copy="${tmp}/release-tools"
cp -R "$TOOLS_ROOT" "$tool_copy"
mkdir -p "${tmp}/repo" "${tmp}/fake-bin"

cat > "${tool_copy}/release.env" <<EOF
ROOT="${tool_copy}"
REPO_DIR="${tmp}/repo"
VERSION="9.9.9"
TAG="\${VERSION}"
GIT_REMOTE="test-remote"
PKG_BASE="apache-doris-operator-\${VERSION}-src"
ARCHIVE_PREFIX="\${PKG_BASE}/"
DEV_SVN_BASE="https://dist.example.test/dev/doris/doris-operator"
DEV_SVN_DIR="\${DEV_SVN_BASE}/\${VERSION}"
RELEASE_SVN_BASE="https://dist.example.test/release/doris/doris-operator"
RELEASE_SVN_DIR="\${RELEASE_SVN_BASE}/\${VERSION}"
KEYS_URL="https://downloads.example.test/doris/KEYS"
DEV_KEYS_SVN_BASE="https://dist.example.test/dev/doris"
RELEASE_KEYS_SVN_BASE="https://dist.example.test/release/doris"
APACHE_ID="release-manager"
APACHE_EMAIL="release-manager@apache.org"
SIGNER_NAME="Release Manager"
SIGNING_KEY="0123456789ABCDEF0123456789ABCDEF01234567"
GITHUB_TAG_URL="https://github.example.test/apache/doris-operator/releases/tag/\${TAG}"
RELEASE_NOTES_URL="https://github.example.test/apache/doris-operator/releases/notes/\${TAG}"
VERIFY_GUIDE_URL="https://doris.example.test/release-verify"
DOWNLOAD_PAGE_URL="https://github.example.test/apache/doris-operator/releases/tag/\${TAG}"
VOTE_TO="dev@example.test"
ANNOUNCE_TO="announce@example.test"
WORK_DIR="${tmp}/work"
EOF

export COMMAND_LOG="${tmp}/commands.log"
: > "$COMMAND_LOG"
cat > "${tmp}/fake-bin/gpg" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'gpg %s\n' "$*" >> "$COMMAND_LOG"
if [[ " $* " == *" --with-colons "* ]]; then
  printf 'sec:-:4096:1:TESTKEY:0:0:::::::scESC:::+:::23:\n'
  printf 'fpr:::::::::0123456789ABCDEF0123456789ABCDEF01234567:\n'
fi
exit 0
EOF
chmod +x "${tmp}/fake-bin/gpg"

PATH="${tmp}/fake-bin:${PATH}" "${tool_copy}/03-vote-mail.sh" > "${tmp}/vote-output"
vote_body="${tmp}/work/vote-email.txt"
vote_eml="${tmp}/work/vote-email.eml"
assert_exists "$vote_body"
assert_exists "$vote_eml"
assert_file_contains "$vote_eml" "Subject: [VOTE] Release Apache Doris Operator 9.9.9"
assert_file_contains "$vote_body" "https://github.example.test/apache/doris-operator/releases/tag/9.9.9"
assert_file_contains "$vote_body" "https://dist.example.test/dev/doris/doris-operator/9.9.9/"
assert_file_contains "$vote_body" "0123456789ABCDEF0123456789ABCDEF01234567"
assert_file_contains "$vote_body" "release-manager@apache.org"
assert_file_contains "$vote_body" "The vote will remain open for at least 72 hours."
assert_file_contains "$vote_body" "[ ] +1 Approve the release"
assert_file_contains "${tmp}/vote-output" "Subject: [VOTE] Release Apache Doris Operator 9.9.9"
assert_file_contains "${tmp}/vote-output" "No email was sent."

: > "$COMMAND_LOG"
PATH="${tmp}/fake-bin:${PATH}" "${tool_copy}/04-release-complete.sh" --mail-only > "${tmp}/announce-output"
announce_body="${tmp}/work/announce-email.txt"
announce_eml="${tmp}/work/announce-email.eml"
assert_exists "$announce_body"
assert_exists "$announce_eml"
assert_file_contains "$announce_eml" "To: announce@example.test"
assert_file_contains "$announce_eml" "Subject: [ANNOUNCE] Apache Doris Operator 9.9.9 release"
assert_file_contains "$announce_body" "automates the deployment and management"
assert_file_contains "$announce_body" "https://dist.example.test/release/doris/doris-operator/9.9.9/apache-doris-operator-9.9.9-src.tar.gz"
assert_file_contains "$announce_body" "Thank you to everyone"
assert_file_contains "${tmp}/announce-output" "mail-only mode: skipping tag, package, signing, and SVN operations"
[[ ! -s "$COMMAND_LOG" ]] || fail "--mail-only invoked an external release command"

if PATH="${tmp}/fake-bin:${PATH}" "${tool_copy}/04-release-complete.sh" --unknown >/dev/null 2>&1; then
  fail "04-release-complete.sh accepted an unknown argument"
fi

pass
