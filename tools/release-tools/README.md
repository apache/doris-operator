<!--
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
-->

# Doris Operator release tools

These scripts package, sign, and publish Apache Doris Operator source releases,
publish the multi-platform operator image, and generate vote and announcement
email drafts. They do not create Git tags or send email.

The checked-in defaults target version and Git tag `26.0.0`.

## Prerequisites

Install `git`, `gpg`, `svn`, `svnmucc`, `sha512sum`, `curl`, `gzip`, `tar`, and
Docker with the Buildx plugin. The selected Git tag must already exist locally
and on the remote configured by `GIT_REMOTE`.

Edit `release.env` before each release. In particular, verify:

- `VERSION`, `TAG`, `GIT_REMOTE`, and all derived artifact/SVN paths.
- `DOCKER_IMAGE_REPOSITORY` and `DOCKER_PLATFORMS`.
- `APACHE_ID`, `APACHE_EMAIL`, and `SIGNER_NAME`.
- `SIGNING_KEY`: required full fingerprint of a locally available secret key.
- release notes, verification, download, and mailing-list URLs.
- `WORK_DIR`, which stores generated artifacts, SVN working copies, and drafts.

`TAG` must be the final version with no RC suffix. Source artifacts and SVN
version directories also use the final version:

```text
apache-doris-operator-26.0.0-src.tar.gz
apache-doris-operator-26.0.0-src.tar.gz.asc
apache-doris-operator-26.0.0-src.tar.gz.sha512
```

SVN credentials are never stored in `release.env` or generated mail files.
Export them in the shell that runs a publishing script:

```bash
export ASF_USERNAME="<apache-id>"
export ASF_PASSWORD="<apache-ldap-password>"
```

Find the full signing-key fingerprint with:

```bash
gpg --list-secret-keys --keyid-format=long --with-fingerprint
```

Copy that fingerprint into `release.env`; `01-check-env.sh` exits immediately
with this guidance when `SIGNING_KEY` is empty.

## Workflow

Run commands from this directory:

```bash
cd tools/release-tools
```

### 1. Check the signing environment

```bash
./01-check-env.sh
```

This requires the signing-key fingerprint configured in `release.env`, checks
the required tools, configures `GPG_TTY`, checks the recommended SHA-512 GPG
preferences, verifies the shared Doris `KEYS` file, and performs a local
sign/verify test. Editing `gpg.conf` and appending a public key to the dev and
release `KEYS` files each require explicit confirmation.

### 2. Package a release candidate when needed

```bash
./02-package-sign-upload.sh
```

This verifies that the local and remote tags resolve to the same commit,
creates a deterministic source archive with `git archive` and `gzip -n`, signs
it, writes a SHA-512 sidecar, and uploads the three source files to:

```text
https://dist.apache.org/repos/dist/dev/doris/doris-operator/<version>/
```

If the configured dev SVN directory already exists, skip this step or select a
new version. The script refuses to overwrite an existing version directory and
asks for confirmation both before staging and before commit.

### 3. Generate the vote email

```bash
./03-vote-mail.sh
```

This writes `vote-email.txt` and `vote-email.eml` under `WORK_DIR`.
It prints the subject and body, then leaves sending to the release manager.

### 4. Build and push the operator image

```bash
docker login
./04-build-image-push.sh
```

Run this only after the source-release vote passes. The script verifies that
the local and remote Git tags resolve to the same commit, extracts that tag into
a temporary clean build context, and displays the platforms and both image tags:

```text
apache/doris:operator-<version>
apache/doris:operator-latest
```

After one final confirmation, it runs one multi-platform Docker Buildx build for
`linux/amd64` and `linux/arm64` and pushes both tags. Docker Hub credentials are
read from Docker's configured credential store; this toolkit does not accept or
store the password.

### 5. Complete a passed release

```bash
./05-release-complete.sh
```

This step requires `svn` and `svnmucc`. The script verifies that the voted
version directory exists in dev SVN and that the matching release SVN directory
does not exist. After one final confirmation, it performs an atomic `svnmucc mv`
from dev SVN to release SVN:

```text
https://dist.apache.org/repos/dist/dev/doris/doris-operator/<version>/
  -> https://dist.apache.org/repos/dist/release/doris/doris-operator/<version>/
```

The repository-side move preserves the exact artifacts that passed the vote and
removes the version directory from dev SVN as part of the same commit. It does
not rebuild, re-sign, or checksum the source package. Only after a successful
move does it create `announce-email.txt` and `announce-email.eml`.

To regenerate only the announcement drafts:

```bash
./05-release-complete.sh --mail-only
```

`--mail-only` skips the SVN promotion.

## Safety boundaries

- No script creates, updates, or pushes a Git tag.
- Local and remote tags are compared by peeled commit ID.
- Generated signatures and checksums are verified immediately.
- Operator images are built from the verified Git tag, not the current working
  tree, and are pushed only after a final confirmation showing both tags.
- Docker credentials remain in Docker's credential store.
- Dev uploads stop before checkout if the version directory exists.
- Formal releases stop unless the dev version exists and the release version
  does not, then use one confirmed atomic SVN move.
- SVN target URLs and staged files are shown before both confirmations.
- SVN uploads contain only the source archive, signature, and checksum.
- Public emails are drafts only.
- No email was sent by any script; the release manager sends drafts manually.
- Formal release completion preserves the artifacts that passed the vote.

## Tests

The suite uses temporary Git repositories and fake GPG, Docker, and SVN
commands. It does not modify a real keyring, container registry, remote Git
repository, SVN repository, or mail system.

```bash
./tests/run.sh
```
