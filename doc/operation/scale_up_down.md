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
