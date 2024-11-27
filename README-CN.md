中文 | [English](README.md)
# doris-operator
Doris-Operator 用于在 Kubernetes 上创建、配置和管理 Doris 集群，能够部署和管理 fe、be、cn、broker 所有组件。  
## 特点
- 通过自定义 DorisCluster 资源管控 Doris 集群。
- 提供定制化存储。
- 实现 Doris 在 Kubernetes 上无感升级。
- 提供服务在 crash 情况下，容器内 Debug 能力。

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
