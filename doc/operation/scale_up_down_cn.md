# Doris扩缩容
Doris在K8S之上的扩缩容可通过修改 DorisCluster 资源对应组件的 replicas 字段来实现。修改可直接编辑对应的资源，也可通过命令的方式。

## 获取 DorisCluster 资源
使用命令 `kubectl -n {namespace} get doriscluster` 获取已部署 DorisCluster 资源的名称.
```shell
kubectl -n default get doriscluster
NAME                  FESTATUS    BESTATUS    CNSTATUS   BROKERSTATUS
doriscluster-sample   available   available
```
## 扩缩容资源
K8S所有运维操作通过修改资源为最终状态，由 Operator 服务自动完成运维。扩缩容操作可通过 `kubectl -n {namespace} edit doriscluster {name}` 直接进入编辑模式修改对应 spec 的 replicas 值，保存退出后 Doris-Operator 完成运维，
也可以通过如下命令实现不同组件的扩缩容。
### FE扩容
**查看当前FE服务数量**  
```shell
kubectl -n default get pods -l "app.kubernetes.io/component=fe"
NAME                       READY   STATUS    RESTARTS       AGE
doriscluster-sample-fe-0   1/1     Running   0              10d
```
**扩容FE**  
```shell
kubectl -n default patch doriscluster doriscluster-sample --type merge --patch '{"spec":{"feSpec":{"replicas":3}}}'
```
**检测扩容结果**  
```shell
kubectl -n default get pods -l "app.kubernetes.io/component=fe"
NAME                       READY   STATUS    RESTARTS   AGE
doriscluster-sample-fe-2   1/1     Running   0          9m37s
doriscluster-sample-fe-1   1/1     Running   0          9m37s
doriscluster-sample-fe-0   1/1     Running   0          8m49s
```
### BE扩容
**查看当前BE服务数量**
```shell
kubectl -n default get pods -l "app.kubernetes.io/component=be"
NAME                       READY   STATUS    RESTARTS      AGE
doriscluster-sample-be-0   1/1     Running   0             3d2h
```
**扩容BE**
```shell
 kubectl -n default patch doriscluster doriscluster-sample --type merge --patch '{"spec":{"beSpec":{"replicas":3}}}'
```
**查看扩容结果**
```shell
 kubectl -n default get pods -l "app.kubernetes.io/component=be"
NAME                       READY   STATUS    RESTARTS      AGE
doriscluster-sample-be-0   1/1     Running   0             3d2h
doriscluster-sample-be-2   1/1     Running   0             12m
doriscluster-sample-be-1   1/1     Running   0             12m
```
