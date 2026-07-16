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

validate_release_config image
require_tools git tar docker || die "install the missing image release prerequisites"
verify_tag_consistency
docker buildx version >/dev/null 2>&1 || die "Docker Buildx is required to publish the operator image"

version_image="${DOCKER_IMAGE_REPOSITORY}:operator-${VERSION}"
latest_image="${DOCKER_IMAGE_REPOSITORY}:operator-latest"

printf 'Image source tag: %s\n' "$TAG"
printf 'Image platforms: %s\n' "$DOCKER_PLATFORMS"
printf 'Image tags to push:\n'
printf '  %s\n' "$version_image"
printf '  %s\n' "$latest_image"

if ! confirm "FINAL confirmation: build and push both operator image tags?"; then
  warn "stopped before building or pushing images"
  exit 0
fi

build_context=""
cleanup_build_context() {
  if [[ -n "$build_context" && -d "$build_context" ]]; then
    rm -rf "$build_context"
  fi
}
trap cleanup_build_context EXIT

build_context="$(mktemp -d "${TMPDIR:-/tmp}/doris-operator-image.XXXXXX")"
if ! git -C "$REPO_DIR" archive "$TAG" | tar -xf - -C "$build_context"; then
  die "failed to prepare the operator image build context from tag ${TAG}"
fi
[[ -f "${build_context}/Dockerfile" ]] || die "Dockerfile is missing from tag ${TAG}"

docker buildx build \
  --platform="$DOCKER_PLATFORMS" \
  -t "$version_image" \
  -t "$latest_image" \
  -f Dockerfile \
  --push \
  "$build_context"

ok "pushed operator images: ${version_image}, ${latest_image}"
