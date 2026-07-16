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

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

repo="${tmp}/repo"
remote="${tmp}/remote.git"
git init -q "$repo"
git -C "$repo" config user.name "Release Test"
git -C "$repo" config user.email "release-test@example.com"
printf 'release content\n' > "${repo}/content.txt"
git -C "$repo" add content.txt
git -C "$repo" commit -q -m initial
git -C "$repo" tag 9.9.9
git init -q --bare "$remote"
git -C "$repo" push -q "$remote" refs/tags/9.9.9

VERSION="9.9.9"
TAG="$VERSION"
REPO_DIR="$repo"
GIT_REMOTE="$remote"
PKG_BASE="apache-doris-operator-${VERSION}-src"
ARCHIVE_PREFIX="${PKG_BASE}/"
WORK_DIR="${tmp}/work"

verify_tag_consistency > "${tmp}/tag-output"
assert_file_contains "${tmp}/tag-output" "tag 9.9.9 resolves"

create_source_archive
assert_exists "$SOURCE_ARCHIVE"
gzip -dc "$SOURCE_ARCHIVE" | tar -tf - > "${tmp}/tar-list"
if awk -v prefix="${ARCHIVE_PREFIX}" 'index($0, prefix) != 1 { exit 1 }' "${tmp}/tar-list"; then
  :
else
  fail "archive contains an entry outside ${ARCHIVE_PREFIX}"
fi
first_digest="$(sha512sum "$SOURCE_ARCHIVE" | awk '{print $1}')"
create_source_archive
second_digest="$(sha512sum "$SOURCE_ARCHIVE" | awk '{print $1}')"
assert_eq "$first_digest" "$second_digest"

fake_bin="${tmp}/fake-bin"
mkdir -p "$fake_bin"
cat > "${fake_bin}/gpg" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
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

real_path="$PATH"
PATH="${fake_bin}:${PATH}"
SIGNING_KEY="0123456789ABCDEF0123456789ABCDEF01234567"
signer="$(resolve_signing_key)"
assert_eq "$SIGNING_KEY" "$signer"
sign_and_verify "$SOURCE_ARCHIVE" "$signer"
checksum_and_verify "$SOURCE_ARCHIVE"
assert_exists "${SOURCE_ARCHIVE}.asc"
assert_exists "${SOURCE_ARCHIVE}.sha512"
assert_file_contains "${SOURCE_ARCHIVE}.sha512" "$(basename "$SOURCE_ARCHIVE")"

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

export PATH
export FAKE_SVN_LOG="${tmp}/svn.log"
: > "$FAKE_SVN_LOG"
unset FAKE_EXISTING_URL || true
DEV_SVN_BASE="https://dist.example.test/dev/doris-operator"
DEV_SVN_DIR="${DEV_SVN_BASE}/${VERSION}"
ASF_USERNAME="release-user"
ASF_PASSWORD="release-password"
export ASF_USERNAME ASF_PASSWORD
SOURCE_ARTIFACTS=("$SOURCE_ARCHIVE" "${SOURCE_ARCHIVE}.asc" "${SOURCE_ARCHIVE}.sha512")

stage_and_commit_version_dir "$DEV_SVN_BASE" "$DEV_SVN_DIR" "dev-svn" "test commit" \
  "${SOURCE_ARTIFACTS[@]}" <<< $'y\ny'
assert_eq "1" "$SVN_COMMITTED"
assert_file_contains "$FAKE_SVN_LOG" "info --non-interactive --no-auth-cache"
assert_file_contains "$FAKE_SVN_LOG" "$DEV_SVN_DIR"
assert_file_contains "$FAKE_SVN_LOG" "commit"

: > "$FAKE_SVN_LOG"
export FAKE_EXISTING_URL="$DEV_SVN_DIR"
if (stage_and_commit_version_dir "$DEV_SVN_BASE" "$DEV_SVN_DIR" "dev-svn" "test commit" \
  "${SOURCE_ARTIFACTS[@]}" <<< $'y\ny') >/dev/null 2>&1; then
  fail "SVN upload accepted an existing version directory"
fi
assert_file_not_contains "$FAKE_SVN_LOG" "checkout"
assert_file_not_contains "$FAKE_SVN_LOG" "commit"

PATH="$real_path"
pass
