# Deploy Doris Cluster by Helm
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/doris)](https://artifacthub.io/packages/search?repo=doris)

This chart for deploying doris on kubernetes use Doris-Operator. Before using this chart, please install doris-operator as [install doc](https://artifacthub.io/packages/helm/doris/doris-operator).  

## Install doris on Kubernetes

1. Add helm-chart repo of Doris-Operator in helm.  
```bash
helm repo add selectdb https://charts.selectdb.com
```
2. please create namespace for deploying doris in kubernetes. example use `doris` as namespace's name.  
```bash
kubectl create namespace doris
```

3. Install the doris use doriscluster chart.
- use default config for deploying doris.
  This deploy only deploy fe and be components using default storageClass for providing persistent volume.
  ```bash
  helm install --namespace doris doriscluster selectdb/doris
  ```
- custom doris deploying.   
  when you want to specify resources or different deployment type, please custom the [`values.yaml`](./values.yaml) and use next command for deploying.  
  ```bash
  helm install --namespace doris -f values.yaml doriscluster selectdb/doris 
  ```

## Uninstall doriscluster Chart
Please confirm the doris cluster is not used, When using next command to uninstall doris.   
```bash
helm uninstall doris
```
