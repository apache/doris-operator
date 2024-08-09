DorisCluster | [DorisDisaggregatedCluster](DISAGGREGATED-README.md)

English | [中文](README-CN.md)
# doris-operator
Doris-Operator for doris creates, configures and manages doris cluster running on kubernetes. Operator provide deploy and manage fe, be, cn，broker components.
Users custom `DorisCluster` CRD to deploy doris as demand.

## Features
- Create Doris clusters by custom DorisCluster resource
- Customized storage provisioning(VolumeClaim templates)
- Customized pod templates
- Doris configuration management
- Doris version upgrades
- Provided HorizontalPodAutoscaler v1 and v2 versions for compute node.

## Requirements
- Kubernetes 1.19+
- Doris's components need 8c cpu and 8G memory at least to normal start.

## Installation
1. Install custom resource definitions:  
```
kubectl create -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/crd/bases/doris.selectdb.com_dorisclusters.yaml
```
2. Install the operator with its RBAC rules:  
the default deployed namespace is doris, when deploy on specific namespace, please pull yaml and update `namespace` field.
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s  https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/operator/operator.yaml
```
## Get Started to Deploy Doris
The [Quick Start Guide](./doc/examples) have some examples to deploy doris on kubernetes. they represent some mode to deploy doris on different situation.   
Example specify 8 cores and 16GB of memory for every fe or be, and deployed 3 fe and 3 be. Please confirm the K8s cluster have enough resources.  
for only deploy fe and be without persistentVolume:
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/doriscluster-sample.yaml
```
This [doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml) displayed to deploy doris with [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/) mode to provide persistent Volume.

>[!WARNING]
>1. currently operator only supports the fqdn mode to deploy doris on kubernetes. when the operator uses the official image to deploy container, the relevant work service will set the `enable_fqdn_mode` as true automatically. by running the doris docker container without k8s-operator, fqdn mode is closed by default. for other configurations about deploying doris on kubernetes, refer to [example/doriscluster-sample-configmap.yaml](./doc/examples/doriscluster-sample-configmap.yaml).
>2. fe and be print log by `kubectl logs -ndoris -f ${pod_name}` also in /opt/apache-doris/fe/log, /opt/apache-doris/be/log in pod. When have not log processing system on k8s, mount a volume for log directory is good idea. the config to mount volume for log can reference the doc[example/doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml).
