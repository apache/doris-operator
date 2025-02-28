English | [中文](README-CN.md)
# doris-operator
Doris-Operator for doris creates, configures and manages doris cluster running on kubernetes. Operator provide deploy and manage fe, be, cn，broker components.
Users custom `DorisCluster` CRD to deploy doris as demand.

[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg?color=f5deb3)](https://www.apache.org/licenses/LICENSE-2.0.html)
[![Operator Release](https://img.shields.io/github/v/release/apache/doris-operator?color=00FFFF)](https://img.shields.io/github/v/release/apache/doris-operator)
[![Tags](https://img.shields.io/github/v/tag/apache/doris-operator?label=latest%20tag&color=00FF7F)](https://img.shields.io/github/v/tag/apache/doris-operator?label=latest%20tag&color=00FF7F)
[![docker pull](https://img.shields.io/docker/pulls/apache/doris?color=1E90FF&logo=docker)](https://img.shields.io/docker/pulls/apache/doris)
[![issues](https://img.shields.io/github/issues-search?query=repo%3Aapache%2Fdoris-operator%20is%3Aopen&color=AFEEEE&label=issues)](https://img.shields.io/github/issues-search?query=repo%3Aapache%2Fdoris-operator%20is%3Aopen)
[![Go Version](https://img.shields.io/github/go-mod/go-version/apache/doris-operator?color=00BFFF)](https://img.shields.io/github/go-mod/go-version/apache/doris-operator)
[![docs](https://img.shields.io/website?url=https%3A%2F%2Fdoris.apache.org%2Fdocs%2Finstall%2Fdeploy-on-kubernetes%2Finstall-config-cluster&label=docs&color=7FFF00)](https://doris.apache.org/docs/install/deploy-on-kubernetes/install-config-cluster)

## Features
- Realized Doris management by custom DorisCluster resource.
- Customized storage provisioning.
- Seamless upgrade Doris.
- Provide the debug ability in container when the service crashed.
- Support kerberos certification of doris on Kubernetes.
- Support deploy storage-compute separation mode of Doris.

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
>1. When custom the FE startup configuration, please set  `enable_fqdn_mode=true`. Please refer to [the official doc](https://doris.apache.org/docs/3.0/install/cluster-deployment/k8s-deploy/compute-storage-coupled/install-config-cluster) for how to use.
