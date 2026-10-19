# Runtime configuration persistence

FE and BE store persisted mutable settings in `fe_custom.conf` and
`be_custom.conf`. Cloud BE also saves configuration received in FE heartbeats.
MS normally persists mutable settings into its startup configuration file, which
its container entrypoint can replace from a ConfigMap on restart.

The operator can preserve these settings separately from user ConfigMaps for
DorisCluster FE/BE and DorisDisaggregatedCluster FE/BE/MS. It does not change user
ConfigMaps, mount an empty volume over the image's `conf` directory, create new
PVCs, or change StatefulSet PVC templates.

## Defaults and compatibility

On its first reconciliation, the operator records a cluster-level default in
`doris.apache.org/runtime-config-policy`:

- New clusters use automatic persistence where a safe existing PVC is available.
- Existing clusters retain their previous Pod templates. Upgrading the operator
  does not migrate or restart them.
- The recorded choice survives operator restarts and individual StatefulSet
  deletion/recreation. Do not manually remove the policy annotation.

A new-cluster component without a suitable PVC keeps its existing behavior and
emits `RuntimeConfigStorageUnavailable`. Its StatefulSet is marked unavailable
for automatic persistence; adding storage later does not silently migrate it.
Stateless clusters are **not** made durable by this feature.

An explicit request is strict:

```yaml
spec:
  runtimeConfigPersistence:
    enabled: true
```

Missing safe storage or an incomplete existing-cluster migration blocks this
request. Set `enabled: false` before initial deployment to opt out. Turning off
an already activated policy is rejected: doing so could discard configuration
that overrides the ConfigMap. No automatic reverse migration is provided.

## Storage and configuration layout

The operator selects a writable, existing per-Pod PVC mount in this order:

1. A log PVC.
2. An FE metadata PVC (`meta_dir` / `doris-meta`).
3. A BE data PVC (`storage_root_path` / `storage`).

A `file_cache_path` root or overlapping directory is excluded because cache
maintenance can delete the entire root. EmptyDir, read-only mounts and mounts
using an existing `subPathExpr` are excluded. A cache-only cloud BE needs a safe
persistent log/configuration volume before explicit enablement. Directory
selection is recorded on the StatefulSet and is not silently switched later.

A directory `.doris-runtime-config/<pod-name>` inside the selected mount is
exposed at `/var/lib/doris/runtime-config`. The Pod name separates overrides even
when an existing PVC is shared. Existing `subPath` mounts retain their prefix.

For DDC compute-group scale-down, the operator retains the selected runtime
configuration PVC so scaling back to the same ordinal can recover its overrides
and migration marker. Other PVCs, including file cache PVCs, follow the existing
cleanup policy. Retained configuration claims continue to consume storage while
the ordinal is absent. This does not change deletion of the cluster or manual
PVC deletion, and does not initialize cloud identity for entirely new nodes.

An init container uses the **same image and security context** as the main
container to create the directory and prepare a writable copy of the base
ConfigMap configuration. When no base ConfigMap exists, it uses the image's
base configuration. It injects:

| Component | Injected setting | Persisted file |
| --- | --- | --- |
| FE | `custom_config_dir=/var/lib/doris/runtime-config` | `fe_custom.conf` |
| BE | `custom_config_dir=/var/lib/doris/runtime-config` | `be_custom.conf` |
| MS | `custom_conf_path=/var/lib/doris/runtime-config/ms_custom.conf` | `ms_custom.conf` |

The main container retains its normal entrypoint. Runtime files are created only
when absent; image/configuration updates never truncate them. Directory mounts
allow the kernel to create temporary files and replace files with `rename`.

Explicit custom paths already backed by writable PVCs remain user-managed. An
explicit nondefault path without durable backing is rejected. ConfigMap-backed
initial `*_custom.conf` files seed the persistent file once and are excluded
from the prepared ConfigMap directory to prevent read-only symlinks.

Images must contain Bash, cp, awk and the existing entrypoint, and support the
corresponding kernel configuration setting. Nonroot deployments must supply
working PVC/emptyDir permissions (for example the appropriate Pod `fsGroup`).
This feature does not change container users or elevate privileges.

## Migrating an existing cluster

1. Upgrade CRDs and the operator first. Leave the persistence spec unset.
2. Stop dynamic configuration updates, scaling, image updates, and concurrent
   automation for the migration window. All existing Pods must be Ready.
3. Back up the live overrides **before** replacing any Pod:

   ```sh
   python3 tools/runtime-config-migrate.py \
     --kind ddc --namespace example --name doris \
     --backup-dir ./runtime-backup
   # Use --kind dcr for DorisCluster; --context selects a kubectl context.
   ```

4. Review the private backup directory. Files are mode 0600 and the directory is
   mode 0700; configuration values are not printed or stored in annotations.
   In particular, MS candidates contain effective settings that differ from the
   currently mounted ConfigMap. The tool cannot reconstruct which historical
   values were deliberately changed: review MS candidates and remove obsolete
   overrides. FDB endpoints and configuration-path settings are excluded.
5. Apply the reviewed backup:

   ```sh
   python3 tools/runtime-config-migrate.py \
     --kind ddc --namespace example --name doris \
     --backup-dir ./runtime-backup --apply
   ```

The tool checks cluster, StatefulSet and Pod identities, replicas, generations,
and unchanged live source configuration. It copies each backup through the old
Pod's existing PVC mount, verifies the copy, and only then patches the migration
annotation and `enabled: true`. It never silently replaces a different existing
backup. The operator validates the approval before changing workloads. Migrated
ordinals require their on-disk migration marker; newly added ordinals initialize
normally. Follow the operator's normal rolling update and verify loaded settings
and a subsequent Pod replacement before reopening configuration updates.

If there is no suitable PVC, an explicit custom path already exists, an MS base
ConfigMap is unavailable, or membership changes, the tool stops. Such clusters
need an individually planned storage/configuration migration. The tool does not
recreate StatefulSets or recover overrides after an old Pod is already lost.
Keep local backups until validation is complete.

## Scope

Persistence closes the loss of mutable configuration during Pod replacement.
A new cloud BE still needs its first FE heartbeat. Kernel query admission and FE
scheduling must independently prevent queries before required initialization.
