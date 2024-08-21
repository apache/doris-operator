<p>Packages:</p>
<ul>
<li>
<a href="#disaggregated.metaservice.doris.com%2fv1">disaggregated.metaservice.doris.com/v1</a>
</li>
</ul>
<h2 id="disaggregated.metaservice.doris.com/v1">disaggregated.metaservice.doris.com/v1</h2>
<div>
<p>Package v1 is the v1 version of the API.</p>
</div>
Resource Types:
<ul><li>
<a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaService">DorisDisaggregatedMetaService</a>
</li></ul>
<h3 id="disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaService">DorisDisaggregatedMetaService
</h3>
<div>
<p>DorisDisaggregatedMetaService is the Schema for the DorisDisaggregatedMetaServices API</p>
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
<code>apiVersion</code><br/>
string</td>
<td>
<code>
disaggregated.metaservice.doris.com/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>DorisDisaggregatedMetaService</code></td>
</tr>
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
<a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceSpec">
DorisDisaggregatedMetaServiceSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>fdb</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.FoundationDB">
FoundationDB
</a>
</em>
</td>
<td>
<p>describe the fdb spec, most configurations are already build-in.</p>
</td>
</tr>
<tr>
<td>
<code>ms</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.MetaService">
MetaService
</a>
</em>
</td>
<td>
<p>the specification of metaservice, metaservice is the component of doris disaggregated cluster.</p>
</td>
</tr>
<tr>
<td>
<code>recycler</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.Recycler">
Recycler
</a>
</em>
</td>
<td>
<p>the specification of recycler, recycler is the component of doris disaggregated cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceStatus">
DorisDisaggregatedMetaServiceStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.AvailableStatus">AvailableStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.BaseStatus">BaseStatus</a>, <a href="#disaggregated.metaservice.doris.com/v1.FDBStatus">FDBStatus</a>)
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
<h3 id="disaggregated.metaservice.doris.com/v1.BaseSpec">BaseSpec
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.MetaService">MetaService</a>, <a href="#disaggregated.metaservice.doris.com/v1.Recycler">Recycler</a>)
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
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<p>Image is the metaservice docker image to deploy. the image can pull from dockerhub selectdb repository.</p>
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
<code>service</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.ExportService">
ExportService
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>export metaservice for accessing from outside k8s.</p>
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
<code>configMaps</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.ConfigMap">
[]ConfigMap
</a>
</em>
</td>
<td>
<p>ConfigMaps describe all configmaps that need to be mounted.</p>
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
<p>(Optional) If specified, the pod&rsquo;s nodeSelector，displayName=&ldquo;Map of nodeSelectors to match when scheduling pods on nodes&rdquo;</p>
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
<em>(Optional)</em>
<p>EnvVars is a slice of environment variables that are added to the pods, the default is empty.</p>
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
<em>(Optional)</em>
<p>Labels for user selector or classify pods</p>
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
<code>persistentVolume</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.PersistentVolume">
PersistentVolume
</a>
</em>
</td>
<td>
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
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.BaseStatus">BaseStatus
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceStatus">DorisDisaggregatedMetaServiceStatus</a>)
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
<a href="#disaggregated.metaservice.doris.com/v1.MetaServicePhase">
MetaServicePhase
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
<a href="#disaggregated.metaservice.doris.com/v1.AvailableStatus">
AvailableStatus
</a>
</em>
</td>
<td>
<p>AvailableStatus represents the metaservice available or not.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.ComponentType">ComponentType
(<code>string</code> alias)</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;fdb&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;metaservice&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;recycler&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.ConfigMap">ConfigMap
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.BaseSpec">BaseSpec</a>)
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
<p>Name specify the configmap in deployed namespace that need to be mounted in pod.</p>
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
<p>MountPath specify the position of configmap be mounted.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceSpec">DorisDisaggregatedMetaServiceSpec
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaService">DorisDisaggregatedMetaService</a>)
</p>
<div>
<p>describe the meta specification of disaggregated cluster</p>
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
<code>fdb</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.FoundationDB">
FoundationDB
</a>
</em>
</td>
<td>
<p>describe the fdb spec, most configurations are already build-in.</p>
</td>
</tr>
<tr>
<td>
<code>ms</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.MetaService">
MetaService
</a>
</em>
</td>
<td>
<p>the specification of metaservice, metaservice is the component of doris disaggregated cluster.</p>
</td>
</tr>
<tr>
<td>
<code>recycler</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.Recycler">
Recycler
</a>
</em>
</td>
<td>
<p>the specification of recycler, recycler is the component of doris disaggregated cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceStatus">DorisDisaggregatedMetaServiceStatus
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaService">DorisDisaggregatedMetaService</a>)
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
<code>fdbStatus</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.FDBStatus">
FDBStatus
</a>
</em>
</td>
<td>
<p>describe the fdb status information: contains access address, fdb available or not, etc&hellip;</p>
</td>
</tr>
<tr>
<td>
<code>msStatus</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.BaseStatus">
BaseStatus
</a>
</em>
</td>
<td>
<p>describe the ms status.</p>
</td>
</tr>
<tr>
<td>
<code>recyclerStatus</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.BaseStatus">
BaseStatus
</a>
</em>
</td>
<td>
<p>descirbe the recycler status.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.ExportService">ExportService
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.BaseSpec">BaseSpec</a>)
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
<code>portMaps</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.PortMap">
[]PortMap
</a>
</em>
</td>
<td>
<p>PortMaps specify node port for target port in pod, when the service type=NodePort.</p>
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
<code>loadBalancerIP</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Only applies to Service Type: LoadBalancer.
This feature depends on whether the underlying cloud-provider supports specifying
the loadBalancerIP when a load balancer is created.
This field will be ignored if the cloud-provider does not support the feature.
This field was under-specified and its meaning varies across implementations,
and it cannot support dual-stack.
As of Kubernetes v1.24, users are encouraged to use implementation-specific annotations when available.
This field may be removed in a future API version.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.FDBStatus">FDBStatus
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceStatus">DorisDisaggregatedMetaServiceStatus</a>)
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
<code>FDBAddress</code><br/>
<em>
string
</em>
</td>
<td>
<p>FDBAddress describe the address for fdbclient using.</p>
</td>
</tr>
<tr>
<td>
<code>fdbResourceName</code><br/>
<em>
string
</em>
</td>
<td>
<p>FDBResourceName specify the name of the kind <code>FoundationDBCluster</code> resource.</p>
</td>
</tr>
<tr>
<td>
<code>availableStatus</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.AvailableStatus">
AvailableStatus
</a>
</em>
</td>
<td>
<p>AvailableStatus represents the fdb available or not.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.FoundationDB">FoundationDB
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceSpec">DorisDisaggregatedMetaServiceSpec</a>)
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
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<p>Image is the fdb docker image to deploy. please reference the selectdb repository to find.
usually no need config, operator will use default image.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarImage</code><br/>
<em>
string
</em>
</td>
<td>
<p>SidecarImage is the fdb sidecar image to deploy. pelease reference the selectdb repository to find.</p>
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
<p>defines the specification of resource cpu and mem. ep: {&ldquo;requests&rdquo;:{&ldquo;cpu&rdquo;: 4, &ldquo;memory&rdquo;: &ldquo;8Gi&rdquo;},&ldquo;limits&rdquo;:{&ldquo;cpu&rdquo;:4,&ldquo;memory&rdquo;:&ldquo;8Gi&rdquo;}}
usually not need config, operator will set default {&ldquo;requests&rdquo;: {&ldquo;cpu&rdquo;: 4, &ldquo;memory&rdquo;: &ldquo;8Gi&rdquo;}, &ldquo;limits&rdquo;: {&ldquo;cpu&rdquo;: 4, &ldquo;memory&rdquo;: &ldquo;8Gi&rdquo;}}</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplate</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaim-v1-core">
Kubernetes core/v1.PersistentVolumeClaim
</a>
</em>
</td>
<td>
<p>VolumeClaimTemplate allows customizing the persistent volume claim for the pod.</p>
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
<code>nodeSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>(Optional) If specified, the pod&rsquo;s nodeSelector，displayName=&ldquo;Map of nodeSelectors to match when scheduling pods on nodes&rdquo;</p>
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
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.MetaService">MetaService
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceSpec">DorisDisaggregatedMetaServiceSpec</a>)
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
<code>BaseSpec</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.BaseSpec">
BaseSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>BaseSpec</code> are embedded into this type.)
</p>
<p>the foundation spec for creating cn software services.
BaseSpec <code>json:&quot;baseSpec,omitempty&quot;</code></p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.MetaServicePhase">MetaServicePhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.BaseStatus">BaseStatus</a>)
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
<tbody><tr><td><p>&#34;Creating&#34;</p></td>
<td><p>Creating represents service in creating stage.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>Failed represents service failed to start, can&rsquo;t be accessed.</p>
</td>
</tr><tr><td><p>&#34;Ready&#34;</p></td>
<td><p>Ready represents the service is ready for accepting requests.</p>
</td>
</tr><tr><td><p>&#34;Upgrading&#34;</p></td>
<td><p>Upgrading represents the spec of the service changed, service in smoothing upgrade.</p>
</td>
</tr></tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.PersistentVolume">PersistentVolume
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.BaseSpec">BaseSpec</a>)
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
<h3 id="disaggregated.metaservice.doris.com/v1.PortMap">PortMap
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.ExportService">ExportService</a>)
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
<h3 id="disaggregated.metaservice.doris.com/v1.Recycler">Recycler
</h3>
<p>
(<em>Appears on:</em><a href="#disaggregated.metaservice.doris.com/v1.DorisDisaggregatedMetaServiceSpec">DorisDisaggregatedMetaServiceSpec</a>)
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
<code>BaseSpec</code><br/>
<em>
<a href="#disaggregated.metaservice.doris.com/v1.BaseSpec">
BaseSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>BaseSpec</code> are embedded into this type.)
</p>
<p>the foundation spec for creating cn software services.
BaseSpec <code>json:&quot;baseSpec,omitempty&quot;</code></p>
</td>
</tr>
</tbody>
</table>
<h3 id="disaggregated.metaservice.doris.com/v1.ServiceRole">ServiceRole
(<code>string</code> alias)</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;access&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>7d8fdc8</code>.
</em></p>
