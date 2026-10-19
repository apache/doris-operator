# Doris扩缩容
Doris在 K8S 之上的扩缩容可通过修改 DorisCluster 资源对应组件的 replicas 字段来实现。修改可直接编辑对应的资源，也可通过重新下发资源。

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

## 存算分离计算组退役缩容

设置 `spec.enableDecommission: true` 后，下调计算组的 `replicas` 会启动持久化缩容任务。
Operator 通过 MS `get_cluster` 返回的节点状态判断退役完成，不再仅凭
`SystemDecommissioned` 或 `TabletNum` 推断，避免不同 SelectDB Cloud 版本的字段语义差异。

对于本次缩容范围内的节点，Operator 可以接管用户手动执行的退役 SQL。
旧版内核可能已在 FE 标记 `SystemDecommissioned=true`，而 MS 仍为 `RUNNING`；
此时继续等待，不重复提交。只执行 SQL、不修改 CR 副本数，不会触发 Kubernetes Pod 缩容。

MS 返回 `DECOMMISSIONED` 后，等待两个 FE 同步周期（至少 20 秒），兼顾配置文件和 FE 当前配置。
所有已注册 FE（包含 Observer）需要可访问。DROP 前检查 FE 的退役状态，
DROP 后确认 MS 和所有 FE 均已移除目标，并在同步等待期后再次确认，再下调 StatefulSet 副本数。
MS 查询失败、未知状态、节点身份变化或 DROP 失败都会阻断缩容。

任务记录保存在 StatefulSet 注解 `doris.disaggregated.cluster/decommission-action`，无需更新 CRD。
任务停滞时检查该注解和集群 Warning 事件；完成前保持副本目标和 `enableDecommission` 不变。
SQL 提交结果不确定时不会自动重发，应核对目标的 MS/FE 状态，必要时手动完成退役；
未经核对记录中的目标节点，不要清除活动任务注解来强制重试。
Operator 需要访问 MS 管理 HTTP 端口和各 FE 查询端口，并具有查看 FE 配置的权限。
