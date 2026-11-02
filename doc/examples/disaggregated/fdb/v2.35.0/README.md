# FoundationDB Operator v2.35.0: fresh installation

These examples separate the controller from the FoundationDB database. The
controller watches only `fdb-system`, where both examples create their database.
Use only one profile. No running-cluster validation is implied by these samples.

## Prerequisites and version contract

- Kubernetes access to create CRDs, RBAC and workloads; a writable SSD-backed
  StorageClass; working cluster DNS and Pod connectivity between FDB and MetaService.
- No existing controller watching `fdb-system`. CRDs are cluster-wide: check all
  existing FDB installations before installing these definitions. Do not overwrite
  shared CRDs or delete CRDs as an upgrade mechanism.
- Operator and CRDs: v2.35.0, sourced from upstream commit
  `fd8fdbd2c6d14375116e97977f1f49e3d17b84a3`. See
  [provenance and CRD checksums](../../../../../config/fdb/v2.35.0/UPSTREAM.md).
- Controller: `selectdb/fdb-kubernetes-operator:v2.35.0`.
- Controller client initializer and database: `selectdb/fdb-kubernetes-monitor:7.3.69`.
  Both images support linux/amd64 and linux/arm64. Replace `selectdb/` with `velodb/`
  in the downloaded YAML to use the corresponding mirror with the same tags.
- MetaService must include a FoundationDB client compatible with FDB 7.3. A
  controller client initializer does not change the client library in MetaService.
  Do not use this guide to upgrade an existing FDB database or its consumers.

## Resource planning

| Setting | Single: development only | Double: capacity-planning starting point |
| --- | --- | --- |
| storage / log / stateless Pods | 1 / 1 / 2 | 3 / 4 / 9 |
| Main container requests per Pod | 1 CPU / 1Gi | 1 CPU / 8Gi |
| Main container limits per Pod | 1 CPU / 2Gi | 2 CPU / 8Gi |
| Sidecar requests = limits per Pod | 100m CPU / 256Mi | 100m CPU / 256Mi |
| PVC per storage / log Pod | 10Gi / 10Gi | 200Gi / 100Gi |
| Total requests including sidecars | 4.4 CPU / 5Gi | 17.6 CPU / 132Gi |
| PVC count / total requested capacity | 2 / 20Gi | 7 / 1000Gi |

These are explicit sample presets, not minimum requirements or production sizing
certification. Low-memory single mode is for small functional tests; validate
memory/cache settings and OOM behavior for the actual workload. Upstream defaults
to 1 CPU / 8Gi per main container. Double mode needs at least three independent
host failure domains, with coordinator and storage/log placement checked in the
running cluster. Capacity, disk latency and recovery headroom must be planned for
the aggregate metadata workload of all Doris clusters sharing FDB.

Totals exclude the Operator (500m CPU / 256Mi requests and limits), its initializer
(10m CPU / 50Mi), MS/FE/BE, system overhead and temporary replacement Pods/PVCs.
Preferred anti-affinity does not guarantee physical separation. Changing it to
required anti-affinity for every role needs at least nine schedulable nodes for
the double example's nine stateless Pods. Keep one FDB server per Pod unless you
also recalculate memory and CPU for multiple servers.

## Install from a checkout of this repository

Set the path to your kubeconfig explicitly; commands below run from the repository
root and do not change the current context.

```sh
export KUBECONFIG=/path/to/kubeconfig
kubectl --kubeconfig "$KUBECONFIG" get crd foundationdbclusters.apps.foundationdb.org
kubectl --kubeconfig "$KUBECONFIG" get deployments -A
kubectl --kubeconfig "$KUBECONFIG" get storageclass
```

If an FDB installation already exists, stop here and evaluate compatibility with
its maintainer. Otherwise:

```sh
kubectl --kubeconfig "$KUBECONFIG" create namespace fdb-system
kubectl --kubeconfig "$KUBECONFIG" apply --server-side -f config/fdb/v2.35.0/crds/
kubectl --kubeconfig "$KUBECONFIG" wait --for=condition=Established --timeout=60s \
  crd/foundationdbclusters.apps.foundationdb.org \
  crd/foundationdbbackups.apps.foundationdb.org \
  crd/foundationdbrestores.apps.foundationdb.org
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system apply -f config/fdb/v2.35.0/operator.yaml
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system rollout status \
  deployment/fdb-kubernetes-operator-controller-manager --timeout=300s
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system apply -f config/fdb/v2.35.0/monitor-rbac.yaml
cp doc/examples/disaggregated/fdb/v2.35.0/cluster-single.yaml /tmp/fdb-cluster.yaml
```

For double mode, copy `cluster-double.yaml` instead. Replace both occurrences of
`REPLACE_WITH_SSD_STORAGE_CLASS` in `/tmp/fdb-cluster.yaml` with your StorageClass,
review resources, then apply:

```sh
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system apply -f /tmp/fdb-cluster.yaml
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system get fdb
```

Expected final state: `AVAILABLE=true`, `FULLREPLICATION=true`, and `RECONCILED`
catches up with `GENERATION`. Pod Running alone is not a successful installation.
Inspect `status.configured`, `status.health`, `status.generations`, PVCs, Events and
controller logs when waiting; no fixed completion time is guaranteed.

```sh
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system get fdb fdb-single -o yaml
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system get configmap fdb-single-config -o yaml
```

Use `fdb-double` / `fdb-double-config` for double mode. Configure Doris MetaService
with this ConfigMap namespace/name and confirm its FDB connection succeeds.
Deleting a Doris cluster must not delete an FDB shared with other clusters.
Deletion of the FDB resource/PVCs can destroy metadata; retain backups separately.

## Existing Doris builds requiring FDB 7.1

Do not assume that a new FDB server series is compatible with an older MetaService.
For a fresh **amd64** 7.1 installation, use the mirrored Monitor `7.1.67` and add
its controller client before creating the database:

```sh
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system patch \
  deployment fdb-kubernetes-operator-controller-manager --type=strategic \
  --patch-file config/fdb/v2.35.0/operator-client-7.1.patch.yaml
kubectl --kubeconfig "$KUBECONFIG" -n fdb-system rollout status \
  deployment/fdb-kubernetes-operator-controller-manager --timeout=300s
```

In the copied cluster YAML, change `spec.version`, the image config `version` and
`tag` from `7.3.69` to `7.1.67` together. Keep the mirror and unified image mode.
The 7.1.67 mirror is amd64-only: all database Pods and the patched controller must
run on amd64 nodes (set `nodeSelector: {kubernetes.io/arch: amd64}` on every relevant
Pod template in a mixed cluster). Verify the MetaService client supports this
server version. This is not a database downgrade procedure.

## Private registries and namespace changes

Mirror the controller and every enabled client/database image with the required
platforms. Change the Operator `image`, initializer `image`, and database
`mainContainer.imageConfigs[].baseImage` together. For unified mode, the Operator
uses the main image for both database containers; no separate split sidecar image
is required. Use `imagePullSecrets` in both controller and database Pod templates
when needed. Keep `spec.version` consistent with the binary inside the image.

To change `fdb-system`, update every namespaced object's metadata and all RBAC
subject namespaces in the installation YAML and the selected database YAML.
The controller's `WATCH_NAMESPACE` follows its own namespace. A new database in
another namespace will not be reconciled by this controller.
