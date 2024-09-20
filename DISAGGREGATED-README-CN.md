中文 | [English](DISAGGREGATED-README.md)
# 存算分离模式部署
存算分离是 Doris 从 3.0.0 开始提供的一种架构模式。存储和计算分离能够显著降低存储成本，在基本不降低性能的情况下将数据存储到价格更低廉的对象存储中，降低成本的同时也能更好地应对计算资源需求剧烈变化的场景。

## 环境要求
- Kubernetes 1.19+
- 宿主机的能够使用的 open files 大于等于 65535 (ulimit -n)
- doris >= 3.0.2

>[!NOTE]
>1. K8s worker 所有节点总资源量大于部署 doris 需要的资源总量。
>2. worker 单个节点的资源量需要大于一个 fe 或 be 所需要的最大资源量。fe 或 be 默认最低启动配置 4c 4Gi 。

## 部署 FoundationDB
在 K8s 上部署存算分离集群需要提前部署好 fdb (foundationdb 简称)。在 k8s 上可参考 fdb 官方提供的 operator 中的[快速部署文档](https://github.com/FoundationDB/fdb-kubernetes-operator)进行部署。  

## 安装 Doris Operator
1. 下发资源定义：
```
kubectl create -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/crd/bases/crds.yaml
```
预期结果：
```
customresourcedefinition.apiextensions.k8s.io/dorisclusters.doris.selectdb.com created
customresourcedefinition.apiextensions.k8s.io/dorisdisaggregatedclusters.disaggregated.cluster.doris.com created
```
2. 部署 Operator 以及依赖的 RBAC 规则：
```
kubectl apply -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/operator/disaggregated-operator.yaml
```
预期结果：
```
kubectl -n doris get pods
NAME                                         READY   STATUS    RESTARTS   AGE
doris-operator-5b667b4954-d674k              1/1     Running   0          11s
```
3. 部署 Doris 存算分离集群
```
kubectl apply -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/disaggregated/cluster/ddc-sample.yaml
```

>[!NOTE]
>1. fdb 的 k8s 部署需要 k8s 至少有三台宿主机作为 worker 节点，如果 k8s 的 worker 节点数少于 3 ，请使用 doris-operator 提供的[单例模式部署](./doc/examples/disaggregated/fdb/cluster-single.yaml)。
>2. 详细部署请参考 doris-operator 官方[部署存算分离文档](https://doris.apache.org/zh-CN/docs/install/cluster-deployment/k8s-deploy/install-quickstart)。
