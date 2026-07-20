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

These scripts package, sign, and publish Apache Doris Operator source releases.
They publish source artifacts only and generate vote and announcement email
drafts. They do not create Git tags or send email.

The checked-in defaults target version and Git tag `26.0.0`.

## Prerequisites

Install `git`, `gpg`, `svn`, `svnmucc`, `sha512sum`, `curl`, and `gzip`. The
selected Git tag must already exist locally and on the remote configured by
`GIT_REMOTE`.

Edit `release.env` before each release. In particular, verify:

- `VERSION`, `TAG`, `GIT_REMOTE`, and all derived artifact/SVN paths.
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

### 4. Complete a passed release

```bash
./04-release-complete.sh
```

The formal release is created independently from dev SVN. The script re-checks
the local and remote tag, freshly packages the selected Git tag, signs the new
archive, creates a fresh checksum, and uploads those newly generated source
files to:

```text
https://dist.apache.org/repos/dist/release/doris/doris-operator/<version>/
```

It does not inspect, compare, promote, move, or delete anything under dev SVN.
It refuses to overwrite an existing release directory and requires two
confirmations. Only after a successful commit does it create
`announce-email.txt` and `announce-email.eml`.

To regenerate only the announcement drafts:

```bash
./04-release-complete.sh --mail-only
```

`--mail-only` skips Git, packaging, GPG, checksums, and SVN.

## Safety boundaries

- No script creates, updates, or pushes a Git tag.
- Local and remote tags are compared by peeled commit ID.
- Generated signatures and checksums are verified immediately.
- Dev and release uploads stop before checkout if the version directory exists.
- SVN target URLs and staged files are shown before both confirmations.
- SVN uploads contain only the source archive, signature, and checksum.
- Public emails are drafts only.
- No email was sent by any script; the release manager sends drafts manually.
- Formal release packaging is independent from dev SVN.

## Tests

The suite uses temporary Git repositories and fake GPG/SVN commands. It does
not modify a real keyring, remote Git repository, SVN repository, or mail
system.

```bash
./tests/run.sh
```
