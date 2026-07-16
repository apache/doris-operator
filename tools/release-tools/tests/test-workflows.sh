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
repo="${tmp}/repo"
remote="${tmp}/remote.git"
fake_bin="${tmp}/fake-bin"
mkdir -p "$fake_bin"

git init -q "$repo"
git -C "$repo" config user.name "Release Test"
git -C "$repo" config user.email "release-test@example.com"
printf 'source\n' > "${repo}/source.txt"
git -C "$repo" add source.txt
git -C "$repo" commit -q -m initial
git -C "$repo" tag 9.9.9
git init -q --bare "$remote"
git -C "$repo" push -q "$remote" refs/tags/9.9.9

cat > "${tool_copy}/release.env" <<EOF
ROOT="${tool_copy}"
REPO_DIR="${repo}"
VERSION="9.9.9"
TAG="\${VERSION}"
GIT_REMOTE="${remote}"
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

export FAKE_GPG_LOG="${tmp}/gpg.log"
export FAKE_SVN_LOG="${tmp}/svn.log"
: > "$FAKE_GPG_LOG"
: > "$FAKE_SVN_LOG"

cat > "${fake_bin}/gpg" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_GPG_LOG"
if [[ " $* " == *" --with-colons "* ]]; then
  printf 'sec:-:4096:1:TESTKEY:0:0:::::::scESC:::+:::23:\n'
  printf 'fpr:::::::::0123456789ABCDEF0123456789ABCDEF01234567:\n'
  exit 0
fi
if [[ " $* " == *" --list-secret-keys "* ]]; then
  exit 0
fi
output=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --output) output="$2"; shift 2 ;;
    --verify) [[ -f "$2" && -f "$3" ]]; exit 0 ;;
    *) shift ;;
  esac
done
[[ -n "$output" ]] || exit 1
printf 'fake signature\n' > "$output"
EOF
chmod +x "${fake_bin}/gpg"

cat > "${fake_bin}/svn" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_SVN_LOG"
command="$1"
shift
case "$command" in
  info)
    url="${@: -1}"
    [[ -n "${FAKE_EXISTING_URL:-}" && "$url" == "$FAKE_EXISTING_URL" ]]
    ;;
  checkout)
    destination="${@: -1}"
    mkdir -p "$destination"
    ;;
  add|status|commit) exit 0 ;;
  *) exit 1 ;;
esac
EOF
chmod +x "${fake_bin}/svn"

test_path="${fake_bin}:${PATH}"
unset FAKE_EXISTING_URL || true
if ! printf 'y\ny\n' | PATH="$test_path" "${tool_copy}/02-package-sign-upload.sh" > "${tmp}/dev-output" 2>&1; then
  cat "${tmp}/dev-output" >&2
  fail "candidate workflow failed"
fi

source_archive="${tmp}/work/apache-doris-operator-9.9.9-src.tar.gz"
assert_exists "$source_archive"
assert_exists "${source_archive}.asc"
assert_exists "${source_archive}.sha512"
assert_file_contains "$FAKE_SVN_LOG" "https://dist.example.test/dev/doris/doris-operator/9.9.9"
staged_dev="${tmp}/work/dev-svn/9.9.9"
assert_exists "${staged_dev}/apache-doris-operator-9.9.9-src.tar.gz"
assert_exists "${staged_dev}/apache-doris-operator-9.9.9-src.tar.gz.asc"
assert_exists "${staged_dev}/apache-doris-operator-9.9.9-src.tar.gz.sha512"
staged_dev_count="$(find "$staged_dev" -maxdepth 1 -type f | wc -l | tr -d ' ')"
assert_eq "3" "$staged_dev_count"

: > "$FAKE_SVN_LOG"
: > "$FAKE_GPG_LOG"
if ! printf 'y\ny\n' | PATH="$test_path" "${tool_copy}/04-release-complete.sh" > "${tmp}/release-output" 2>&1; then
  cat "${tmp}/release-output" >&2
  fail "formal release workflow failed"
fi
assert_file_contains "$FAKE_SVN_LOG" "https://dist.example.test/release/doris/doris-operator/9.9.9"
assert_file_not_contains "$FAKE_SVN_LOG" "https://dist.example.test/dev/doris/doris-operator"
assert_file_contains "$FAKE_GPG_LOG" "apache-doris-operator-9.9.9-src.tar.gz"
staged_release="${tmp}/work/release-svn/9.9.9"
staged_release_count="$(find "$staged_release" -maxdepth 1 -type f | wc -l | tr -d ' ')"
assert_eq "3" "$staged_release_count"
assert_exists "${tmp}/work/announce-email.txt"
assert_exists "${tmp}/work/announce-email.eml"

: > "$FAKE_SVN_LOG"
export FAKE_EXISTING_URL="https://dist.example.test/dev/doris/doris-operator/9.9.9"
if printf 'y\ny\n' | PATH="$test_path" "${tool_copy}/02-package-sign-upload.sh" >/dev/null 2>&1; then
  fail "candidate workflow overwrote an existing SVN version directory"
fi
assert_file_not_contains "$FAKE_SVN_LOG" "checkout"
assert_file_not_contains "$FAKE_SVN_LOG" "commit"

pass
