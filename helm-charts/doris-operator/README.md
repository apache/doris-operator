# Deploy Doris-Operator by Helm-Chart

[![Doris repo](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/doris)](https://github.com/apache/doris)
[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg?color=f5deb3)](https://www.apache.org/licenses/LICENSE-2.0.html)
[![Operator Release](https://img.shields.io/github/v/release/apache/doris-operator?color=00FFFF)](https://github.com/apache/doris-operator/releases)
[![Tags](https://img.shields.io/github/v/tag/apache/doris-operator?label=latest%20tag&color=00FF7F)](https://github.com/apache/doris-operator/tags)
[![docker pull](https://img.shields.io/docker/pulls/apache/doris?color=1E90FF&logo=docker)](https://img.shields.io/docker/pulls/apache/doris)
[![issues](https://img.shields.io/github/issues-search?query=repo%3Aapache%2Fdoris-operator%20is%3Aopen&color=AFEEEE&label=issues)](https://github.com/apache/doris-operator/issues)
[![Go Version](https://img.shields.io/github/go-mod/go-version/apache/doris-operator?color=00FFFF)](https://img.shields.io/github/go-mod/go-version/apache/doris-operator)
[![docs](https://img.shields.io/website?url=https%3A%2F%2Fdoris.apache.org%2Fdocs%2Finstall%2Fdeploy-on-kubernetes%2Finstall-config-cluster&label=docs&color=7FFF00)](https://doris.apache.org/docs/install/deploy-on-kubernetes/install-config-cluster)

Doris-Operator is plugins of Kubernetes controller for providing doris to user. Doris-Operator be build with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder). This helm-chart deploy [doris-operator](https://github.com/selectdb/doris-operator) on Kubernetes.
## Install doris-operator

### Add helm-chart repo of doris-operator in helm 

This helm chart have resources about RBAC , deployment ...etc for doris-operator running.
    
1. Add the selectdb repository
    ```Bash
    helm repo add selectdb https://charts.selectdb.com
    ```

2. Update the Helm Chart Repo to the latest version
    ```Bash
    $ helm repo update selectdb
    ```

3. Check the Helm Chart Repo is the latest version
    ```Bash
    helm search repo selectdb
    NAME                       CHART VERSION    APP VERSION   DESCRIPTION
    selectdb/doris-operator    1.3.1            1.3.1         Doris-operator for doris creat ...
    selectdb/doris             1.3.1            2.0.3         Apache Doris is an easy-to-use ...
    ```

### Install the doris-operator
- Install doris-operator in `doris` namespace using the default config:
    ```Bash
    helm install operator selectdb/doris-operator --create-namespace  -n doris
    ```
- Custom the values.yaml, use the follow command to deploy:
    ```Bash
    helm install -f values.yaml operator selectdb/doris-operator --create-namespace -n doris
    ```
  
### Validate installation Status
Check the deployment status of Pods through the kubectl get pods command. Observe that the Pod of doris-operator is in the Running state and all containers in the Pod are ready, that means, the deployment is successful.
```Bash
kubectl get pod --namespace doris
NAME                              READY   STATUS    RESTARTS   AGE
doris-operator-866bd449bb-zl5mr   1/1     Running   0          18m
```

## Uninstall doris-operator 
Please confirm that Doris is not used in Kubernetes, and the data in doris is not valued, use the follow command to uninstall.
```Bash
helm -n doris uninstall operator
```
