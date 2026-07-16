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

if [[ -n "${RELEASE_COMMON_SH_LOADED:-}" ]]; then
  return 0
fi
RELEASE_COMMON_SH_LOADED=1

ok() { printf '[ OK ] %s\n' "$*"; }
warn() { printf '[WARN] %s\n' "$*" >&2; }
die() { printf '[FAIL] %s\n' "$*" >&2; exit 1; }

confirm() {
  local answer
  read -r -p "$1 [y/N] " answer || return 1
  case "$answer" in
    y|Y|yes|YES|Yes) return 0 ;;
    *) return 1 ;;
  esac
}

require_tools() {
  local tool missing=0
  for tool in "$@"; do
    if ! command -v "$tool" >/dev/null 2>&1; then
      warn "missing required tool: ${tool}"
      missing=1
    fi
  done
  [[ "$missing" -eq 0 ]]
}

require_config_value() {
  local name="$1"
  [[ -n "${!name:-}" ]] || die "release.env: ${name} must not be empty"
}

require_configured_signing_key() {
  if [[ -n "${SIGNING_KEY:-}" ]]; then
    return 0
  fi

  {
    printf '[FAIL] SIGNING_KEY is required in release.env.\n'
    printf 'Find available secret signing keys with:\n'
    printf '  gpg --list-secret-keys --keyid-format=long --with-fingerprint\n'
    printf 'Copy the full fingerprint into release.env, for example:\n'
    printf '  SIGNING_KEY="<full fingerprint>"\n'
  } >&2
  return 1
}

validate_release_config() {
  local mode="${1:-full}" name
  local required=(
    ROOT VERSION TAG PKG_BASE ARCHIVE_PREFIX
    DEV_SVN_BASE DEV_SVN_DIR RELEASE_SVN_BASE RELEASE_SVN_DIR
    KEYS_URL DEV_KEYS_SVN_BASE RELEASE_KEYS_SVN_BASE
    APACHE_ID APACHE_EMAIL SIGNER_NAME GITHUB_TAG_URL RELEASE_NOTES_URL
    VERIFY_GUIDE_URL DOWNLOAD_PAGE_URL VOTE_TO ANNOUNCE_TO WORK_DIR
  )

  case "$mode" in
    full) required+=(REPO_DIR GIT_REMOTE) ;;
    image) required+=(REPO_DIR GIT_REMOTE DOCKER_IMAGE_REPOSITORY DOCKER_PLATFORMS) ;;
    mail|release) ;;
    *) die "unknown release configuration validation mode: ${mode}" ;;
  esac

  for name in "${required[@]}"; do
    require_config_value "$name"
  done

  [[ "$TAG" == "$VERSION" ]] || die "release.env: TAG must equal VERSION"
  [[ "$PKG_BASE" == "apache-doris-operator-${VERSION}-src" ]] ||
    die "release.env: PKG_BASE must be apache-doris-operator-${VERSION}-src"
  [[ "$ARCHIVE_PREFIX" == "${PKG_BASE}/" ]] ||
    die "release.env: ARCHIVE_PREFIX must be ${PKG_BASE}/"
  [[ "$DEV_SVN_DIR" == "${DEV_SVN_BASE}/${VERSION}" ]] ||
    die "release.env: DEV_SVN_DIR must be ${DEV_SVN_BASE}/${VERSION}"
  [[ "$RELEASE_SVN_DIR" == "${RELEASE_SVN_BASE}/${VERSION}" ]] ||
    die "release.env: RELEASE_SVN_DIR must be ${RELEASE_SVN_BASE}/${VERSION}"
  [[ "$WORK_DIR" == /* ]] || die "release.env: WORK_DIR must be an absolute path"

  if [[ "$mode" == "full" || "$mode" == "image" ]]; then
    [[ "$REPO_DIR" == /* ]] || die "release.env: REPO_DIR must be an absolute path"
  fi
}

list_secret_key_fingerprints() {
  gpg --batch --with-colons --list-secret-keys 2>/dev/null |
    awk -F: '$1 == "sec" { want_fingerprint = 1; next }
      want_fingerprint && $1 == "fpr" { print $10; want_fingerprint = 0 }'
}

signing_key_fingerprint() {
  local key="$1" fingerprint
  fingerprint="$(
    gpg --batch --with-colons --fingerprint --list-secret-keys "$key" 2>/dev/null |
      awk -F: '$1 == "sec" { want_fingerprint = 1; next }
        want_fingerprint && $1 == "fpr" { print $10; exit }'
  )"
  [[ -n "$fingerprint" ]] || die "cannot resolve fingerprint for signing key: ${key}"
  printf '%s\n' "$fingerprint"
}

resolve_signing_key() {
  local fingerprints count

  if [[ -n "${SIGNING_KEY:-}" ]]; then
    gpg --batch --list-secret-keys "$SIGNING_KEY" >/dev/null 2>&1 ||
      die "configured SIGNING_KEY is not a usable secret key: ${SIGNING_KEY}"
    signing_key_fingerprint "$SIGNING_KEY"
    return
  fi

  fingerprints="$(list_secret_key_fingerprints)"
  count="$(printf '%s\n' "$fingerprints" | awk 'NF { count++ } END { print count + 0 }')"
  case "$count" in
    1) printf '%s\n' "$fingerprints" ;;
    0) die "no usable secret key found; run ./01-check-env.sh first" ;;
    *) die "multiple secret keys found; set SIGNING_KEY in release.env" ;;
  esac
}

verify_tag_consistency() {
  local local_commit remote_refs remote_commit

  git -C "$REPO_DIR" show-ref --verify --quiet "refs/tags/${TAG}" ||
    die "local tag ${TAG} does not exist in ${REPO_DIR}"
  local_commit="$(git -C "$REPO_DIR" rev-parse "${TAG}^{commit}")"

  remote_refs="$(
    git -C "$REPO_DIR" ls-remote --tags "$GIT_REMOTE" \
      "refs/tags/${TAG}" "refs/tags/${TAG}^{}"
  )" || die "failed to query tag ${TAG} from ${GIT_REMOTE}"
  remote_commit="$(
    printf '%s\n' "$remote_refs" |
      awk -v direct="refs/tags/${TAG}" -v peeled="refs/tags/${TAG}^{}" '
        $2 == direct { direct_id = $1 }
        $2 == peeled { peeled_id = $1 }
        END { if (peeled_id != "") print peeled_id; else print direct_id }'
  )"

  [[ -n "$remote_commit" ]] || die "remote tag ${TAG} does not exist on ${GIT_REMOTE}"
  [[ "$local_commit" == "$remote_commit" ]] ||
    die "tag mismatch for ${TAG}: local=${local_commit}, ${GIT_REMOTE}=${remote_commit}"
  ok "tag ${TAG} resolves to ${local_commit} locally and on ${GIT_REMOTE}"
}

SOURCE_ARCHIVE=""
SOURCE_SIGNATURE=""
SOURCE_CHECKSUM=""
SOURCE_ARTIFACTS=()

create_source_archive() {
  local archive temporary
  mkdir -p "$WORK_DIR"
  archive="${WORK_DIR}/${PKG_BASE}.tar.gz"
  temporary="${archive}.tmp.$$"
  rm -f "$temporary" "$archive"

  if ! (
    set -o pipefail
    git -C "$REPO_DIR" archive --format=tar --prefix="$ARCHIVE_PREFIX" "$TAG" |
      gzip -n > "$temporary"
  ); then
    rm -f "$temporary"
    die "failed to create source archive from tag ${TAG}"
  fi

  mv "$temporary" "$archive"
  SOURCE_ARCHIVE="$archive"
  ok "source archive created: ${SOURCE_ARCHIVE}"
}

sign_and_verify() {
  local file="$1" signer="$2" signature="${1}.asc"
  rm -f "$signature"
  gpg --local-user "$signer" --armor --output "$signature" --detach-sign "$file"
  gpg --verify "$signature" "$file"
  ok "signature verified: ${signature}"
}

checksum_and_verify() {
  local file="$1" directory basename checksum
  directory="$(cd "$(dirname "$file")" && pwd)"
  basename="$(basename "$file")"
  checksum="${basename}.sha512"
  (
    cd "$directory"
    rm -f "$checksum"
    sha512sum "$basename" > "$checksum"
    sha512sum --check "$checksum"
  )
  ok "SHA-512 verified: ${file}.sha512"
}

prepare_source_artifacts() {
  local signer="$1"
  create_source_archive
  sign_and_verify "$SOURCE_ARCHIVE" "$signer"
  checksum_and_verify "$SOURCE_ARCHIVE"
  SOURCE_SIGNATURE="${SOURCE_ARCHIVE}.asc"
  SOURCE_CHECKSUM="${SOURCE_ARCHIVE}.sha512"
  SOURCE_ARTIFACTS=("$SOURCE_ARCHIVE" "$SOURCE_SIGNATURE" "$SOURCE_CHECKSUM")
}

SVN_AUTH_ARGS=()
SVN_COMMITTED=0

build_svn_auth_args() {
  SVN_AUTH_ARGS=(--non-interactive --no-auth-cache)
  [[ -n "${ASF_USERNAME:-}" ]] && SVN_AUTH_ARGS+=(--username "$ASF_USERNAME")
  [[ -n "${ASF_PASSWORD:-}" ]] && SVN_AUTH_ARGS+=(--password "$ASF_PASSWORD")
  return 0
}

svn_url_exists() {
  local url="$1"
  svn info "${SVN_AUTH_ARGS[@]}" "$url" >/dev/null 2>&1
}

stage_and_commit_version_dir() {
  local svn_base="$1" svn_dir="$2" stage_name="$3" commit_message="$4"
  shift 4
  local relative working_copy file

  [[ "$svn_dir" == "${svn_base}/"* ]] || die "SVN target is not below configured base: ${svn_dir}"
  relative="${svn_dir#${svn_base}/}"
  [[ -n "$relative" && "$relative" != */* ]] || die "SVN target must be one version directory: ${svn_dir}"
  [[ "$#" -gt 0 ]] || die "no files supplied for SVN upload"

  SVN_COMMITTED=0
  build_svn_auth_args
  if svn_url_exists "$svn_dir"; then
    die "SVN version directory already exists; refusing to overwrite: ${svn_dir}"
  fi

  printf 'Target SVN directory: %s/\n' "$svn_dir"
  printf 'Files to stage:\n'
  for file in "$@"; do
    [[ -f "$file" ]] || die "staged file does not exist: ${file}"
    printf '  %s\n' "$file"
  done

  if ! confirm "Checkout ${svn_base} and stage these files?"; then
    warn "stopped before modifying SVN"
    return 0
  fi

  mkdir -p "$WORK_DIR"
  working_copy="${WORK_DIR}/${stage_name}"
  rm -rf "$working_copy"
  svn checkout --depth empty "${SVN_AUTH_ARGS[@]}" "$svn_base" "$working_copy"
  mkdir -p "${working_copy}/${relative}"
  for file in "$@"; do
    cp "$file" "${working_copy}/${relative}/"
  done
  svn add "${working_copy}/${relative}"
  svn status "$working_copy"

  printf 'SVN commit target: %s/\n' "$svn_dir"
  if ! confirm "FINAL confirmation: commit the staged release files?"; then
    warn "staged working copy left at ${working_copy}"
    return 0
  fi

  svn commit "${SVN_AUTH_ARGS[@]}" -m "$commit_message" "$working_copy"
  SVN_COMMITTED=1
  ok "committed release files: ${svn_dir}/"
}

move_svn_version_dir() {
  local source_base="$1" source_dir="$2" target_base="$3" target_dir="$4" commit_message="$5"
  local source_relative target_relative

  [[ "$source_dir" == "${source_base}/"* ]] ||
    die "SVN source is not below configured base: ${source_dir}"
  source_relative="${source_dir#${source_base}/}"
  [[ -n "$source_relative" && "$source_relative" != */* ]] ||
    die "SVN source must be one version directory: ${source_dir}"

  [[ "$target_dir" == "${target_base}/"* ]] ||
    die "SVN target is not below configured base: ${target_dir}"
  target_relative="${target_dir#${target_base}/}"
  [[ -n "$target_relative" && "$target_relative" != */* ]] ||
    die "SVN target must be one version directory: ${target_dir}"

  SVN_COMMITTED=0
  build_svn_auth_args
  svn_url_exists "$source_dir" || die "dev SVN version directory does not exist: ${source_dir}"
  if svn_url_exists "$target_dir"; then
    die "release SVN version directory already exists; refusing to overwrite: ${target_dir}"
  fi

  printf 'SVN move source: %s/\n' "$source_dir"
  printf 'SVN move target: %s/\n' "$target_dir"
  if ! confirm "FINAL confirmation: move the voted release from dev SVN to release SVN?"; then
    warn "stopped before modifying SVN"
    return 0
  fi

  svnmucc "${SVN_AUTH_ARGS[@]}" -m "$commit_message" mv "$source_dir" "$target_dir"
  SVN_COMMITTED=1
  ok "moved release from ${source_dir}/ to ${target_dir}/"
}

gpg_config_file() {
  local home="${GNUPGHOME:-}"
  if [[ -z "$home" ]]; then
    home="$(gpgconf --list-dirs homedir 2>/dev/null || true)"
  fi
  if [[ -z "$home" ]]; then
    [[ -n "${HOME:-}" ]] || die "cannot locate the GnuPG home directory"
    home="${HOME}/.gnupg"
  fi
  printf '%s/gpg.conf\n' "$home"
}

recommended_gpg_settings_present() {
  local config="$1"
  grep -Eq '^[[:space:]]*personal-digest-preferences[[:space:]]+SHA512([[:space:]]|$)' "$config" 2>/dev/null &&
    grep -Eq '^[[:space:]]*cert-digest-algo[[:space:]]+SHA512([[:space:]]|$)' "$config" 2>/dev/null
}

append_recommended_gpg_settings() {
  local config="$1"
  mkdir -p "$(dirname "$config")"
  {
    printf '\n# ASF release signing recommendations\n'
    printf 'personal-digest-preferences SHA512\n'
    printf 'cert-digest-algo SHA512\n'
    printf 'default-preference-list SHA512 SHA384 SHA256 SHA224 AES256 AES192 AES CAST5 ZLIB BZIP2 ZIP Uncompressed\n'
  } >> "$config"
}

import_secret_key() {
  local key_file="$1"
  [[ -f "$key_file" ]] || die "secret key file not found: ${key_file}"
  gpg --import "$key_file"
}

generate_signing_key() {
  local real_name="$1" email="$2"
  [[ "${#real_name}" -ge 5 ]] || die "signing-key real name must contain at least five characters"
  [[ "$email" == *@apache.org ]] || die "signing-key email must be an @apache.org address"
  gpg --quick-generate-key "${real_name} (CODE SIGNING KEY) <${email}>" rsa4096 sign never
}

key_in_keys_stream() {
  local fingerprint="$1" keyring result=1
  keyring="$(mktemp -d "${TMPDIR:-/tmp}/doris-operator-keys.XXXXXX")"
  chmod 700 "$keyring"
  if gpg --homedir "$keyring" --batch --import >/dev/null 2>&1 &&
    gpg --homedir "$keyring" --batch --list-keys "$fingerprint" >/dev/null 2>&1; then
    result=0
  fi
  rm -rf "$keyring"
  return "$result"
}

published_key_exists() {
  local fingerprint="$1"
  curl -fsSL "$KEYS_URL" | key_in_keys_stream "$fingerprint"
}

publish_key_to_doris_keys() {
  local fingerprint="$1" spec name base working_copy
  build_svn_auth_args
  mkdir -p "$WORK_DIR"

  for spec in "dev=${DEV_KEYS_SVN_BASE}" "release=${RELEASE_KEYS_SVN_BASE}"; do
    name="${spec%%=*}"
    base="${spec#*=}"
    working_copy="${WORK_DIR}/keys-${name}"
    rm -rf "$working_copy"
    svn checkout --depth files "${SVN_AUTH_ARGS[@]}" "$base" "$working_copy"
    [[ -f "${working_copy}/KEYS" ]] || die "KEYS not found in ${base}"

    if key_in_keys_stream "$fingerprint" < "${working_copy}/KEYS"; then
      ok "signing key already present in ${name} KEYS"
      continue
    fi

    {
      printf '\n'
      gpg --list-sigs "$fingerprint"
      gpg --armor --export "$fingerprint"
    } >> "${working_copy}/KEYS"
    svn commit "${SVN_AUTH_ARGS[@]}" -m "Add KEYS entry for ${APACHE_ID}" "${working_copy}/KEYS"
    ok "appended signing key to ${base}/KEYS"
  done
}

test_signing_key() {
  local signer="$1" temporary signature result=0
  temporary="$(mktemp "${TMPDIR:-/tmp}/doris-operator-sign.XXXXXX")"
  signature="${temporary}.asc"
  printf 'Apache Doris Operator %s signing test\n' "$TAG" > "$temporary"
  if ! gpg --local-user "$signer" --armor --output "$signature" --detach-sign "$temporary" ||
    ! gpg --verify "$signature" "$temporary"; then
    result=1
  fi
  rm -f "$temporary" "$signature"
  return "$result"
}
