# Non-root images, privileged CPU helper, and existing volumes

`DorisCluster` and `DorisDisaggregatedCluster` expose one new switch:

```yaml
spec:
  enablePrivilegedHelper: false
```

The default is `false`. The image supplies the process identity; this switch
never changes the Doris process to root or makes its container privileged.
Explicit existing `runAsUser`/`runAsGroup` settings take precedence over the
image. Remove an explicit UID 0 override when switching to the image's non-root
identity. UID/GID changes between two non-root images require a storage migration
and are rejected rather than silently rewriting existing data.

## Installation and upgrade

| Transition | Required CR change | Operator behavior |
| --- | --- | --- |
| New non-root cluster | Create the normal CR | Resolve the image user; set fsGroup; check writable PVC mounts as that user. No privileged or root repair container. |
| Root image to root image | Change image references | Preserve root identity; no ownership migration. |
| Root image to non-root image | Change image references | Prepare dedicated existing PVC permissions before starting the target user. The helper switch can stay false. |
| Non-root to non-root, same UID/GID | Change image references | Reuse permissions and check access; no repeated full-tree repair on subsequent image upgrades. |

Installing this Operator version also reconciles the security settings of
existing StatefulSets and can trigger a rolling Pod replacement before an image
change. Plan the Operator upgrade accordingly.

Use immutable tags or digests. For images without a fully specified runtime
UID/GID, the Operator runs a short image-identity Pod, using the workload's pull
secrets, service account name, node selector and tolerations. It executes `id`,
mounts no data or API token, drops all capabilities, and stores the numeric user
in its termination message. Owned probe Pods are reused and removed by garbage
collection when the CR is deleted. The Operator service account needs `create`
on Pods in addition to its existing read permissions. The actual registry image
user is not inferred from the tag or version number. Multi-architecture images
must use the same UID/GID on each platform. In namespaces enforcing Restricted
Pod Security, explicitly configure the numeric UID/GID using the existing
container security context to avoid identity probes whose user is not yet known
to admission. Automatic root-owned volume repair still needs a policy exception
or storage administrator preparation.

Root-to-non-root transitions inject `migrate-volume-permissions`, followed by
`check-volume-permissions`. The repair container runs as UID 0 with
`privileged: false`, `allowPrivilegeEscalation: false`, all capabilities dropped
except `CHOWN`, `FOWNER`, and `DAC_OVERRIDE`. It mounts only the dedicated data
volumes that need repair. It changes ownership without following symlinks or
crossing filesystem boundaries, preserves data, and makes existing directories
and files accessible to the target owner. Failures stop initialization; normal
StatefulSet rollout then waits for this Pod before replacing the next one.

The migration state is attached to the StatefulSet template revision. Retries
of the migration image keep the idempotent initializer, including after an
Operator restart. A further image change is rejected until that rollout has
completed. A subsequent same-user image upgrade removes the repair initializer.
There is deliberately no success marker inside writable Doris data that a
Doris process could forge. Consequently a Pod restart while still using the
migration image rechecks the tree. Large PVCs can lengthen this initial upgrade.

Only per-Pod `volumeClaimTemplates` are automatically repaired. External/shared
PVCs are checked as the target user, including existing files on a root upgrade;
the Operator does not recursively change permissions on data another workload
may be writing. Such storage must already grant the target UID/GID access.
When external PVCs are present, automatic fsGroup is also omitted to avoid
changing their group ownership; all mounted data volumes must already be
accessible, or an explicitly configured fsGroup remains the administrator's
choice. Read-only PVC mounts are not repaired or write-probed. PVCs must be dedicated to
the declared Doris data paths; concurrent out-of-band writers are unsupported
during migration.

`fsGroup` support depends on the volume driver. Neither root nor a privileged
container can bypass NFS `root_squash`, server-side ACLs, read-only exports, or
admission policies. For a restricted export, the storage administrator must
prepare the actual exported Doris directories on the server for the target
numeric UID/GID (or equivalent ACLs), including existing files and parent
traversal permissions. Keep `root_squash`; opening the helper switch does not
solve this. Policies prohibiting every UID-0 container also prohibit automatic
legacy ownership repair. Prepare storage externally in those environments.

## Privileged helper

Set `enablePrivilegedHelper: true` to enable CPU cgroups for BE/CN and DDC compute
containers. It creates a restartable init sidecar (`doris-cgroup-helper`), sharing
the Pod's process namespace. Only this helper is privileged and mounts the host
cgroup filesystem. Doris remains under its resolved user with no capabilities
when non-root.

The main startup wrapper sends its PID and process start time. The helper
validates the UID, Pod UID, container cgroup scope and process generation, then
creates a threaded CPU subtree **under that BE container's cgroup**. It delegates
only that subtree to Doris through a propagated bind mount. It never replaces
or raises the parent container's CPU or memory limits. The wrapper waits for a
root-owned readiness acknowledgement before executing the ordinary Doris
entrypoint. The helper watches for main-container restarts and remounts the new
container's subtree. Helper readiness is included in Pod readiness.

The helper configures `doris_cgroup_cpu_path=/doris-helper/cgroup`; do not override
this path in persistent/custom BE configuration when using this mode. Existing
`enableWorkloadGroup` no longer enables privilege on the main container: use the
new switch for OS-level CPU control. Disabling the helper removes the helper and
its host mounts through a rolling Pod replacement. Other WG functions that do
not require OS CPU delegation remain Doris functionality.

This implementation requires Kubernetes restartable init sidecars (1.29+), Linux
cgroup v2, the systemd cgroup driver and containerd's systemd scope layout. An
unsupported hierarchy fails initialization instead of creating an unconstrained
cgroup elsewhere. The helper is trusted privileged infrastructure and must only
run images trusted by the cluster administrator.

The old privileged `default-init` host-wide sysctl/swapoff initializer is no
longer injected. Prepare required node settings before deployment. User-supplied
privileged initializers are rejected when the helper switch is false; privileged
Doris main containers are rejected regardless of the switch.

## Memory accounting requirement

Doris releases that require `memory` in the **leaf** `cgroup.controllers` can
misidentify the host's RAM as available memory after entering a threaded CPU
subgroup. The kernel still enforces the original memory limit; the wrong Doris
budget can nevertheless cause OOM kills. Such images need the Doris memory
controller detection fix before enabling the helper. A numeric `mem_limit`
override is not the fix: effective container memory accounting must be tested
with helper off and on. See the associated validation evidence for tested image
versions and any outstanding build/runtime checks.
