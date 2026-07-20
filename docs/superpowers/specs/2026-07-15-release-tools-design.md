# Doris Operator Release Tools Design

## Context

The `doris-operator` repository currently relies on manual release steps. This
change adds a small, reusable release toolkit modeled after
`apache/doris/tools/release-tools`, while preserving the existing Doris Operator
release conventions.

The first target release is `26.0.0` from the Git tag `26.0.0`.

The design intentionally keeps the current Operator naming scheme:

- Git tags use the final version only, such as `26.0.0`.
- SVN directories use the final version only.
- Source artifact names do not contain an RC suffix.
- A formal release is packaged and signed again from the Git tag. It does not
  promote, compare with, or reuse the artifacts already in dev SVN.

## Goals

The toolkit must provide the selected capabilities:

- A: shared release configuration
- B: environment and prerequisite checks
- C: GPG signing key and Doris `KEYS` management
- D: local and remote Git tag consistency checks
- E: source packaging from a tag
- H: upload of a release candidate to dev SVN
- I: generation of the vote email draft
- K: generation of the announcement email draft

The formal release flow must also package and sign the selected tag again, then
upload the new source artifacts to release SVN before generating the announcement
email.

## Non-goals

The toolkit will not:

- Create, update, or push Git tags.
- Build or push container images.
- Package or publish Helm charts.
- Create GitHub releases or edit GitHub release notes.
- Send public email automatically.
- Count votes or generate the vote result email.
- Validate ASF release policy.
- Reuse, compare, move, or delete the dev SVN artifacts during formal release.

## Chosen Structure

The repository will add `tools/release-tools` with four numbered entry points and
a shared library:

```text
tools/release-tools/
├── 01-check-env.sh
├── 02-package-sign-upload.sh
├── 03-vote-mail.sh
├── 04-release-complete.sh
├── README.md
├── release.env
├── lib/
│   └── release-common.sh
└── tests/
    ├── run.sh
    └── test-*.sh
```

The numbered scripts keep the workflow familiar to Doris release managers. The
shared library owns validation, tag checks, packaging, signing, checksums, and
common path handling so the dev and formal release flows cannot drift silently.

## Configuration

Every entry point sources `release.env`. The initial file will target `26.0.0`
and remain reusable for later versions.

Required or derived configuration includes:

```bash
VERSION="26.0.0"
TAG="${VERSION}"
GIT_REMOTE="upstream-apache"

PKG_BASE="apache-doris-operator-${VERSION}-src"
ARCHIVE_PREFIX="${PKG_BASE}/"

DEV_SVN_BASE="https://dist.apache.org/repos/dist/dev/doris/doris-operator"
DEV_SVN_DIR="${DEV_SVN_BASE}/${VERSION}"
RELEASE_SVN_BASE="https://dist.apache.org/repos/dist/release/doris/doris-operator"
RELEASE_SVN_DIR="${RELEASE_SVN_BASE}/${VERSION}"

KEYS_URL="https://downloads.apache.org/doris/KEYS"
DEV_KEYS_SVN_BASE="https://dist.apache.org/repos/dist/dev/doris"
RELEASE_KEYS_SVN_BASE="https://dist.apache.org/repos/dist/release/doris"
```

The file also defines the Apache ID, Apache email, signer display name, required
signing-key fingerprint, release notes URL, verification guide URL, download URL,
mailing-list addresses, and work directory.

The scripts read SVN credentials only from `ASF_USERNAME` and `ASF_PASSWORD` in
the environment. They never store credentials in `release.env` or generated
files.

## Shared Library

`lib/release-common.sh` will expose focused functions for these operations:

- Validate required configuration and derived paths.
- Resolve exactly one usable signing key or honor `SIGNING_KEY`.
- Build SVN authentication argument arrays from environment variables.
- Confirm state-changing operations.
- Verify that the local tag exists.
- Resolve local and remote tags to commit IDs and compare them.
- Create the source archive from the selected tag.
- Create and verify detached ASCII-armored GPG signatures.
- Create and verify SHA-512 checksum files.
- Stage and commit one version directory to a configured SVN root.

The source package command will use `git archive` with the prefix
`apache-doris-operator-<VERSION>-src/`. Compression will use deterministic gzip
metadata so repeated packaging of the same Git tree does not add a timestamp to
the gzip header.

## Script Flows

### 01-check-env.sh

This script prepares and validates the local signing environment.

It will:

1. Require `SIGNING_KEY` in `release.env` and explain how to find the full
   fingerprint when it is missing.
2. Validate `release.env`.
3. Check for `git`, `gpg`, `svn`, `svnmucc`, `sha512sum`, `curl`, and `gzip`.
4. Set `GPG_TTY` so GPG can request a passphrase.
5. Check the recommended SHA-512 GPG settings and offer to append them.
6. Resolve the configured signing key.
7. Check whether the public key appears in the shared Doris `KEYS` file.
8. Offer to append the public key to both Doris dev and release `KEYS` files.
9. Run a local sign-and-verify test.
10. Report whether SVN credentials are present.

Every state-changing key operation requires confirmation.

### 02-package-sign-upload.sh

This script prepares a dev SVN release candidate for a version whose dev folder
does not already exist.

It will:

1. Validate the environment and resolve the signer.
2. Verify that local and remote `TAG` values resolve to the same commit.
3. Create `apache-doris-operator-<VERSION>-src.tar.gz` from the tag.
4. Create and verify its `.asc` signature.
5. Create and verify its `.sha512` checksum.
6. Check whether `DEV_SVN_DIR` already exists.
7. Stop without modifying SVN if the directory exists.
8. Display the target URL and files.
9. Require two confirmations before committing the source archive and sidecars
    to dev SVN.

The script refuses to overwrite an existing dev SVN version directory.

### 03-vote-mail.sh

This script generates a vote email that follows the existing Doris Operator
mailing-list format.

The message will include:

- The `Apache Doris Operator <VERSION>` vote subject.
- The GitHub release tag URL.
- The configured release notes URL.
- The dev SVN candidate URL.
- The signing-key fingerprint and Apache email.
- The shared Doris `KEYS` URL.
- The Doris verification guide.
- The standard 72-hour vote choices.

The script writes `vote-email.txt` and `vote-email.eml` to `WORK_DIR`, prints the
subject and body for review, and tells the release manager to send it manually.

### 04-release-complete.sh

This script implements the user-specified formal release flow.

It will:

1. Validate the environment and resolve the signer.
2. Verify that local and remote `TAG` values resolve to the same commit.
3. Package the tag again into a new source tarball.
4. Create and verify a new detached signature.
5. Create and verify a new SHA-512 checksum.
6. Check whether `RELEASE_SVN_DIR` already exists.
7. Stop without modifying SVN if the release directory exists.
8. Display the release SVN target and staged files.
9. Require two confirmations before committing the new source archive and
    sidecars to release SVN.
10. Generate `announce-email.txt` and `announce-email.eml` after a successful
    commit.

The formal release flow does not inspect or modify `DEV_SVN_DIR`.

`--mail-only` skips packaging and SVN operations and regenerates only the
announcement drafts.

## Announcement Email

The announcement draft will include:

- Subject: `[ANNOUNCE] Apache Doris Operator <VERSION> release`.
- A short description of Doris Operator.
- The configured public download page or GitHub release URL.
- The formal source artifact URL.
- The release notes URL.
- A thank-you and signer name.

The recipient stays configurable. The script never sends the email.

## Safety and Error Handling

All scripts will use `set -euo pipefail` and quote path expansions.

State-changing operations use these safeguards:

- No script creates or pushes a Git tag.
- Tag checks compare commit IDs, not tag object IDs.
- SVN credentials remain in environment variables.
- SVN target URLs are printed before checkout or commit.
- Dev and release uploads stop if their version directory already exists.
- SVN commits require two explicit confirmations.
- Generated signatures and checksums are verified before upload.
- A failed command exits with a clear error and leaves existing SVN content
  untouched.
- Public emails are drafts only.

## Testing

Shell tests will run without modifying external services. Tests will place fake
`git`, `gpg`, `svn`, `svnmucc`, and checksum commands at the front of `PATH` or
use temporary local repositories where practical.

The suite will cover:

- Configuration validation and the `26.0.0` defaults.
- Artifact and archive-prefix naming.
- Local and remote tag commit comparison.
- Source archive creation from the configured tag.
- Signature and checksum creation and verification.
- Exact dev and release SVN targets.
- Refusal to overwrite an existing SVN version directory.
- Vote email fields and Operator-specific wording.
- Announcement email fields.
- Formal release packaging that does not read or alter dev SVN.
- `--mail-only` behavior.
- Shell syntax checks for every script.

`tests/run.sh` will provide one command for the complete release-tool test suite.

## Acceptance Criteria

The work is complete when:

1. All four scripts and `release.env` are documented and executable.
2. `01-check-env.sh` can validate and prepare the selected signing environment.
3. `02-package-sign-upload.sh` can safely prepare a future Operator candidate.
4. `03-vote-mail.sh` creates reviewable Operator vote email drafts.
5. `04-release-complete.sh` can freshly package and sign Tag `26.0.0`, stage the
   three source files for the configured release SVN directory, and generate the
   announcement drafts.
6. Neither upload path overwrites an existing SVN version directory.
7. No script sends email or stores credentials.
8. The offline test suite passes.
