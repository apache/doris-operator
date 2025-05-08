# Deploy FoundationDb Cluster by Helm
This chart will deploy foundationdb operator and a foundationdb cluster. The foundationdb operator chart clone from [foundationdb operator repo](https://github.com/FoundationDB/fdb-kubernetes-operator/tree/main/charts/fdb-operator).
The foundationdb cluster is a practice for doris.

## Install foundationdb and operator
1. Add the selectdb repository
    ```Bash
    $ helm repo add selectdb https://charts.selectdb.com
    ```
2. Update the Helm Chart Repo to the latest version
    ```Bash
    $ helm repo update selectdb
    ```
3. Check the Helm Chart Repo is the latest version
    ```Bash
    $ helm search repo selectdb
    NAME                        CHART VERSION    APP VERSION   DESCRIPTION
    selectdb/doris-operator     1.3.1            1.3.1         Doris-operator for doris creat ...
    selectdb/doris              1.3.1            2.0.3         Apache Doris is an easy-to-use ...
    selectdb/doris-foundationdb 0.2.0            7.1.38        A Helm chart for FoundationDB
    ```
4. Install doris-foundationdb
   ```Bash
   $ helm install doris-foundationdb selectdb/doris-foundationdb
   ```