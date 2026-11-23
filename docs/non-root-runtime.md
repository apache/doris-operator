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
| Root image to root image | Change image references | Preserve the existing default initializer, explicit WG policy and runtime settings; no ownership migration. |
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
the Pod's process namespace. Only the auxiliary containers use privilege; the runtime helper mounts the host
cgroup filesystem. Doris remains under its resolved user with no capabilities
when non-root.

The main startup wrapper sends its PID and process start time. The helper
validates the UID, Pod UID, container cgroup scope and process generation. It
keeps the BE leader in the container's memory domain. A propagated bind mount
makes `/doris-helper/cgroup/query` refer to that same domain: Doris writes its
process to `query/cgroup.procs`, so this is deliberately an alias, not a nested
threaded cgroup. Only WG worker threads move into the numeric threaded children
that Doris creates below it. A small home directory exposes the required cgroup
control files as bind mounts; it does not fabricate controller or memory values.

The helper delegates directory creation and the process/thread membership and
subtree-control files. It does not change ownership or values of the container's
`cpu.max`, `memory.max`, or `cgroup.type`. These limit files remain root-owned and
are not writable by the non-root BE. The host cgroup mount is helper-only; Doris
receives only its own container's delegation view.

The wrapper waits for a root-owned readiness acknowledgement before executing
the ordinary Doris entrypoint. The helper handles main-container restarts and
remounts the new container's domain. Helper readiness is included in Pod
readiness. For images that skip CPU quota lookup at a cgroup namespace root, the
wrapper supplies `num_cores` from the container's CPU quota and process affinity
when the effective configuration has no positive explicit `num_cores`. An
explicit positive value remains the administrator's choice. Resource changes
that affect this startup value require a Pod restart.

The helper configures `doris_cgroup_cpu_path=/doris-helper/cgroup`; do not override
this path in persistent/custom BE configuration when using this mode. For non-root
images, use the new switch for OS-level CPU control. Root BE and compute containers
retain the existing `enableWorkloadGroup` behavior when the helper is disabled.
Disabling the helper removes the helper and
its host mounts through a rolling Pod replacement. Other WG functions that do
not require OS CPU delegation remain Doris functionality.

This implementation requires Kubernetes restartable init sidecars (1.29+), Linux
cgroup v2, the systemd cgroup driver and containerd's systemd scope layout. An
unsupported hierarchy fails initialization instead of creating an unconstrained
cgroup elsewhere. The helper is trusted privileged infrastructure and must only
run images trusted by the cluster administrator.

With the switch enabled, the existing `default-init` also runs as root to prepare
`vm.max_map_count` and swap settings, preserving the former system initialization
behavior. Active swap files must be accessible from that initializer; host-only
swap paths must be disabled on the node by the administrator. Privilege does not
make a host filesystem path appear in a container. The initializer skips
`swapoff -a` when no swap is active. The existing `skipDefaultSystemInit` still
skips that step when explicitly requested. For non-root images with the switch disabled no privileged system initializer is injected;
prepare required node settings externally. User-supplied privileged initializers
are rejected for non-root images while the switch is false. With a root image and the default switch value, existing initializer and workload-group settings continue to apply.

## CPU and memory accounting

A threaded CPU child has no memory controller of its own. Some Doris images
check only the leaf `cgroup.controllers` before looking for ancestor memory
files; moving the whole BE process there makes them fall back to host RAM. The
kernel limit remains enforced, but the inflated Doris budget can cause OOMs.

The helper avoids that layout. BE's leader stays in its original container
memory domain and sees the real `memory.max`; WG worker threads share that same
memory domain while their CPU usage is controlled in threaded children. This
requires no BE memory-reader patch and no numeric `mem_limit` override. Test the
actual image with helper off and on, including memory detection, WG CPU quotas,
Pod resource limits, and both helper and main-container restarts. Turning the
switch off does not promise OS-level WG resource control; turning it on must
preserve the resource behavior expected from a root deployment.
