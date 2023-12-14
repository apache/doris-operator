# Deploy Operator by doris-operator Chart

[Helm](https://helm.sh/) is a package manager for Kubernetes. A [Helm Chart](https://helm.sh/docs/topics/charts/) is a Helm package and contains all of the resource definitions necessary to run an application on a Kubernetes cluster. This topic describes how to use Helm to automatically deploy a Doris operator on a Kubernetes cluster.

## Install doris-operator Chart

1. Add the Helm Chart Repo for Doris. The Helm Chart contains the definitions of the doris-perator and the custom resource doriscluster.
    1. Add the selectdb repository:

       ```Bash
       helm repo add selectdb https://charts.selectdb.com
       ```

    2. Create a namespace for doris-operator:

       ```Bash
       kubectl create namespace doris
       ```

    3. Update the Helm Chart Repo to the latest version.

        ```Bash
        helm repo update
        ```

    4. View the Helm Chart Repo that you added.

       ```Bash
       $ helm search repo selectdb
       NAME                         CHART VERSION    APP VERSION  DESCRIPTION
       selectdb/operator            0.1.0            1.3.0        A Helm chart for Apache Doris Kubernetes Operator
       selectdb/doriscluster        0.1.0            2.0.2        A Helm chart for Apache Doris cluster
       ```

2. Install the operator Chart.

   ```Bash
   helm install --namespace doris operator selectdb/operator
   ```

   Please see [values.yaml](./values.yaml) for more details.

## Uninstall operator Chart

```Bash
helm uninstall doris-operator
```
