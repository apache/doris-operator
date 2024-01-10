# Deploy Doris Cluster by Helm
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/doris)](https://artifacthub.io/packages/search?repo=doris)

This chart for deploying doris on kubernetes use Doris-Operator. Before using this chart, please install doris-operator as [install doc](https://artifacthub.io/packages/helm/doris/doris-operator).  

## Install doris 

### Add helm-chart repo and install doris-operator 
this document and doris-operator installation document are duplicated. you can skip If they have already been executed completely.
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
    NAME                       CHART VERSION    APP VERSION   DESCRIPTION
    selectdb/doris-operator    1.3.1            1.3.1         Doris-operator for doris creat ...
    selectdb/doris             1.3.1            2.0.3         Apache Doris is an easy-to-use ...
    ```
4. Install doris-operator (with default config in a namespace named `doris`)
   ```Bash
   $ helm install operator selectdb/doris-operator
   ```

### Install the doris use doriscluster  
- Use default config for deploying doris 
  This deploy only deploy fe and be components using default storageClass for providing persistent volume.
  ```bash
  $ helm install doriscluster selectdb/doris
  ```
- Custom doris deploying 
  when you want to specify resources or different deployment type, please custom the [`values.yaml`](./values.yaml) and use next command for deploying.  
  ```bash
  $ helm install -f values.yaml doriscluster selectdb/doris 
  ```

### Validate installation status
After executing the installation command, deployment and distribution, service deployment scheduling and startup will take a certain amount of time. Check the deployment status of Pods through the kubectl get pods command.  
Observe that the Pod of `doriscluster` is in the `Running` state and all containers in the Pod are ready, that means, the deployment is successful.

   ```Bash
    $ kubectl get pod --namespace doris
    NAME                     READY   STATUS    RESTARTS   AGE
    doriscluster-helm-fe-0   1/1     Running   0          1m39s
    doriscluster-helm-fe-1   1/1     Running   0          1m39s
    doriscluster-helm-fe-2   1/1     Running   0          1m39s
    doriscluster-helm-be-0   1/1     Running   0          16s
    doriscluster-helm-be-1   1/1     Running   0          16s
    doriscluster-helm-be-2   1/1     Running   0          16s
   ```

## Uninstall doriscluster
Please confirm the Doris is not used, when using next command to uninstall `doriscluster`.
```bash
$ helm uninstall doriscluster
```
