<p>Packages:</p>
<ul>
<li>
<a href="#disaggregated.cluster.doris.com%2fv1">disaggregated.cluster.doris.com/v1</a>
</li>
</ul>
<h2 id="disaggregated.cluster.doris.com/v1">disaggregated.cluster.doris.com/v1</h2>
<div>
<p>Package v1 is the v1 version of the API.</p>
</div>
Resource Types:
<ul></ul>
<h3 id="disaggregated.cluster.doris.com/v1.AvailableStatus">AvailableStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.ComputeClusterStatus">ComputeClusterStatus</a>, <a href="#disaggregated.cluster.doris.com/v1.FEStatus">FEStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Available&#34;</p></td>
<td><p>Available represents the service is available.</p>
</td>
</tr><tr><td><p>&#34;UnAvailable&#34;</p></td>
<td><p>UnAvailable represents the service not available for using.</p>
</td>
</tr></tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.ClusterHealth">ClusterHealth
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterStatus">DorisDisaggregatedClusterStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>health</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.Health">
Health
</a>
</em>
</td>
<td>
<p>represents the cluster overall status.</p>
</td>
</tr>
<tr>
<td>
<code>feAvailable</code><br/>
<em>
bool
</em>
</td>
<td>
<p>represents the fe available or not.</p>
</td>
</tr>
<tr>
<td>
<code>ccCount</code><br/>
<em>
int32
</em>
</td>
<td>
<p>the number of compute cluster.</p>
</td>
</tr>
<tr>
<td>
<code>ccAvailableCount</code><br/>
<em>
int32
</em>
</td>
<td>
<p>the available numbers of compute cluster.</p>
</td>
</tr>
<tr>
<td>
<code>ccFullAvailableCount</code><br/>
<em>
int32
</em>
</td>
<td>
<p>the full available numbers of compute cluster, represents all pod in compute cluster are ready.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.CommonSpec">CommonSpec
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.ComputeCluster">ComputeCluster</a>, <a href="#disaggregated.cluster.doris.com/v1.FeSpec">FeSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicas</code><br/>
<em>
int32
</em>
</td>
<td>
<p>Replicas represent the number of desired Pod.
fe default is 2. fe is master-slave architecture only one is master.</p>
</td>
</tr>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<p>Image is the Disaggregated docker image to deploy. please reference the selectdb repository to find.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
If specified, these secrets will be passed to individual puller implementations for them to use.
More info: <a href="https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod">https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod</a></p>
</td>
</tr>
<tr>
<td>
<code>startTimeout</code><br/>
<em>
int32
</em>
</td>
<td>
<p>pod start timeout, unit is second</p>
</td>
</tr>
<tr>
<td>
<code>ResourceRequirements</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResourceRequirements</code> are embedded into this type.)
</p>
<p>defines the specification of resource cpu and mem. ep: {&ldquo;requests&rdquo;:{&ldquo;cpu&rdquo;: 4, &ldquo;memory&rdquo;: &ldquo;8Gi&rdquo;},&ldquo;limits&rdquo;:{&ldquo;cpu&rdquo;:4,&ldquo;memory&rdquo;:&ldquo;8Gi&rdquo;}}</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Labels for organize and categorize objects</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations is an unstructured key value map stored with a resource that may be
set by external tools to store and retrieve arbitrary metadata.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Affinity is a group of affinity scheduling rules.</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolume</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.PersistentVolume">
PersistentVolume
</a>
</em>
</td>
<td>
<p>VolumeClaimTemplate allows customizing the persistent volume claim for the pod.</p>
</td>
</tr>
<tr>
<td>
<code>noStoreLog</code><br/>
<em>
bool
</em>
</td>
<td>
<p>when set true, the log will store in disk that created by volumeClaimTemplate</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>(Optional) Tolerations for scheduling pods onto some dedicated nodes</p>
</td>
</tr>
<tr>
<td>
<code>service</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.ExportService">
ExportService
</a>
</em>
</td>
<td>
<p>export metaservice for accessing from outside k8s.</p>
</td>
</tr>
<tr>
<td>
<code>configMaps</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.ConfigMap">
[]ConfigMap
</a>
</em>
</td>
<td>
<p>ConfigMaps describe all configmap that need to be mounted.</p>
</td>
</tr>
<tr>
<td>
<code>nodeSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>specify what&rsquo;s node to deploy compute cluster pod.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccount</code><br/>
<em>
string
</em>
</td>
<td>
<p>serviceAccount for compute node access cloud service.</p>
</td>
</tr>
<tr>
<td>
<code>hostAliases</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#hostalias-v1-core">
[]Kubernetes core/v1.HostAlias
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HostAliases is an optional list of hosts and IPs that will be injected into the pod&rsquo;s hosts
file if specified. This is only valid for non-hostNetwork pods.</p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#podsecuritycontext-v1-core">
Kubernetes core/v1.PodSecurityContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Security context for pod.</p>
</td>
</tr>
<tr>
<td>
<code>containerSecurityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Security context for all containers running in the pod (unless they override it).</p>
</td>
</tr>
<tr>
<td>
<code>envVars</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<p>EnvVars is a slice of environment variables that are added to the pods, the default is empty.</p>
</td>
</tr>
<tr>
<td>
<code>systemInitialization</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.SystemInitialization">
SystemInitialization
</a>
</em>
</td>
<td>
<p>SystemInitialization for fe, be setting system parameters.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.ComputeCluster">ComputeCluster
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterSpec">DorisDisaggregatedClusterSpec</a>)
</p>
<div>
<p>ComputeCluster describe the specification that a group of compute node.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the identifier of computeCluster, name can be used specify what computeCluster to run sql. if not set, will use <code>computeCluster</code> and the index in array to set.ep: computeCluster-1.</p>
</td>
</tr>
<tr>
<td>
<code>clusterId</code><br/>
<em>
string
</em>
</td>
<td>
<p>ClusterId is the identifier of computeCluster, this will distinguish all computeCluster in meta.</p>
</td>
</tr>
<tr>
<td>
<code>CommonSpec</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.CommonSpec">
CommonSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>CommonSpec</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.ComputeClusterStatus">ComputeClusterStatus
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterStatus">DorisDisaggregatedClusterStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.Phase">
Phase
</a>
</em>
</td>
<td>
<p>Phase represent the stage of reconciling.</p>
</td>
</tr>
<tr>
<td>
<code>statefulsetName</code><br/>
<em>
string
</em>
</td>
<td>
<p>the statefulset of control this compute cluster pods.</p>
</td>
</tr>
<tr>
<td>
<code>serviceName</code><br/>
<em>
string
</em>
</td>
<td>
<p>the service that can access the compute cluster pods.</p>
</td>
</tr>
<tr>
<td>
<code>ComputeClusterName</code><br/>
<em>
string
</em>
</td>
<td>
<p>represents the compute cluster.</p>
</td>
</tr>
<tr>
<td>
<code>availableStatus</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.AvailableStatus">
AvailableStatus
</a>
</em>
</td>
<td>
<p>AvailableStatus represents the compute cluster available or not.</p>
</td>
</tr>
<tr>
<td>
<code>clusterId</code><br/>
<em>
string
</em>
</td>
<td>
<p>ClusterId display  the clusterId of compute cluster in meta.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code><br/>
<em>
int32
</em>
</td>
<td>
<p>replicas is the number of Pods created by the StatefulSet controller.</p>
</td>
</tr>
<tr>
<td>
<code>availableReplicas</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of available pods (ready for at least minReadySeconds) targeted by this statefulset.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.ConfigMap">ConfigMap
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.CommonSpec">CommonSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name specify the configmap need to be mounted in pod in deployed namespace.</p>
</td>
</tr>
<tr>
<td>
<code>mountPath</code><br/>
<em>
string
</em>
</td>
<td>
<p>display the path of configMap be mounted in pod. the component start conf please mount to /etc/doris, ep: fe-configmap contains &lsquo;fe.conf&rsquo;, mountPath must be &lsquo;/etc/doris&rsquo;.
key in configMap&rsquo;s data is file name.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.DisMS">DisMS
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterSpec">DorisDisaggregatedClusterSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace specify the namespace of metaservice deployed.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name specify the name of metaservice resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.DisaggregatedComponentType">DisaggregatedComponentType
(<code>string</code> alias)</h3>
<div>
</div>
<h3 id="disaggregated.cluster.doris.com/v1.DorisDisaggregatedCluster">DorisDisaggregatedCluster
</h3>
<div>
<p>DorisDisaggregatedCluster defined as CRD format, have type, metadata, spec, status, fields.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterSpec">
DorisDisaggregatedClusterSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>instanceConfigMap</code><br/>
<em>
string
</em>
</td>
<td>
<p>VaultConfigmap specify the configmap that have configuration of file object information. example S3.
configmap have to config, please reference the doc.</p>
</td>
</tr>
<tr>
<td>
<code>disMS</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.DisMS">
DisMS
</a>
</em>
</td>
<td>
<p>MetaService describe the metaservice that cluster want to storage metadata.</p>
</td>
</tr>
<tr>
<td>
<code>feSpec</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.FeSpec">
FeSpec
</a>
</em>
</td>
<td>
<p>FeSpec describe the fe specification of doris disaggregated cluster.</p>
</td>
</tr>
<tr>
<td>
<code>computeClusters</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.ComputeCluster">
[]ComputeCluster
</a>
</em>
</td>
<td>
<p>ComputeClusters describe a list of ComputeCluster, ComputeCluster is a group of compute node to do same thing.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterStatus">
DorisDisaggregatedClusterStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterSpec">DorisDisaggregatedClusterSpec
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedCluster">DorisDisaggregatedCluster</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>instanceConfigMap</code><br/>
<em>
string
</em>
</td>
<td>
<p>VaultConfigmap specify the configmap that have configuration of file object information. example S3.
configmap have to config, please reference the doc.</p>
</td>
</tr>
<tr>
<td>
<code>disMS</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.DisMS">
DisMS
</a>
</em>
</td>
<td>
<p>MetaService describe the metaservice that cluster want to storage metadata.</p>
</td>
</tr>
<tr>
<td>
<code>feSpec</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.FeSpec">
FeSpec
</a>
</em>
</td>
<td>
<p>FeSpec describe the fe specification of doris disaggregated cluster.</p>
</td>
</tr>
<tr>
<td>
<code>computeClusters</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.ComputeCluster">
[]ComputeCluster
</a>
</em>
</td>
<td>
<p>ComputeClusters describe a list of ComputeCluster, ComputeCluster is a group of compute node to do same thing.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterStatus">DorisDisaggregatedClusterStatus
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedCluster">DorisDisaggregatedCluster</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>instanceId</code><br/>
<em>
string
</em>
</td>
<td>
<p>ClusterId display  the clusterId of DorisDisaggregatedCluster in meta.</p>
</td>
</tr>
<tr>
<td>
<code>msEndpoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>the ms address for store meta of disaggregated cluster.</p>
</td>
</tr>
<tr>
<td>
<code>msToken</code><br/>
<em>
string
</em>
</td>
<td>
<p>the token for access ms service.</p>
</td>
</tr>
<tr>
<td>
<code>feStatus</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.FEStatus">
FEStatus
</a>
</em>
</td>
<td>
<p>FEStatus describe the fe status.</p>
</td>
</tr>
<tr>
<td>
<code>clusterHealth</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.ClusterHealth">
ClusterHealth
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>computeClusterStatuses</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.ComputeClusterStatus">
[]ComputeClusterStatus
</a>
</em>
</td>
<td>
<p>ComputeClusterStatuses reflect a list of computeCluster status.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.ExportService">ExportService
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.CommonSpec">CommonSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#servicetype-v1-core">
Kubernetes core/v1.ServiceType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>type of service,the possible value for the service type are : ClusterIP, NodePort, LoadBalancer,ExternalName.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types">https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types</a></p>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations for using function on different cloud platform.</p>
</td>
</tr>
<tr>
<td>
<code>portMaps</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.PortMap">
[]PortMap
</a>
</em>
</td>
<td>
<p>PortMaps specify node port for target port in pod, when the service type=NodePort.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.FEStatus">FEStatus
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterStatus">DorisDisaggregatedClusterStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.Phase">
Phase
</a>
</em>
</td>
<td>
<p>Phase represent the stage of reconciling.</p>
</td>
</tr>
<tr>
<td>
<code>availableStatus</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.AvailableStatus">
AvailableStatus
</a>
</em>
</td>
<td>
<p>AvailableStatus represents the fe available or not.</p>
</td>
</tr>
<tr>
<td>
<code>clusterId</code><br/>
<em>
string
</em>
</td>
<td>
<p>ClusterId display  the clusterId of fe in meta.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.FeSpec">FeSpec
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.DorisDisaggregatedClusterSpec">DorisDisaggregatedClusterSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>CommonSpec</code><br/>
<em>
<a href="#disaggregated.cluster.doris.com/v1.CommonSpec">
CommonSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>CommonSpec</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.Health">Health
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.ClusterHealth">ClusterHealth</a>)
</p>
<div>
</div>
<h3 id="disaggregated.cluster.doris.com/v1.PersistentVolume">PersistentVolume
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.CommonSpec">CommonSpec</a>)
</p>
<div>
<p>PersistentVolume defines volume information and container mount information.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>persistentVolumeClaimSpec</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaimspec-v1-core">
Kubernetes core/v1.PersistentVolumeClaimSpec
</a>
</em>
</td>
<td>
<p>PersistentVolumeClaimSpec is a list of claim spec about storage that pods are required.</p>
</td>
</tr>
<tr>
<td>
<code>mountPaths</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>specify mountPaths, if not config, operator will refer from be.conf <code>cache_file_path</code>.
when mountPaths=[]{&ldquo;/opt/path1&rdquo;, &ldquo;/opt/path2&rdquo;}, will create two pvc mount the two paths. also, operator will mount the cache_file_path config in be.conf .
if mountPaths have duplicated path in cache_file_path, operator will only create one pvc.</p>
</td>
</tr>
<tr>
<td>
<code>logNotStore</code><br/>
<em>
bool
</em>
</td>
<td>
<p>if config true, the log will mount a pvc to store logs. the pvc size is definitely 200Gi, as the log recycling system will regular recycling.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotation for PVC pods. Users can adapt the storage authentication and pv binding of the cloud platform through configuration.
It only takes effect in the first configuration and cannot be added or modified later.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.Phase">Phase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.ComputeClusterStatus">ComputeClusterStatus</a>, <a href="#disaggregated.cluster.doris.com/v1.FEStatus">FEStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>Failed represents service failed to start, can&rsquo;t be accessed.</p>
</td>
</tr><tr><td><p>&#34;Ready&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Reconciling&#34;</p></td>
<td><p>Creating represents service in creating stage.</p>
</td>
</tr><tr><td><p>&#34;Upgrading&#34;</p></td>
<td><p>Upgrading represents the spec of the service changed, service in smoothing upgrade.</p>
</td>
</tr></tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.PortMap">PortMap
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.ExportService">ExportService</a>)
</p>
<div>
<p>PortMap for ServiceType=NodePort situation.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>nodePort</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The port on each node on which this service is exposed when type is
NodePort or LoadBalancer.  Usually assigned by the system. If a value is
specified, in-range, and not in use it will be used, otherwise the
operation will fail.  If not specified, a port will be allocated if this
Service requires one.  If this field is specified when creating a
Service which does not need it, creation will fail. This field will be
wiped when updating a Service to no longer need it (e.g. changing type
from NodePort to ClusterIP).
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport">https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport</a>
need in 30000-32767</p>
</td>
</tr>
<tr>
<td>
<code>targetPort</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Number or name of the port to access on the pods targeted by the service.
Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.
If this is a string, it will be looked up as a named port in the
target Pod&rsquo;s container ports. If this is not specified, the value
of the &lsquo;port&rsquo; field is used (an identity map).
This field is ignored for services with clusterIP=None, and should be
omitted or set equal to the &lsquo;port&rsquo; field.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service">https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.Secret">Secret
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>specify the secret need to be mounted in deployed namespace.</p>
</td>
</tr>
<tr>
<td>
<code>mountPath</code><br/>
<em>
string
</em>
</td>
<td>
<p>display the path of secret be mounted in pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.cluster.doris.com/v1.SystemInitialization">SystemInitialization
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.cluster.doris.com/v1.CommonSpec">CommonSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>initImage</code><br/>
<em>
string
</em>
</td>
<td>
<p>Image for doris initialization, default is selectdb/alpine:latest.</p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Entrypoint array. Not executed within a shell.</p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Arguments to the entrypoint.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>7d8fdc8</code>.
</em></p>
