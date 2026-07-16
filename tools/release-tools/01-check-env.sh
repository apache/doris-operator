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

require_configured_signing_key
validate_release_config

printf '== Apache Doris Operator %s signing environment ==\n' "$VERSION"

problems=0
for tool in git gpg svn svnmucc sha512sum curl gzip; do
  if command -v "$tool" >/dev/null 2>&1; then
    ok "tool available: ${tool}"
  else
    warn "missing required tool: ${tool}"
    problems=$((problems + 1))
  fi
done

if [[ "$problems" -gt 0 ]]; then
  die "${problems} required tool(s) missing"
fi

export GPG_TTY="$(tty 2>/dev/null || true)"
ok "GPG_TTY=${GPG_TTY:-<not attached to a terminal>}"

gpg_config="$(gpg_config_file)"
if recommended_gpg_settings_present "$gpg_config"; then
  ok "GnuPG SHA-512 preferences are configured"
else
  warn "recommended SHA-512 settings are missing from ${gpg_config}"
  if confirm "Append the recommended SHA-512 settings?"; then
    append_recommended_gpg_settings "$gpg_config"
    ok "updated ${gpg_config}"
  else
    problems=$((problems + 1))
  fi
fi

SIGNER="$(resolve_signing_key)"
ok "signing-key fingerprint: ${SIGNER}"

if published_key_exists "$SIGNER"; then
  ok "signing key is present in ${KEYS_URL}"
else
  warn "signing key is not present in ${KEYS_URL}"
  if confirm "Append this key to both Doris dev and release KEYS files?"; then
    publish_key_to_doris_keys "$SIGNER"
  else
    problems=$((problems + 1))
  fi
fi

if test_signing_key "$SIGNER"; then
  ok "local sign-and-verify test succeeded"
else
  warn "local sign-and-verify test failed"
  problems=$((problems + 1))
fi

if [[ -n "${ASF_USERNAME:-}" && -n "${ASF_PASSWORD:-}" ]]; then
  ok "ASF_USERNAME and ASF_PASSWORD are present in the environment"
else
  warn "ASF_USERNAME and ASF_PASSWORD are not both set; SVN publishing will require them"
fi

if [[ "$problems" -gt 0 ]]; then
  die "environment is not ready (${problems} problem(s))"
fi
ok "environment looks READY for ${VERSION}"
