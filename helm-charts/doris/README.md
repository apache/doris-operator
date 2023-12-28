# Deploy Doris Cluster by doriscluster Chart

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/doris)](https://artifacthub.io/packages/search?repo=doris)

## Install doriscluster Chart

1. [Add the selectdb Helm repository and Install doris-operator Chart](../doris-operator/README.md).

    ```bash
    $ helm repo add selectdb https://charts.selectdb.com
    ```
    
2. Install the [doris opreator](../doris-operator/README.md) Chart.

3. Install the doriscluster Chart.

    ```bash
    helm install --namespace doris doriscluster selectdb/doriscluster
    ```

   Please see  [values.yaml](./values.yaml)  for more details.

## Uninstall doriscluster Chart

```bash
helm uninstall doriscluster
```
