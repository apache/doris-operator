# FE heap defaults in containers

For newly created DorisCluster and DorisDisaggregatedCluster resources, the
operator adapts image-provided FE JVM defaults when `limits.memory` is set:

| Configuration entry | Heap defaults |
| --- | --- |
| `JAVA_OPTS` (JDK 8) | `-Xmx` equal to half the limit, rounded down to MiB; no explicit `-Xms` |
| `JAVA_OPTS_FOR_JDK_17` | `-XX:InitialRAMPercentage=25 -XX:MaxRAMPercentage=50` |

The fixed image `-Xms` and `-Xmx` values are removed before adding these defaults.
Other image JVM options, including GC, logging and module access, are retained.
The JDK 17 calculation requires the image JVM to recognize the container memory
limit correctly. Verify the effective heap on your supported image/cgroup
combination. Reserving half the limit for non-heap memory is a default budget,
not a guarantee against OOMKilled.

No adaptation is performed without a memory limit. `requests.memory` is not a
heap budget. This feature does not change BE JVM settings or Helm templates.

## User configuration

Each explicit JVM entry in a user `fe.conf` takes precedence as a whole. For
example, setting `JAVA_OPTS_FOR_JDK_17` preserves that exact value, whether it
uses fixed or percentage heap sizes. The operator does not merge defaults into
that entry or override it even if it equals an old image default. An omitted
entry is populated from the corresponding image default, if the image has it.
ConfigMaps and image files are never edited. Other files in the configuration
mount are copied along with `fe.conf`.

The standard `/etc/doris` configuration directory (or a literal
`CONFIGMAP_MOUNT_PATH`) is prepared using a same-image init container and an
`emptyDir`. Direct mounts over the FE home/conf directory and custom
`DORIS_HOME`/`DORIS_ROOT` environments remain user-managed. Explicit JVM
environment entries are also left alone; whether the image startup script
honors them is unchanged. Prefer `fe.conf` for overrides.

The preparation runs before optional runtime configuration persistence. It does
not require that feature or a PVC, and preserves its configuration changes.

## Existing clusters and operator upgrades

Before creating workloads or initializing status, the operator saves a decision
in the CR annotation `doris.apache.org/fe-heap-policy`:

- `auto`: adapt default heaps when a memory limit is configured.
- `legacy`: retain the previous behavior and Pod template for this feature.

A CR with existing status or an owned StatefulSet defaults to `legacy`, including
StatefulSets with older non-controller owner references. New CRs default to
`auto`. The annotation survives operator restarts, scaling and StatefulSet
recreation. Do not remove it to change policies.

To opt an existing cluster in during a planned FE update:

```sh
kubectl annotate dcr <name> -n <namespace> doris.apache.org/fe-heap-policy=auto --overwrite
# For disaggregated clusters, use ddc instead of dcr.
```

This changes the desired FE Pod template and uses the existing update workflow.
Explicit user JVM entries still take precedence. Set the annotation to `legacy`
on a new CR to opt out. Changing an already active cluster back to `legacy` also
changes its template and restores image defaults for entries not supplied by the
user; plan this as an FE configuration update.

For verification, inspect `fe.conf` inside the running FE and the JVM effective
flags (`jcmd <pid> VM.flags` where available). For an 8Gi limit, JDK 8 should use
`-Xmx4096m`; JDK 17 should have an initial heap around 2Gi and maximum around 4Gi,
subject to JVM alignment/ergonomics. Include a real startup check on both JDKs,
custom fixed/percentage settings, and an upgrade check that existing FE Pod
UIDs and templates remain unchanged until explicitly enabled.
