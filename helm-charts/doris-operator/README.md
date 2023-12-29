# Deploy Doris-Operator by Helm-Chart

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/doris)](https://artifacthub.io/packages/search?repo=doris)

Doris-Operator is plugins of Kubernetes controller for providing doris to user. Doris-Operator be build with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder). This helm-chart deploy [doris-operator](https://github.com/selectdb/doris-operator) on Kubernetes.
## Install doris-operator Chart

1. Add helm-chart repo of Doris-Operator in helm. This helm chart have resources about RBAC , deployment ...etc for doris-operator running.
    1. Add the selectdb repository.
    ```Bash
    helm repo add selectdb https://charts.selectdb.com
    ```

    2. Update the Helm Chart Repo to the latest version.
   ```Bash
   helm repo update selectdb
   ```

    3. Check the Helm Chart Repo is the latest version.
   ```Bash
   helm search repo selectdb
   NAME                       CHART VERSION    APP VERSION   DESCRIPTION
   selectdb/doris-operator    1.3.1            1.3.1         Doris-operator for doris creat ...
   selectdb/doris             1.3.1            2.0.3         Apache Doris is an easy-to-use ...
   ```
2. we install doris operator in `doris` namespace, so the first we should create `doris` namespace.
   ```Bash
   kubectl create namespace doris
   ```
3. Install the Doris Operator.
- install doris operator in doris namespace.
   ```Bash
   helm install --namespace doris operator selectdb/doris-operator
   ```
- The repo defines the basic function for running doris-operator, Please use next command to deploy operator, when you have completed customization of [`values.yaml`](./values.yaml).
   ```Bash
   helm install --namespace doris -f values.yaml operator selectdb/doris-operator 
   ```

## Uninstall Doris-Operator
When you have confirmed have not `doris` running in kubernetes, Please use next command to uninstall operator.
```Bash
helm uninstall operator
```
