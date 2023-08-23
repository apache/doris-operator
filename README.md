# doris-operator
Doris-operator for doris creates, configures and manages doris cluster running on kubernetes.

## Features
- Create Doris clusters defined as custom resources
- Customized storage provisioning(VolumeClaim templates)
- Customized pod templates
- Doris configuration management
- Doris version upgrades

## Requirements
- Kubernetes 1.19+

## Installation
1. Install custom resource definitions:  
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/config/crd/bases/doris.selectdb.com_dorisclusters.yaml
```
2. Install the operator with its RBAC rules:  
the default deployed namespace is doris, when deploy on specific namespace, please pull yaml and update `namespace` field.
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/config/operator/operator.yaml
```

## Get Started to Deploy Doris
The [Quick Start Guide](./doc/examples) have examples for deploy doris on kubernetes. It provides examples for different features to deploy.  
Now the operator provide fe and be deployment and management, in next generation will complete cn deployment and management.
for fe and be deploy example:
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/doc/examples/doriscluster-sample.yaml
```

## Notice 
 Now operator only support the fqdn mode to deploy doris on kubernetes. you should config set `enable_fqdn_mode = true` in every component config file.
 the apache doris docker image default value is false. recommend you reference [example/doriscluster-sample-comfigmap.yaml](./doc/examples/doriscluster-sample-comfigmap.yaml) custom config to deploy doris on kubernetes.