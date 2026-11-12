# Backend-first image updates

For an existing FE StatefulSet, the operator defers an FE image change until
backends have reached their current desired images and are Ready:

- `DorisCluster`: the BE StatefulSet and all expected BE Pods.
- `DorisDisaggregatedCluster`: every compute group and all its expected BE Pods.

Users can update FE and BE images in one CR patch, or submit the BE patch first
and the FE patch later. The check uses the complete current CR and actual
workloads, so it also works after an operator restart. There is no additional
upgrade phase to persist and no new CRD field to install.

Before updating FE, the backend StatefulSet must contain the desired main
container image and replica count, and its controller must have observed its
current generation. Each expected Pod must be Running, Ready, not terminating,
and use both the desired main container image and the StatefulSet's update
revision. The comparison uses the StatefulSet generation, not the CR generation:
a later FE-only patch does not require another BE rollout.

Graceful compute-group rollouts must also finish their existing graceful action.
OnDelete rollouts are checked using individual Pod revisions rather than
`currentRevision == updateRevision`, which can remain false after an OnDelete
rollout is complete. Groups intentionally scaled to zero do not require Pods.

While FE waits, backend reconciliation and FE Service repair continue. The
operator emits a Normal `WaitBackendUpgrade` event naming the blocking backend
or Pod and retries after five seconds, even if the old workloads still appear
healthy in CR status:

```sh
kubectl --namespace <namespace> get events \
  --field-selector reason=WaitBackendUpgrade
```

An absent FE StatefulSet uses the existing creation/recovery path and retains
the FE-before-BE startup dependency. When the FE StatefulSet already has the
desired image, ordinary configuration changes, scaling, restart handling, and
repair retain their existing behavior. If an FE
image change and other FE changes are submitted together, the FE StatefulSet
changes wait together; Services can still be repaired.

Use a distinct image tag or digest for each version. Replacing the contents of
an unchanged tag is not an image-reference change and cannot trigger this gate.
The gate also applies to image-reference changes used for rollback. It does not
cancel an FE rollout that has already been submitted before a later BE change.

This ordering check establishes Kubernetes rollout readiness. It does not
validate Doris version compatibility or provide uninterrupted service for
single-replica components; application health checks remain the responsibility
of the upgrade caller and the existing Doris probes/graceful rollout logic.
