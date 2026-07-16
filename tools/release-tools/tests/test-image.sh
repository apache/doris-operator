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
mkdir -p "$repo" "$fake_bin"

git init -q "$repo"
git -C "$repo" config user.name "Release Test"
git -C "$repo" config user.email "release-test@example.com"
printf 'tagged source\n' > "${repo}/source.txt"
printf 'FROM scratch\nCOPY source.txt /source.txt\n' > "${repo}/Dockerfile"
git -C "$repo" add Dockerfile source.txt
git -C "$repo" commit -q -m tagged
git -C "$repo" tag 9.9.9
git init -q --bare "$remote"
git -C "$repo" push -q "$remote" refs/tags/9.9.9
printf 'working tree source\n' > "${repo}/source.txt"

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
DOCKER_IMAGE_REPOSITORY="apache/doris"
DOCKER_PLATFORMS="linux/amd64,linux/arm64"
EOF

export FAKE_DOCKER_LOG="${tmp}/docker.log"
cat > "${fake_bin}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_DOCKER_LOG"

if [[ "${1:-} ${2:-}" == "buildx version" ]]; then
  exit "${FAKE_BUILDX_VERSION_EXIT_CODE:-0}"
fi

if [[ "${1:-} ${2:-}" == "buildx build" ]]; then
  context="${@: -1}"
  printf 'context=%s\n' "$context" >> "$FAKE_DOCKER_LOG"
  printf 'context-source=%s\n' "$(tr -d '\n' < "${context}/source.txt")" >> "$FAKE_DOCKER_LOG"
  [[ -f "${context}/Dockerfile" ]]
  exit "${FAKE_DOCKER_BUILD_EXIT_CODE:-0}"
fi

exit 1
EOF
chmod +x "${fake_bin}/docker"

test_path="${fake_bin}:${PATH}"
: > "$FAKE_DOCKER_LOG"
if ! printf 'y\n' | PATH="$test_path" "${tool_copy}/04-build-image-push.sh" > "${tmp}/image-output" 2>&1; then
  cat "${tmp}/image-output" >&2
  fail "operator image workflow failed"
fi
assert_file_contains "$FAKE_DOCKER_LOG" "buildx version"
assert_file_contains "$FAKE_DOCKER_LOG" "buildx build --platform=linux/amd64,linux/arm64"
assert_file_contains "$FAKE_DOCKER_LOG" "-t apache/doris:operator-9.9.9"
assert_file_contains "$FAKE_DOCKER_LOG" "-t apache/doris:operator-latest"
assert_file_contains "$FAKE_DOCKER_LOG" "-f Dockerfile"
assert_file_contains "$FAKE_DOCKER_LOG" "--push"
assert_file_contains "$FAKE_DOCKER_LOG" "context-source=tagged source"
assert_file_not_contains "$FAKE_DOCKER_LOG" "context-source=working tree source"
build_context="$(sed -n 's/^context=//p' "$FAKE_DOCKER_LOG")"
assert_not_exists "$build_context"
assert_file_contains "${tmp}/image-output" "pushed operator images"

: > "$FAKE_DOCKER_LOG"
if ! printf 'n\n' | PATH="$test_path" "${tool_copy}/04-build-image-push.sh" > "${tmp}/decline-output" 2>&1; then
  fail "declining the operator image push returned failure"
fi
assert_file_not_contains "$FAKE_DOCKER_LOG" "buildx build"
assert_file_contains "${tmp}/decline-output" "stopped before building or pushing images"

: > "$FAKE_DOCKER_LOG"
export FAKE_DOCKER_BUILD_EXIT_CODE=42
if printf 'y\n' | PATH="$test_path" "${tool_copy}/04-build-image-push.sh" > "${tmp}/failure-output" 2>&1; then
  fail "operator image workflow accepted a failed Buildx push"
fi
unset FAKE_DOCKER_BUILD_EXIT_CODE
assert_file_contains "$FAKE_DOCKER_LOG" "buildx build"
failed_context="$(sed -n 's/^context=//p' "$FAKE_DOCKER_LOG")"
assert_not_exists "$failed_context"

: > "$FAKE_DOCKER_LOG"
export FAKE_BUILDX_VERSION_EXIT_CODE=1
if printf 'y\n' | PATH="$test_path" "${tool_copy}/04-build-image-push.sh" >/dev/null 2>&1; then
  fail "operator image workflow accepted a missing Buildx plugin"
fi
unset FAKE_BUILDX_VERSION_EXIT_CODE
assert_file_not_contains "$FAKE_DOCKER_LOG" "buildx build"

pass
