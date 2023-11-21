中文 | [English](doris_debug_crashloopbackoff.md)
# Debug `CrashLoopBackOff`
在k8s环境中服务因为一些预期之外的事情会进入`CrashLoopBackOff`状态，在这种状态下，单纯通过describe和log无法判定服务出问题的原因。当服务进入`CrashLoopBackOff`状态时，需要有一种机制允许部署服务的pod进入running状态方便用户通过`exec`进入容器内进行debug。
doris-operator提供了`debug`的运行模式。下面描述了当服务进入`CrashLoopBackOff`时如何进入`debug`模式进行人工debug，以及解决后如何恢复到正常启动状态。
## Debug模式启动
当服务一个pod进入`CrashLoopBackOff`或者正常运行过程中无法再正常启动时，通过一下步骤让服务进入`debug`模式，进行手动启动服务查找问题。
1. 通过一下命令给运行有问题的pod进行添加annnotation
```
    kubectl annotate pod ${pod_name} -n ${namespace} selectdb.com.doris/runmode=debug
```
当服务进行下一次重启时候，服务会检测到标识debug模式启动的annotation就会进入debug模式启动。  
2. 当服务进入`debug`模式，此时服务的pod显示为正常状态，用户可以通过如下命令进入pod内部。
```
    kubectl -n ${namespace} exec -ti ${pod_name} bash
```
3. debug下手动启动服务，当用户进入pod内部，通过修改对应配置文件有关http的端口进行手动执行`start_xx.sh`脚本，脚本目录为`/opt/apache-doris/xx/bin`下。
## 退出Debug模式
当服务定位到问题后需要退出debug运行，此时只需要按照如下命令删除对应的pod，服务就会按照正常的模式启动。
```
    kubectl delete pod ${pod_name} -n ${namespace}
```
