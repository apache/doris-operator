# Controlling DorisCluster reconciliation during an operator upgrade

The operator supports pausing reconciliation for an individual `DorisCluster`.
Deletion cleanup is not paused, and removing the annotation triggers reconciliation again.

```shell
kubectl -n <namespace> annotate doriscluster <name> \
  apache.org.doris/reconcile-paused=true --overwrite

kubectl -n <namespace> annotate doriscluster <name> \
  apache.org.doris/reconcile-paused-
```

The legacy `selectdb.com.doris/reconcile-paused` annotation is also accepted.
This allows existing clusters to be resumed one at a time after an operator
upgrade while newly created, unannotated clusters continue to reconcile.

The leader election lease name can be configured with
`--leader-election-id`. Its default remains `e1370669.selectdb.com`. When two
operators run in the same operator namespace, give them different IDs only if
their `--namespace` watch scopes do not overlap:

```shell
/dorisoperator --leader-elect \
  --leader-election-id=doris-operator-team-a \
  --namespace=team-a
```

Overlapping watch scopes with different leader election IDs are unsafe because
both operators can reconcile the same resources.
