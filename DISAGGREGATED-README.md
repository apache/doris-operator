English | [中文](DISAGGREGATED-README-CN.md)

# Deploy Separation of Storage and Compute Cluster
Separation of storage and compute is an architecture pattern provided by Doris from 3.0.0 version. The separation of storage and compute can significantly reduce storage costs, allowing data to be stored in cheaper object storage without significantly compromising performance. This not only reduces costs but also better responds to scenarios with rapidly changing demands for computing resources.
## Custom Resources
In separation of storage and compute architecture, cluster contains the following components: fdb, ms, recycler, fe, be. Doris Operator deploys fdb, ms, and recycler using the 'DorisDisaggregatedMetaService' resource. The 'DorisDisaggregatedCluster' resource be used to deploy fe and compute cluster (the group of be).
## Requirements
- Kubernetes 1.19+
- the `open files` should greater than 65535 for host system config. (ulimit -n)

>[!NOTE]
>1. The total resources of cpu and memory about K8s worker should greater than the required to deploy doris cluster.
>2. The resources of a K8s worker node should be greater than the resources required by one fe or be. fe or be default resource requirement: 4c, 4Gi.

## Install Operator
1. deploy CustomResourceDefinitions
```
kubectl create -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/crd/bases/crds.yaml
```
Expected result:
```
customresourcedefinition.apiextensions.k8s.io/foundationdbclusters.apps.foundationdb.org created
customresourcedefinition.apiextensions.k8s.io/foundationdbbackups.apps.foundationdb.org created
customresourcedefinition.apiextensions.k8s.io/foundationdbrestores.apps.foundationdb.org created
customresourcedefinition.apiextensions.k8s.io/dorisclusters.doris.selectdb.com created
customresourcedefinition.apiextensions.k8s.io/dorisdisaggregatedclusters.disaggregated.cluster.doris.com created
customresourcedefinition.apiextensions.k8s.io/dorisdisaggregatedmetaservices.disaggregated.metaservice.doris.com created
```
2. Install the operator with its RBAC rules:
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/operator/disaggregated-operator.yaml
```
Expected result:
```
kubectl -n doris get pods
NAME                                         READY   STATUS    RESTARTS   AGE
doris-operator-fdb-manager-d75574c47-b2sqx   1/1     Running   0          11s
doris-operator-5b667b4954-d674k              1/1     Running   0          11s
```
## Deploy an Separation of Storage and Compute Cluster
[examples](./doc/examples/disaggregated/cluster) contains deployment examples for common configurations. The simple example deployment as follows:
1. Deploy `DorisDisaggregatedMetaService` resource:
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/disaggregated/metaservice/ddm-sample.yaml
```
Expected result:
```
kubectl get ddm
NAME                   FDBSTATUS   MSSTATUS   RECYCLERSTATUS
meta-service-release   Available   Ready      Ready
```
2. Deploy `ConfigMap` that contains object information for cluster:
Separation of storage and compute uses object storage as the backend storage, requiring prior planning of the object storage to be used. Configure object storage information in JSON format according to the [Storage and computation separation interface](https://doris.apache.org/docs/dev/compute-storage-decoupled/creating-cluster/#built-in-storage-vault) format.
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/disaggregated/cluster/object-store-info.yaml
```
Expected result:
```
configmap/vault-test created
```
>[!NOTE]
>1. Deploying a storage computing separation cluster requires pre-planning the object storage to be used, Configure the object storage information to the namespace that the Doris storage and computation separation cluster needs to deployed, through a `ConfigMap`.
>2. The configuration in the examples only displays the basic configuration required for object storage, all values are fictional and cannot be used in real-life scenarios. If you need to build a real and usable cluster, please use real data to fill in.

3. Deploy `DorisDisaggregatedCluster` resource:
```
kubectl apply -f https://raw.githubusercontent.com/selectdb/doris-operator/$(curl -s https://api.github.com/repos/selectdb/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/disaggregated/cluster/ddc-sample.yaml
```
Expected result:
```
kubectl get ddc                                                                                                
NAME                         CLUSTERHEALTH   FEPHASE   CCCOUNT   CCAVAILABLECOUNT   CCFULLAVAILABLECOUNT
test-disaggregated-cluster   green           Ready     1         1                  1                          
```