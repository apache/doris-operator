English | [中文](README-CN.md)
# doris-operator
Doris-Operator for doris creates, configures and manages doris cluster running on kubernetes. Operator provide deploy and manage fe, be, cn，broker components.
Users custom `DorisCluster` CRD to deploy doris as demand.

## Features
- Realized Doris management by custom DorisCluster resource.
- Customized storage provisioning.
- Seamless upgrade Doris.
- Provide the debug ability in container when the service crashed.

## Requirements
- Kubernetes 1.19+
- Doris's components need 8c cpu and 8G memory at least to normal start.

## Installation
1. Install custom resource definitions:  
```shell
kubectl create -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/crd/bases/doris.apache.com_dorisclusters.yaml
```
2. Install the operator with its RBAC rules:  
the default deployed namespace is doris, when deploy on specific namespace, please pull yaml and update `namespace` field.
```shell
kubectl apply -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s  https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/operator/operator.yaml
```
3. Install Doris on Kubernetes:
```shell
kubectl apply -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/doriscluster-sample.yaml 
```

>[!WARNING]
>1. When custom the startup configuration, pelease set  `enable_fqdn_mode=true`. Pelease reference [the offical doc](https://doris.apache.org/docs/3.0/install/cluster-deployment/k8s-deploy/compute-storage-coupled/install-config-cluster) for how to use.
