# Deploy Doris Storage-Compute Decoupled Cluster
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/doris)](https://artifacthub.io/packages/search?repo=doris)

## Prepare
1. install doris operator
    Deploy the doris operator in your kubernetes cluster. deploy doris operator have two methods: [directly yaml](https://github.com/apache/doris-operator); [doris operator helm chart](https://artifacthub.io/packages/helm/doris/doris-operator).
2. install foundationDB
    Doris storage-compute decoupled cluster use the FoundationDB as meta storage component, please prepare a foundationdb cluster before deploy doris storage-compute decoupled cluster. please refer the [foundationdb official doc site](https://apple.github.io/foundationdb/administration.html#starting-and-stopping) to deploy on machine, or you can use the [fdb-kubernetes-operator](https://github.com/FoundationDB/fdb-kubernetes-operator) to deploy on kubernetes. doris operator provide a sample [helm chart](https://artifacthub.io/packages/helm/doris/doris-foundationdb) that integrate the official fdb-kubernetes-operator chart to deploy foundationdb on kubernetes. 

## Install
### Add helm-chart repo
1. add the selectdb repository
    ```Bash
    helm repo add selectdb https://charts.selectdb.com
    ```
2. update the helm chart repo to the latest version
    ```Bash
    helm repo udpate selectdb
    ```
3. check the helm chart repo is the latest version
    ```Bash
    $ helm search repo selectdb
    NAME                           CHART VERSION     APP VERSION   DESCRIPTION
    selectdb/doris-operator        25.4.0            1.3.1         Doris-operator for doris creat ...
    selectdb/doris                 25.4.0            2.1.7         Apache Doris is an easy-to-use ...
    selectdb/doris-foundationdb    0.2.0             v2.3.0        A Helm chart for foundationDB  ...
    ```
### Install the doris storage-compute decoupled cluster
#### Use default config
- fdb deployed on kubernetes
    ```Bash
    helm install doris-disaggregated --set  msSpec.fdb.namespace={namespace} --set msSpec.fdb.fdbClusterName={fdbClusterName}
    ```
    please use the real namespace replace the {namespace} as the foundationdb deployed namespace, if you use the [fdb-kubernetes-operator](https://github.com/FoundationDB/fdb-kubernetes-operator) or [doris-foundationdb](https://artifacthub.io/packages/helm/doris/doris-foundationdb) deploy foundationdb.
    {fdbClusterName} is the `FoundationDBCluster` resource's name. 
- fdb deployed on machine
    ```Bash
    helm install doris-disaggregated --set msSpec.fdb.address={address}
    ```
  {address} is the address of fdb accessed, it is content of [fdb.cluster file](https://apple.github.io/foundationdb/administration.html#cluster-files).

#### Custom deploying
1. use the follow command to download and unpack the chart
    ```Bash
    helm pull --untar selectdb/doris-disaggregated
    ```

2. helm install doris storage-compute decoupled cluster
    when you want to specify resources or different deployment type, please custom the [`values.yaml`](./values.yaml) and use next command for deploying.
    ```Bash
    helm install doris-disaggregated -f values.yaml doris-disaggregated
    ```
## Uninstall cluster
Please confirm the Doris storage-compute decoupled cluster is not used, when using next command to uninstall `doris-disaggregated`.
```Bash
helm uninstall doris-disaggregated
```
