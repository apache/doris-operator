# Deploy Doris Cluster by doriscluster Chart

[Helm](https://helm.sh/) is a package manager for Kubernetes. A [Helm Chart](https://helm.sh/docs/topics/charts/) is a Helm package and contains all of the resource definitions necessary to run an application on a Kubernetes cluster. This topic describes how to use Helm to automatically deploy a doris cluster on a Kubernetes cluster.

## Install doriscluster Chart

1. [Add the selectdb Helm repository and Install doris-operator Chart](../doris-operator/README.md).

    ```bash
    $ helm repo add selectdb https://charts.selectdb.com
    ```
    
2. Install the doriscluster Chart.

    ```bash
    helm install --namespace doris doriscluster selectdb/doriscluster
    ```

   Please see [values.yaml](./values.yaml) for more details.

## Uninstall doriscluster Chart

```bash
helm uninstall doriscluster
```
