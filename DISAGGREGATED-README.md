English | [中文](DISAGGREGATED-README-CN.md)

# Deploy Separation of Storage and Compute Cluster
Separation of storage and compute is an architecture pattern provided by Doris from 3.0.0 version. The separation of storage and compute can significantly reduce storage costs, allowing data to be stored in cheaper object storage without significantly compromising performance. This not only reduces costs but also better responds to scenarios with rapidly changing demands for computing resources.
## Requirements
- Kubernetes 1.19+
- the `open files` should greater than 65535 for host system config. (ulimit -n)
- doris >= 3.0.2

>[!NOTE]
>1. The total resources of cpu and memory about K8s worker should greater than the required to deploy doris cluster.
>2. The resources of a K8s worker node should be greater than the resources required by one fe or be. fe or be default resource requirement: 4c, 4Gi.

## Install FoundationDB
To deploy a storage-computing separation cluster on K8s, you need to deploy fdb in advance. For deployment on k8s, refer to the [Quick Deployment Document](https://github.com/FoundationDB/fdb-kubernetes-operator) in the operator provided by fdb officially.

### install the fdb-kubernetes-operator and CRDs

```
kubectl create -f https://raw.githubusercontent.com/FoundationDB/fdb-kubernetes-operator/main/config/crd/bases/apps.foundationdb.org_foundationdbclusters.yaml
kubectl create -f https://raw.githubusercontent.com/FoundationDB/fdb-kubernetes-operator/main/config/crd/bases/apps.foundationdb.org_foundationdbbackups.yaml
kubectl create -f https://raw.githubusercontent.com/FoundationDB/fdb-kubernetes-operator/main/config/crd/bases/apps.foundationdb.org_foundationdbrestores.yaml
kubectl apply -f https://raw.githubusercontent.com/foundationdb/fdb-kubernetes-operator/main/config/samples/deployment.yaml
```

Expected result:

```
customresourcedefinition.apiextensions.k8s.io/foundationdbclusters.apps.foundationdb.org created
customresourcedefinition.apiextensions.k8s.io/foundationdbbackups.apps.foundationdb.org created
customresourcedefinition.apiextensions.k8s.io/foundationdbrestores.apps.foundationdb.org created
serviceaccount/fdb-kubernetes-operator-controller-manager created
clusterrole.rbac.authorization.k8s.io/fdb-kubernetes-operator-manager-clusterrole created
clusterrole.rbac.authorization.k8s.io/fdb-kubernetes-operator-manager-role created
rolebinding.rbac.authorization.k8s.io/fdb-kubernetes-operator-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/fdb-kubernetes-operator-manager-clusterrolebinding created
deployment.apps/fdb-kubernetes-operator-controller-manager created
```

### set up a fdb sample cluster

```
kubectl apply -f https://raw.githubusercontent.com/foundationdb/fdb-kubernetes-operator/main/config/samples/cluster.yaml
```

Use the `kubectl get fdb` command to check the cluster building status until the `AVAILABLE` is found to be true.

Expected result:

```
NAME           GENERATION   RECONCILED   AVAILABLE   FULLREPLICATION   VERSION   AGE
test-cluster   1            1            true        true              7.1.26    3m18s
```

## Install Doris Operator
1. deploy CustomResourceDefinitions
```
kubectl create -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/crd/bases/crds.yaml
```
Expected result:
```
customresourcedefinition.apiextensions.k8s.io/dorisclusters.doris.selectdb.com created
customresourcedefinition.apiextensions.k8s.io/dorisdisaggregatedclusters.disaggregated.cluster.doris.com created
```
2. Install the operator with its RBAC rules:
```
kubectl apply -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/config/operator/disaggregated-operator.yaml
```
Expected result:
```
kubectl -n doris get pods
NAME                                         READY   STATUS    RESTARTS   AGE
doris-operator-5b667b4954-d674k              1/1     Running   0          11s
```
## Deploy an Separation of Storage and Compute Cluster
[examples](./doc/examples/disaggregated/cluster) contains deployment examples for common configurations. The simple example deployment as follows:
Deploy `DorisDisaggregatedCluster` resource:
```
kubectl apply -f https://raw.githubusercontent.com/apache/doris-operator/$(curl -s https://api.github.com/repos/apache/doris-operator/releases/latest | grep tag_name | cut -d '"' -f4)/doc/examples/disaggregated/cluster/ddc-sample.yaml
```
>[!NOTE]
> 1. FDB's k8s deployment requires at least three hosts as worker nodes in k8s. If the number of worker nodes in k8s is less than 3, please use the [singleton mode deployment](./doc/examples/disaggregated/fdb/cluster-single.yaml) provided by Doris operator
> 2. For detailed deployment, please refer to [official doc](https://doris.apache.org/docs/dev/install/cluster-deployment/k8s-deploy/compute-storage-decoupled/install-quickstart).