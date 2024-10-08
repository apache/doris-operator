# Deploy Doris-Operator by Helm-Chart

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/doris)](https://artifacthub.io/packages/search?repo=doris)

Doris-Operator is plugins of Kubernetes controller for providing doris to user. Doris-Operator be build with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder). This helm-chart deploy [doris-operator](https://github.com/selectdb/doris-operator) on Kubernetes.
## Install doris-operator

### Add helm-chart repo of doris-operator in helm 

This helm chart have resources about RBAC , deployment ...etc for doris-operator running.
    
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

### Install the doris-operator
- Install doris-operator with default config in a namespace named `doris` 
   ```Bash
   $ helm install operator selectdb/doris-operator
   ```
- The repo defines the basic function for running doris-operator, Please use next command to deploy doris-operator, when you have completed customization of [`values.yaml`](./values.yaml) 
   ```Bash
   $ helm install -f values.yaml operator selectdb/doris-operator 
   ```
  
### Validate installation Status
Check the deployment status of Pods through the kubectl get pods command. Observe that the Pod of doris-operator is in the Running state and all containers in the Pod are ready, that means, the deployment is successful.
   ```Bash
   $ kubectl get pod --namespace doris
   NAME                              READY   STATUS    RESTARTS   AGE
   doris-operator-866bd449bb-zl5mr   1/1     Running   0          18m
   ```

## Uninstall doris-operator 
Please confirm that Doris is not running in Kubernetes, use next command to uninstall `doris-operator`.
```Bash
$ helm uninstall operator
```
