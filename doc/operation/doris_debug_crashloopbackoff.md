English | [中文](doris_debug_crashloopbackoff_ch.md)

# Debug `CrashLoopBackOff`
Service will failed to startup on k8s for some unexpected reason.When pod in `CrashLoopBackOff` status, `kubectl describe` and `logs` will not provide support for fixing the issue.
So operator provide a `debug` mode fo starting pod. Using debug mode to start, pod will always start successfully. But the service in pod will not start ok. The doc describe how to use `debug mode` to debug service issue.
## Start on Debug Mode
When pod enter into `CrashLoopBackOff` status, follow next steps to debug service.
1. Add a annotation to the crash pod.
```
    kubectl annotate pod ${pod_name} -n ${namespace} selectdb.com.doris/runmode=debug
```
When the pod start in next. Service will detect the annotation and start in debug mode.  
2. Service start with `Debug` mode will always start successfully. you can use follow command enter into container.
```
    kubectl -n ${namespace} exec -ti ${pod_name} bash
```
3.In debug mode the pod is running, but the service not start ok. when you want start service by yourself, you should edit the config file update the port about http(fe=http_port,be=webserver_port) as other value. Next, execute the script `start_xx.sh` in `/opt/apache-doris/xx/bin` directory to start service.
## Exit Debug mode
When you fix the started failed issue, you can delete the response pod, the pod will restart as normal mode.
```
    kubectl delete pod ${pod_name} -n ${namespace}
```