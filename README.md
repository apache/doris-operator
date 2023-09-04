# doris-operator
Doris-operator for doris creates, configures and manages doris cluster running on kubernetes. Operator provide deploy and manage fe, be, cn components.
Users custom `DorisCluster` CRD to deploy doris as demand.

## Features
- Create Doris clusters defined as custom resource
- Customized storage provisioning(VolumeClaim templates)
- Customized pod templates
- Doris configuration management
- Doris version upgrades
- Provided HorizontalPodAutoscaler v1 and v2 versions for compute node.

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
The [Quick Start Guide](./doc/examples) have some examples to deploy doris on kubernetes. they represent some mode to deploy doris on different situation.
for only deploy fe and be without persistentVolume:
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/master/doc/examples/doriscluster-sample.yaml
```
This [doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml) displayed to deploy doris with [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/) mode to provide persistent Volume.

## Notice 
 1. Now operator only support the fqdn mode to deploy doris on kubernetes. you should config set `enable_fqdn_mode = true` in every component config file. The apache doris docker image default value is false. recommend you reference [example/doriscluster-sample-comfigmap.yaml](./doc/examples/doriscluster-sample-comfigmap.yaml) to custom config to deploy doris on kubernetes.
 3. fe and be print log in /opt/apache-doris/fe/log, /opt/apache-doris/be/log. When have not log processing system on k8s, mount a volume for log directory is good idea. the config to mount volume for log can reference the doc[example/doriscluster-sample-storageclass.yaml](./doc/examples/doriscluster-sample-storageclass.yaml).