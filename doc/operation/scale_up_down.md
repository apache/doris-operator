# Doris Scaling
The scaling of Doris on K8S should update the replicas field of DorisCluster resource. 
## Check DorisCluster Resource
The Doris have installed into default namespace.
Using the command `kubectl -n {namespace} get doriscluter` for getting the name of the deployed DorisCluster resource.
```shell
kubectl -n default get doriscluster
NAME                  FESTATUS    BESTATUS    CNSTATUS   BROKERSTATUS
doriscluster-sample   available   available
```
## scaling
use kubectl to modify `spec.feSpec.replicas` to scaling fe component of doris.
### FE scaling
**check fe replicas**
```shell
kubectl -n default get pods -l "app.kubernetes.io/component=fe"
NAME                       READY   STATUS    RESTARTS       AGE
doriscluster-sample-fe-0   1/1     Running   0              10d
```
**modify fe replicas**
```shell
kubectl -n default patch doriscluster doriscluster-sample --type merge --patch '{"spec":{"feSpec":{"replicas":3}}}'
```
**check result**
```shell
NAME                       READY   STATUS    RESTARTS   AGE
doriscluster-sample-fe-2   1/1     Running   0          9m37s
doriscluster-sample-fe-1   1/1     Running   0          9m37s
doriscluster-sample-fe-0   1/1     Running   0          8m49s
```
### BE scaling
**check be replicas**
```shell
kubectl -n default get pods -l "app.kubernetes.io/component=be"
NAME                       READY   STATUS    RESTARTS      AGE
doriscluster-sample-be-0   1/1     Running   0             3d2h
```
**modify be replicas**
```shell
 kubectl -n default patch doriscluster doriscluster-sample --type merge --patch '{"spec":{"beSpec":{"replicas":3}}}'
```
**check result**
```shell
 kubectl -n default get pods -l "app.kubernetes.io/component=be"
NAME                       READY   STATUS    RESTARTS      AGE
doriscluster-sample-be-0   1/1     Running   0             3d2h
doriscluster-sample-be-2   1/1     Running   0             12m
doriscluster-sample-be-1   1/1     Running   0             12m
```

## Disaggregated compute group decommission

With `spec.enableDecommission: true`, lowering a compute group's `replicas`
starts a persistent scale-down task. The operator uses MS `get_cluster` node
states to decide when decommission has finished. `SystemDecommissioned` and
`TabletNum` alone are not completion signals across SelectDB Cloud versions.

The operator adopts manual decommission SQL for nodes in the requested scale-down
range. On legacy kernels, FE may already report `SystemDecommissioned=true`
while MS still reports `RUNNING`; the operator waits instead of submitting again.
Manual decommission without a CR replica change does not remove Kubernetes pods.

After MS reports `DECOMMISSIONED`, the operator waits two FE synchronization
periods (at least 20 seconds), using both configured and effective FE settings.
Every registered FE, including observers, must be reachable. The operator checks
FE decommission state before DROP and waits until both MS and all FE views have
removed each target across a further synchronization wait before lowering StatefulSet replicas. MS API errors, unknown
states, changed node identities and DROP failures block scale-down.

Progress is stored in the StatefulSet annotation
`doris.disaggregated.cluster/decommission-action`. No CRD update is needed.
Inspect it and cluster warning events if a task stalls. Keep the replica target
and `enableDecommission` unchanged until the task finishes. A SQL submission
whose outcome is uncertain is not automatically retried; verify the target's
MS/FE state and complete the decommission manually if necessary. Do not clear
an active task annotation to force a retry without checking its recorded targets.
The operator requires network access to the MS administrative HTTP port and each
FE query port, plus permission to inspect FE configuration.
