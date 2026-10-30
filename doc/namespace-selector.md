# Sharing controllers across namespaces

A controller can reconcile a dynamically selected group of namespaces:

```yaml
# Controller Deployment
args:
  - --leader-elect
  - --namespace-selector=selectdb.com/operator-group=qa
env:
  - name: ENABLE_WEBHOOK
    value: "false"
```

Label business namespaces before creating Doris CRs:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: qa-run-123
  labels:
    selectdb.com/operator-group: qa
```

Run another group with a disjoint selector and a separate controller namespace.
Groups must not share the default leader-election Lease. An unfiltered global
controller would still overlap all groups and must not run alongside them.

`--namespace` and `--namespace-selector` are mutually exclusive. Selector mode
requires webhooks disabled and namespace get/list/watch permission, supplied by
the Helm Chart. The Chart exposes `dorisOperator.namespaceSelector` and disables
webhooks for this mode. Empty configuration preserves the existing global or
single-namespace behavior.

The controller reads namespace labels before each Doris reconciliation. Read
errors stop reconciliation; removing the group label stops future reconciliations.
Namespace label events enqueue existing CRs, so a newly labeled namespace is
adopted without restarting the controller. Label selectors are Kubernetes selectors;
the platform deploying multiple controllers is responsible for making them disjoint.
Do not transfer namespaces between groups while CRs/finalizers remain.

The global read cache remains available for explicit dependencies such as another
namespace's FDB connection ConfigMap. This filters reconciliation, not authorization;
use RBAC and separate clusters when read isolation or incompatible CRDs are required.
