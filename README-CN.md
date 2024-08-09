DorisCluster | [DorisDisaggregatedCluster](DISAGGREGATED-README-CN.md)
中文 | [English](README.md)
# doris-operator
Doris-Operator 用于在 Kubernetes 上创建、配置和管理 Doris 集群，能够部署和管理 fe、be、cn、broker 所有组件。  
## 特点
- 通过自定义 DorisCluster 资源创建Doris集群  
- 提供定制化存储（PersistentVolumeClaim 模板）  
- 定制 pod 模板  
- Doris 配置文件与组件解耦，灵活管理相关组件配置   
- Doris 版本平滑升级  
- 提供 HorizontalPodAutoscaler v1 和 v2 版本满足不同k8s环境计算节点的自动弹性扩缩容  
## 环境要求  
- Kubernetes 1.19+  
- Doris 的 FE 和 BE 组件正常启动至少需要8c和8G资源
## 安装  
1. 安装 DorisCluster 资源定义：  
```  
kubectl create -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/crd/bases/doris.selectdb.com_dorisclusters.yaml
```
2. 安装 Doris-Operator 服务以及所依赖的 RBAC 权限等相关资源  
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/operator/operator.yaml
```
## 部署 Doris  
[部署范例](./doc/examples)中提供了一些使用Kubernetes特性部署 Doris 的范例。  
默认的部署样例中，每个 fe 和 be 最少需要8核和16G的内存，且每个服务部署3个实例。 在使用默认部署之前，确保 K8s 集群有足够的资源能够部署成功。  
部署使用容器自身存储(重启丢失数据属易失性介质)包含 fe,be 服务的 Doris 集群，命令如下：  
```  
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/doriscluster-sample.yaml
```  
[doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml) 展示使用 [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/) 模式提供存储卷部署 doris 的样例。

>[!WARNING]
>1. 目前 Doris-operator 限定 Doris 在 Kubernetes 上必须使用 FQDN 模式启动和通信。 在使用 [DockerHub selectdb](https://hub.docker.com/?namespace=selectdb) 组织下的官方镜像部署时，`enable_fqdn_mode` 会被默认设置为 `true`。其他方式使用镜像时， fqdn 默认仍然是 false 。详细配置请参考文档 [example/doriscluster-sample-configmap.yaml](./doc/examples/doriscluster-sample-configmap.yaml)。  
>2. 服务正常运行时，可以通过 `kubectl -n doris logs -f {pod_name}` 命令查看日志，也可以到容器内部的 `/opt/apache-doris/fe/log` 或 `/opt/apache-doris/be/log` 中查看日志。当 k8s 上没有日志处理系统时，建议为日志目录挂载存储盘，便于追溯较早的大量运行日志。为日志挂载存储盘可以参考文档 [example/doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml)。  
