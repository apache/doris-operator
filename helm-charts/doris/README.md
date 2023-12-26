# Deploy Doris Cluster by doriscluster Chart

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/doris)](https://artifacthub.io/packages/search?repo=doris)

## Install doriscluster Chart

1. [Add the selectdb Helm repository and Install doris-operator Chart](../doris-operator/README.md).

    ```bash
    $ helm repo add selectdb https://charts.selectdb.com
    ```
    
2. Install the [doris opreator](../doris-operator/README.md) Chart.
   <div class="artifacthub-widget" data-url="https://artifacthub.io/packages/helm/doris/operator" data-theme="light" data-header="true" data-stars="true" data-responsive="false"><blockquote><p lang="en" dir="ltr"><b>operator</b>: A Helm chart for Apache Doris Kubernetes Operator</p>&mdash; Open in <a href="https://artifacthub.io/packages/helm/doris/operator">Artifact Hub</a></blockquote></div><script async src="https://artifacthub.io/artifacthub-widget.js"></script>

3. Install the doriscluster Chart.

    ```bash
    helm install --namespace doris doriscluster selectdb/doriscluster
    ```

   Please see  [values.yaml](./values.yaml)  for more details.

## Uninstall doriscluster Chart

```bash
helm uninstall doriscluster
```
