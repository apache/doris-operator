中文 | [English](README.md)
# doris-operator
Doris-operator 用于在 Kubernetes 上创建、配置和管理 Doris 集群。Doris-Operator 能够部署和管理 fe、be、cn、broker 服务组件。
用户自定义的 `DorisCluster` CRD 以按需求部署 doris。  
## 特点  
- 通过自定义 DorisCluster 资源创建 Doris 集群  
- 提供定制化存储（VolumeClaim 模板）  
- 定制的 pod 模板  
- Doris 配置文件与组件解耦，灵活管理相关配置   
- Doris 版本平滑升级  
- 提供 HorizontalPodAutoscaler v1 和 v2 版本满足不同k8s环境计算节点的自动弹性扩缩容  
## 环境要求  
- Kubernetes 1.19+  
## 安装  
1. 安装 DorisCluster 资源定义：  
```  
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/config/crd/bases/doris.selectdb.com_dorisclusters.yaml  
```  
2. 安装 Doris-Operator 在 k8s 上部署所需 RBAC 权限相关资源，默认部署到名称 'doris' 的 `namespace` 。  
```  
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/config/operator/operator.yaml  
```  
## 开始部署 Doris  
[快速开始指南](./doc/examples) 提供了一些通用场景下部署 doris 的示例。。  
部署不带持久卷的 fe 和 be：  
```  
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/doc/examples/doriscluster-sample.yaml  
```  
配置 [doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml) 显示了使用 [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/) 模式提供存储卷部署 doris 的样例。  
## 注意  
1. 目前 Doris-operator 仅支持 fqdn 模式在 kubernetes 上部署 doris。当 Doris-operator 使用官方镜像部署容器时，相关工作服务会自动将 `enable_fqdn_mode` 设置为 `true`。通过 docker 运行（未经 k8s-operator）容器时，默认关闭 fqdn 模式。有关在 kubernetes 上部署 doris 的其他配置，请参考 [example/doriscluster-sample-comfigmap.yaml](./doc/examples/doriscluster-sample-comfigmap.yaml)。  
2. fe 和 be 可在`kubectl logs -ndoris -f ${pod_name}` 命令下查看日志，也可以在容器内部的 `/opt/apache-doris/fe/log` 或 `/opt/apache-doris/be/log` 中打印日志。当 k8s 上没有日志处理系统时，建议为日志目录挂载文件目录，便于追溯较早的大量运行日志。关于为日志挂载文件目录的配置可以参考文档 [example/doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml)。  
