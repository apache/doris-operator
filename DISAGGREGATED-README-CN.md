中文 | [English](DISAGGREGATED-README.md)
# 存算分离模式部署
存算分离是 Doris 从 3.0.0 开始提供的一种架构模式。存储和计算分离能够显著降低存储成本，在基本不降低性能的情况下将数据存储到价格更低廉的对象存储中，降低成本的同时也能更好地应对计算资源需求剧烈变化的场景。
## 资源简介
Doris 存算分离包括以下组件：fdb, ms, recycler, fe, be 。 Doris-Operator 使用 `DorisDiaggregatedMetaService` 资源部署 fdb, ms, recycler 。使用 `DorisDisaggregatedCluster` 资源部署 fe，计算集群（一组 be）。
## 环境要求
- Kubernetes 1.19+
- 宿主机的能够使用的 open files 大于等于 65535 (ulimit -n)

>[!NOTE]
>1. K8s worker 所有节点总资源量大于部署 doris 需要的资源总量。
>2. worker 单个节点的资源量需要大于一个 fe 或 be 所需要的最大资源量。fe 或 be 默认最低启动配置 4c 4Gi 。

## 安装 Operator
1. 下发资源定义：
```
kubectl create -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/crd/bases/crds.yaml
```
预期结果：
```
customresourcedefinition.apiextensions.k8s.io/foundationdbclusters.apps.foundationdb.org created
customresourcedefinition.apiextensions.k8s.io/foundationdbbackups.apps.foundationdb.org created
customresourcedefinition.apiextensions.k8s.io/foundationdbrestores.apps.foundationdb.org created
customresourcedefinition.apiextensions.k8s.io/dorisclusters.doris.selectdb.com created
customresourcedefinition.apiextensions.k8s.io/dorisdisaggregatedclusters.disaggregated.cluster.doris.com created
customresourcedefinition.apiextensions.k8s.io/dorisdisaggregatedmetaservices.disaggregated.metaservice.doris.com created
```
2. 部署 Operator 以及依赖的 RBAC 规则：
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/operator/disaggregated-operator.yaml
```
预期结果：
```
kubectl -n doris get pods
NAME                                         READY   STATUS    RESTARTS   AGE
doris-operator-fdb-manager-d75574c47-b2sqx   1/1     Running   0          11s
doris-operator-5b667b4954-d674k              1/1     Running   0          11s
```
## 快速部署存算分离集群
[部署案例](./doc/examples/disaggregated/cluster) 中提供了常见配置的部署样例。以下使用最简单模式快速搭建拥有 1 套计算集群的 Doris 存算分离数据仓库：
1. 下发 `DorisDisaggregatedMetaService` 资源: 
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/disaggregated/metaservice/ddm-sample.yaml
```
预期结果：
```
kubectl get ddm
NAME                   FDBSTATUS   MSSTATUS   RECYCLERSTATUS
meta-service-release   Available   Ready      Ready
```
2. 下发包含对象存储信息的 ConfigMap 资源：  
存算分离以对象存储作为后端存储，需要提前规划好使用的对象存储。按照 [Doris 存算分离接口](https://doris.apache.org/zh-CN/docs/dev/compute-storage-decoupled/creating-cluster#%E5%86%85%E7%BD%AE%E5%AD%98%E5%82%A8%E5%90%8E%E7%AB%AF)接口格式将对象存储信息配置成 json 格式，以 `instance.conf` 为 key ， json 格式的对象存储信息作为 value 配置到 ConfigMap 的 data 中。  
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/disaggregated/cluster/object-store-info.yaml
```
预期结果：
```
configmap/vault-test created
```
>[!NOTE]
>1. 部署存算分离集群需要预先规划好使用的对象存储，将对象存储信息通过 ConfigMap 配置到 doris 存算分离集群需要部署的 Namespace 下。
>2. 案例中的配置主要为展示对象存储的基本配置所需信息，所有的值均为虚构不能用于真实场景，如果需要搭建真实可用集群请使用真实数据填写。

3. 下发 `DorisDisaggregatedCluster` 资源部署集群：
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/disaggregated/cluster/ddc-sample.yaml
```
预期结果：
```
kubectl get ddc
NAME                         CLUSTERHEALTH   FEPHASE   CCCOUNT   CCAVAILABLECOUNT   CCFULLAVAILABLECOUNT
test-disaggregated-cluster   green           Ready     1         1                  1
```
