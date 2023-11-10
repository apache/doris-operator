# doris-operator
Doris-operator 用于在 Kubernetes 上创建、配置和管理 Doris 集群。Doris-operator提供部署和管理 fe、be、cn、broker 组件。
用户自定义的 `DorisCluster` CRD 以按需求部署 doris。  
## 特点  
- 通过自定义 DorisCluster 资源创建 Doris 集群  
- 提供定制化存储（VolumeClaim 模板）  
- 定制的 pod 模板  
- Doris 配置管理  
- Doris 版本升级  
- 为计算节点提供水平弹性拓展（HorizontalPodAutoscaler）的 v1 和 v2 版本。  
## 环境要求  
- Kubernetes 1.19+  
## 安装  
1. 安装自定义资源定义：  
```  
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/config/crd/bases/doris.selectdb.com_dorisclusters.yaml  
```  
2. 安装具有其 RBAC 规则的 Doris-operator：  
   默认部署 namespace 是 doris，当在特定 namespace 部署时，请拉取 yaml 并更新 `namespace` 字段。  
```  
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/config/operator/operator.yaml  
```  
## 开始部署 Doris  
[快速开始指南](./doc/examples) 提供了一些示例展示 在 kubernetes 上部署 doris的不同模式。  
仅部署不带持久卷的 fe 和 be：  
```  
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/doc/examples/doriscluster-sample.yaml  
```  
配置 [doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml) 显示了使用 [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/) 模式部署带有持久卷的 doris。  
## 注意  
1. 目前 Doris-operator 仅支持 fqdn 模式在 kubernetes 上部署 doris。当 Doris-operator 使用官方镜像部署容器时，相关工作服务会自动将 `enable_fqdn_mode` 设置为 true。通过运行没有 k8s-operator 的 docker 容器，默认关闭 fqdn 模式。有关在 kubernetes 上部署 doris 的其他配置，请参考 [example/doriscluster-sample-comfigmap.yaml](./doc/examples/doriscluster-sample-comfigmap.yaml)。  
2. fe 和 be 在 /opt/apache-doris/fe/log、/opt/apache-doris/be/log 中打印日志。当 k8s 上没有日志处理系统时，建议为日志目录挂载文件目录。关于为日志挂载文件目录的配置可以参考文档 [example/doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml)。  
