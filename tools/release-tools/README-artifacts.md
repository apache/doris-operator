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

# Build and publish release artifacts

The Build Release Artifacts workflow builds container images and Helm charts
from an existing Git tag and uploads the files to GitHub Actions. Publish the
images and charts from your workstation with write access to the corresponding
destinations. [Workflow definition](../../.github/workflows/release-artifacts.yml)

## Build and download the artifacts

1. Confirm that the release tag exists in `apache/doris-operator`. You need
   repository write access to run the workflow.
2. In [GitHub Actions](https://github.com/apache/doris-operator/actions), select
   **Build Release Artifacts** and click **Run workflow**. Select `master` as
   the workflow branch and enter the tag name, for example `26.0.1`, in the
   `tag` field. GitHub loads the workflow from `master` and the workflow checks
   out the source from the requested tag.
3. After the run succeeds, open **Summary** and click **Download release
   artifacts for 26.0.1**. Sign in to GitHub to download the ZIP. Unzip it and
   run the publishing commands from the directory that contains `images/` and
   `charts/`.

You can also start the same workflow with the GitHub CLI.

```bash
gh workflow run release-artifacts.yml --repo apache/doris-operator --ref master -f tag=26.0.1
```

The ZIP contains `images/doris.k8s-operator.oci.tar` and
`images/doris-debug-ubuntu.oci.tar`. Each archive contains `linux/amd64` and
`linux/arm64`. The `charts/` directory contains the `doris` and `doris-operator`
chart packages and an index for those packages.

## Push the images to Docker Hub

Install Skopeo and `jq`. Run `skopeo login docker.io` and enter your Docker Hub
username and a write-enabled access token at the password prompt. Use an
account with write access to each destination or sign in with the appropriate
account before each copy. [Skopeo installation](https://github.com/podman-container-tools/skopeo/blob/main/install.md)
· [Docker Hub access tokens](https://docs.docker.com/security/access-tokens/personal-access-tokens/)

Set `tag` to the tag from the workflow run. The Operator destination below
matches the `DOCKER_IMAGE` value that the source release scripts use in the
release notes. The Debug destination matches the previous publishing workflow.
To publish the Operator to its previous repository, set `operator_image` to
`docker.io/selectdb/doris.k8s-operator:${tag}`. [Release configuration](release.env)

```bash
tag='<release-tag>'
operator_image="docker.io/apache/doris:operator-${tag}"
debug_image="docker.io/selectdb/doris-debug-ubuntu:${tag}"

skopeo copy --all "oci-archive:images/doris.k8s-operator.oci.tar" "docker://${operator_image}"
skopeo copy --all "oci-archive:images/doris-debug-ubuntu.oci.tar" "docker://${debug_image}"
```

`--all` copies both platform images and their manifest index. After pushing,
inspect each registry tag and confirm that it lists `linux/amd64` and `linux/arm64`.
[Skopeo copy options](https://github.com/podman-container-tools/skopeo/blob/main/docs/skopeo-copy.1.md)

```bash
for image in "$operator_image" "$debug_image"; do
  printf '%s\n' "$image"
  skopeo inspect --raw "docker://${image}" |
    jq -r '.manifests[].platform | "\(.os)/\(.architecture)"'
done
```

## Publish the Helm charts

Install Helm and configure `ossutil` locally for the `selectdb-charts` bucket in
OSS China (Beijing), using `oss-cn-beijing.aliyuncs.com` and credentials with
write access to the bucket. [OSS upload documentation](https://www.alibabacloud.com/help/en/oss/developer-reference/cp-upload-file)

The bundled `charts/index.yaml` lists only the two packages in the ZIP. Merge
them into the current repository index to preserve published versions. Upload
both packages before updating the public index so every new entry points to a
downloadable package. [Helm index documentation](https://helm.sh/docs/helm/helm_repo_index/)

```bash
(
  set -e
  repo_url=https://charts.selectdb.com
  bucket_url=oss://selectdb-charts

  curl -fsSL "$repo_url/index.yaml" -o current-index.yaml
  helm repo index ./charts --url "$repo_url" --merge ./current-index.yaml

  for chart in ./charts/*.tgz; do
    ossutil cp "$chart" "$bucket_url/$(basename "$chart")"
  done
  ossutil cp ./charts/index.yaml "$bucket_url/index.yaml"
)
```

To verify the release, fetch each chart through the public repository. Use the
version from each `.tgz` filename.

```bash
helm show chart doris --repo https://charts.selectdb.com --version '<doris-chart-version>'
helm show chart doris-operator --repo https://charts.selectdb.com --version '<doris-operator-chart-version>'
```
