中文 | [English](README.md)
# doris-operator
Doris-Operator 用于在 Kubernetes 上创建、配置和管理 Doris 集群，能够部署和管理 fe、be、cn、broker 所有组件。  

[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg?color=f5deb3)](https://www.apache.org/licenses/LICENSE-2.0.html)
[![Operator Release](https://img.shields.io/github/v/release/apache/doris-operator?color=00FFFF)](https://github.com/apache/doris-operator/releases)
[![Tags](https://img.shields.io/github/v/tag/apache/doris-operator?label=latest%20tag&color=00FF7F)](https://github.com/apache/doris-operator/tags)
[![docker pull](https://img.shields.io/docker/pulls/apache/doris?color=1E90FF&logo=docker)](https://img.shields.io/docker/pulls/apache/doris)
[![issues](https://img.shields.io/github/issues-search?query=repo%3Aapache%2Fdoris-operator%20is%3Aopen&color=AFEEEE&label=issues)](https://github.com/apache/doris-operator/issues)
[![Go Version](https://img.shields.io/github/go-mod/go-version/apache/doris-operator?color=00FFFF)](https://img.shields.io/github/go-mod/go-version/apache/doris-operator)
[![docs](https://img.shields.io/website?url=https%3A%2F%2Fdoris.apache.org%2Fdocs%2Finstall%2Fdeploy-on-kubernetes%2Finstall-config-cluster&label=docs&color=7FFF00)](https://doris.apache.org/docs/install/deploy-on-kubernetes/install-config-cluster)

## 特点
- 通过自定义 DorisCluster 资源管控 Doris 集群。
- 提供定制化存储。
- 实现 Doris 在 Kubernetes 上无感升级。
- 提供服务在 crash 情况下，容器内 Debug 能力。
- 支持使用 kerberos 认证服务。
- 支持部署 Doris 存算分离模式。

## 环境要求  
- Kubernetes 1.19+  
- Doris 的 FE 和 BE 组件正常启动至少需要8c和8G资源

## 安装  
1. 安装 DorisCluster 资源定义：  
```  
kubectl create -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/crd/bases/doris.apache.com_dorisclusters.yaml
```
2. 安装 Doris-Operator 服务以及所依赖的 RBAC 权限等相关资源：
```
kubectl apply -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/operator/operator.yaml
```
3. 在 Kubernetes 上部署 Doris 集群：
```  
kubectl apply -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/doriscluster-sample.yaml
```
>[!WARNING]
>1. 当定制化 FE 启动配置时，请设置 `enable_fqdn_mode=true`。请参考[官方文档](https://doris.apache.org/zh-CN/docs/3.0/install/cluster-deployment/k8s-deploy/compute-storage-coupled/install-quickstart)了解更详细的使用介绍。

## 文档
- 存算一体
    - [deploy doris operator on kubernetes](https://doris.apache.org/zh-CN/docs/install/deploy-on-kubernetes/install-doris-operator)
    - [config doris to deploy](https://doris.apache.org/zh-CN/docs/install/deploy-on-kubernetes/install-config-cluster)
    - [deploy doris on kubernetes](https://doris.apache.org/zh-CN/docs/install/deploy-on-kubernetes/install-doris-cluster)
    - [how to access doris cluster in kubernetes](https://doris.apache.org/zh-CN/docs/install/deploy-on-kubernetes/access-cluster)
    - [cluster operation](https://doris.apache.org/zh-CN/docs/install/deploy-on-kubernetes/cluster-operation)
- 存算分离
    - [quick start](https://doris.apache.org/zh-CN/docs/3.0/install/deploy-on-kubernetes/separating-storage-compute/install-doris-cluster)
    - [deploy foundationDB](https://doris.apache.org/zh-CN/docs/3.0/install/deploy-on-kubernetes/separating-storage-compute/install-fdb)
    - [config meta service](https://doris.apache.org/zh-CN/docs/3.0/install/deploy-on-kubernetes/separating-storage-compute/config-ms)
    - [config fe specification](https://doris.apache.org/zh-CN/docs/3.0/install/deploy-on-kubernetes/separating-storage-compute/config-fe)
    - [config compute specification](http://doris.apache.org/zh-CN/docs/3.0/install/deploy-on-kubernetes/separating-storage-compute/config-cg)

## 开源支持
- [CN Forum](https://ask.selectdb.com/)
- [github issues](https://github.com/apache/doris-operator/issues)
- [Slack](https://apachedoriscommunity.slack.com/archives/C02T886T5AR)