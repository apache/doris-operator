<p>Packages:</p>
<ul>
<li>
<a href="#doris.selectdb.com%2fv1">doris.selectdb.com/v1</a>
</li>
</ul>
<h2 id="doris.selectdb.com/v1">doris.selectdb.com/v1</h2>
<div>
<p>Package v1 is the v1 version of the API.</p>
</div>
Resource Types:
<ul><li>
<a href="#doris.selectdb.com/v1.DorisCluster">DorisCluster</a>
</li></ul>
<h3 id="doris.selectdb.com/v1.DorisCluster">DorisCluster
</h3>
<div>
<p>DorisCluster is the Schema for the dorisclusters API</p>
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
doris.selectdb.com/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>DorisCluster</code></td>
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
<a href="#doris.selectdb.com/v1.DorisClusterSpec">
DorisClusterSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>feSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.FeSpec">
FeSpec
</a>
</em>
</td>
<td>
<p>defines the fe cluster state that will be created by operator.</p>
</td>
</tr>
<tr>
<td>
<code>beSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.BeSpec">
BeSpec
</a>
</em>
</td>
<td>
<p>defines the be cluster state pod that will be created by operator.</p>
</td>
</tr>
<tr>
<td>
<code>cnSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.CnSpec">
CnSpec
</a>
</em>
</td>
<td>
<p>defines the cn cluster state that will be created by operator.</p>
</td>
</tr>
<tr>
<td>
<code>brokerSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.BrokerSpec">
BrokerSpec
</a>
</em>
</td>
<td>
<p>defines the broker state that will be created by operator.</p>
</td>
</tr>
<tr>
<td>
<code>adminUser</code><br/>
<em>
<a href="#doris.selectdb.com/v1.AdminUser">
AdminUser
</a>
</em>
</td>
<td>
<p>administrator for register or drop component from fe cluster. adminUser for all component register and operator drop component.</p>
</td>
</tr>
<tr>
<td>
<code>authSecret</code><br/>
<em>
string
</em>
</td>
<td>
<p>the name of secret that type is <code>kubernetes.io/basic-auth</code> and contains keys username, password for management doris node in cluster as fe, be register.
the password key is <code>password</code>. the username defaults to <code>root</code> and is omitempty.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#doris.selectdb.com/v1.DorisClusterStatus">
DorisClusterStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.AdminUser">AdminUser
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.DorisClusterSpec">DorisClusterSpec</a>)
</p>
<div>
<p>AdminUser describe administrator for manage components in specified cluster.</p>
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
<p>the user name for admin service&rsquo;s node.</p>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
string
</em>
</td>
<td>
<p>password, login to doris db.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.AutoScalerVersion">AutoScalerVersion
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.AutoScalingPolicy">AutoScalingPolicy</a>, <a href="#doris.selectdb.com/v1.HorizontalScaler">HorizontalScaler</a>)
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
<tbody><tr><td><p>&#34;v1&#34;</p></td>
<td><p>the cn service use v1 autoscaler. reference to <a href="https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/">https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/</a></p>
</td>
</tr><tr><td><p>&#34;v2&#34;</p></td>
<td><p>the cn service use v2. reference to  <a href="https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/">https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/</a></p>
</td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.AutoScalingPolicy">AutoScalingPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.CnSpec">CnSpec</a>)
</p>
<div>
<p>AutoScalingPolicy defines the auto scale</p>
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
<code>hpaPolicy</code><br/>
<em>
<a href="#doris.selectdb.com/v1.HPAPolicy">
HPAPolicy
</a>
</em>
</td>
<td>
<p>the policy of cn autoscale. operator use autoscaling v2.</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
<a href="#doris.selectdb.com/v1.AutoScalerVersion">
AutoScalerVersion
</a>
</em>
</td>
<td>
<p>version represents the autoscaler version for cn service. only support v1,,v2</p>
</td>
</tr>
<tr>
<td>
<code>minReplicas</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>the min numbers of target.</p>
</td>
</tr>
<tr>
<td>
<code>maxReplicas</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>the max numbers of target.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.BaseSpec">BaseSpec
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.BeSpec">BeSpec</a>, <a href="#doris.selectdb.com/v1.BrokerSpec">BrokerSpec</a>, <a href="#doris.selectdb.com/v1.CnSpec">CnSpec</a>, <a href="#doris.selectdb.com/v1.FeSpec">FeSpec</a>)
</p>
<div>
<p>BaseSpec describe the foundation spec of pod about doris components.</p>
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
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>annotation for fe pods. user can config monitor annotation for collect to monitor system.</p>
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
<p>serviceAccount for cn access cloud service.</p>
</td>
</tr>
<tr>
<td>
<code>service</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ExportService">
ExportService
</a>
</em>
</td>
<td>
<p>expose doris components for accessing.
example: if you want to use <code>stream load</code> to load data into doris out k8s, you can use be service and config different service type for loading data.</p>
</td>
</tr>
<tr>
<td>
<code>feAddress</code><br/>
<em>
<a href="#doris.selectdb.com/v1.FeAddress">
FeAddress
</a>
</em>
</td>
<td>
<p>specify register fe addresses</p>
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
<em>(Optional)</em>
<p>Replicas is the number of desired cn Pod.</p>
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
<p>Image for a doris cn deployment.</p>
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
<code>configMapInfo</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ConfigMapInfo">
ConfigMapInfo
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the reference for cn configMap.</p>
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
<p>defines the specification of resource cpu and mem.</p>
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
<p>cnEnvVars is a slice of environment variables that are added to the pods, the default is empty.</p>
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
<p>If specified, the pod&rsquo;s scheduling constraints.</p>
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
<code>podLabels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>podLabels for user selector or classify pods</p>
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
<code>persistentVolumes</code><br/>
<em>
<a href="#doris.selectdb.com/v1.PersistentVolume">
[]PersistentVolume
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>systemInitialization</code><br/>
<em>
<a href="#doris.selectdb.com/v1.SystemInitialization">
SystemInitialization
</a>
</em>
</td>
<td>
<p>SystemInitialization for fe, be and cn setting system parameters.</p>
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
<p>Pod security context for cn pod.</p>
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
<p>Container security context for cn container.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.BeSpec">BeSpec
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.DorisClusterSpec">DorisClusterSpec</a>)
</p>
<div>
<p>BeSpec describes a template for creating copies of a be software service.</p>
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
<a href="#doris.selectdb.com/v1.BaseSpec">
BaseSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>BaseSpec</code> are embedded into this type.)
</p>
<p>the foundation spec for creating be software services.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.BrokerSpec">BrokerSpec
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.DorisClusterSpec">DorisClusterSpec</a>)
</p>
<div>
<p>BrokerSpec describes a template for creating copies of a broker software service, if deploy broker we recommend you add affinity for deploy with be pod.</p>
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
<a href="#doris.selectdb.com/v1.BaseSpec">
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
<tr>
<td>
<code>kickOffAffinityBe</code><br/>
<em>
bool
</em>
</td>
<td>
<p>enable affinity with be , if kickoff affinity, the operator will set affinity on broker with be.
The affinity is preferred not required.
When the user custom affinity the switch does not take effect anymore.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.CnSpec">CnSpec
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.DorisClusterSpec">DorisClusterSpec</a>)
</p>
<div>
<p>CnSpec describes a template for creating copies of a cn software service. cn, the service for external table.</p>
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
<a href="#doris.selectdb.com/v1.BaseSpec">
BaseSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>BaseSpec</code> are embedded into this type.)
</p>
<p>the foundation spec for creating cn software services.</p>
</td>
</tr>
<tr>
<td>
<code>autoScalingPolicy</code><br/>
<em>
<a href="#doris.selectdb.com/v1.AutoScalingPolicy">
AutoScalingPolicy
</a>
</em>
</td>
<td>
<p>AutoScalingPolicy auto scaling strategy</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.CnStatus">CnStatus
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.DorisClusterStatus">DorisClusterStatus</a>)
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
<code>ComponentStatus</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ComponentStatus">
ComponentStatus
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentStatus</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>horizontalScaler</code><br/>
<em>
<a href="#doris.selectdb.com/v1.HorizontalScaler">
HorizontalScaler
</a>
</em>
</td>
<td>
<p>HorizontalAutoscaler have the autoscaler information.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.ComponentCondition">ComponentCondition
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.ComponentStatus">ComponentStatus</a>)
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
<code>subResourceName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ComponentPhase">
ComponentPhase
</a>
</em>
</td>
<td>
<p>Phase of statefulset condition.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>The last time this condition was updated.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
<p>The reason for the condition&rsquo;s last transition.</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<p>A human readable message indicating details about the transition.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.ComponentPhase">ComponentPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.ComponentCondition">ComponentCondition</a>)
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
<tbody><tr><td><p>&#34;available&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;haveMemberFailed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;reconciling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;waitScheduling&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.ComponentStatus">ComponentStatus
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.CnStatus">CnStatus</a>, <a href="#doris.selectdb.com/v1.DorisClusterStatus">DorisClusterStatus</a>)
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
<code>accessService</code><br/>
<em>
string
</em>
</td>
<td>
<p>DorisComponentStatus represents the status of a doris component.
the name of fe service exposed for user.</p>
</td>
</tr>
<tr>
<td>
<code>failedInstances</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>FailedInstances failed pod names.</p>
</td>
</tr>
<tr>
<td>
<code>creatingInstances</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>CreatingInstances in creating pod names.</p>
</td>
</tr>
<tr>
<td>
<code>runningInstances</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>RunningInstances in running status pod names.</p>
</td>
</tr>
<tr>
<td>
<code>componentCondition</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ComponentCondition">
ComponentCondition
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.ComponentType">ComponentType
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
<tbody><tr><td><p>&#34;be&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;broker&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;cn&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;fe&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.ConfigMapInfo">ConfigMapInfo
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.BaseSpec">BaseSpec</a>)
</p>
<div>
<p>ConfigMapInfo specify configmap to mount for component.</p>
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
<code>configMapName</code><br/>
<em>
string
</em>
</td>
<td>
<p>the config info for start progress.</p>
</td>
</tr>
<tr>
<td>
<code>resolveKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>represents the key of configMap. for doris it refers to the config file name for start doris component.
example: if deploy fe, the resolveKey = fe.conf, if deploy be  resolveKey = be.conf, etc.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.ContainerResourceMetricSource">ContainerResourceMetricSource
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.MetricSpec">MetricSpec</a>)
</p>
<div>
<p>ContainerResourceMetricSource indicates how to scale on a resource metric known to
Kubernetes, as specified in requests and limits, describing each pod in the
current scale target (e.g. CPU or memory).  The values will be averaged
together before being compared to the target.  Such metrics are built in to
Kubernetes, and have special scaling options on top of those available to
normal per-pod metrics using the &ldquo;pods&rdquo; source.  Only one &ldquo;target&rdquo; type
should be set.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcename-v1-core">
Kubernetes core/v1.ResourceName
</a>
</em>
</td>
<td>
<p>name is the name of the resource in question.</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricTarget">
MetricTarget
</a>
</em>
</td>
<td>
<p>target specifies the target value for the given metric</p>
</td>
</tr>
<tr>
<td>
<code>container</code><br/>
<em>
string
</em>
</td>
<td>
<p>container is the name of the container in the pods of the scaling target</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.CrossVersionObjectReference">CrossVersionObjectReference
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.ObjectMetricSource">ObjectMetricSource</a>)
</p>
<div>
<p>CrossVersionObjectReference contains enough information to let you identify the referred resource.</p>
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
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
<p>Kind of the referent; More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds&quot;">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds&rdquo;</a></p>
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
<p>Name of the referent; More info: <a href="http://kubernetes.io/docs/user-guide/identifiers#names">http://kubernetes.io/docs/user-guide/identifiers#names</a></p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>API version of the referent</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.DorisClusterSpec">DorisClusterSpec
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.DorisCluster">DorisCluster</a>)
</p>
<div>
<p>DorisClusterSpec defines the desired state of DorisCluster</p>
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
<code>feSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.FeSpec">
FeSpec
</a>
</em>
</td>
<td>
<p>defines the fe cluster state that will be created by operator.</p>
</td>
</tr>
<tr>
<td>
<code>beSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.BeSpec">
BeSpec
</a>
</em>
</td>
<td>
<p>defines the be cluster state pod that will be created by operator.</p>
</td>
</tr>
<tr>
<td>
<code>cnSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.CnSpec">
CnSpec
</a>
</em>
</td>
<td>
<p>defines the cn cluster state that will be created by operator.</p>
</td>
</tr>
<tr>
<td>
<code>brokerSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.BrokerSpec">
BrokerSpec
</a>
</em>
</td>
<td>
<p>defines the broker state that will be created by operator.</p>
</td>
</tr>
<tr>
<td>
<code>adminUser</code><br/>
<em>
<a href="#doris.selectdb.com/v1.AdminUser">
AdminUser
</a>
</em>
</td>
<td>
<p>administrator for register or drop component from fe cluster. adminUser for all component register and operator drop component.</p>
</td>
</tr>
<tr>
<td>
<code>authSecret</code><br/>
<em>
string
</em>
</td>
<td>
<p>the name of secret that type is <code>kubernetes.io/basic-auth</code> and contains keys username, password for management doris node in cluster as fe, be register.
the password key is <code>password</code>. the username defaults to <code>root</code> and is omitempty.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.DorisClusterStatus">DorisClusterStatus
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.DorisCluster">DorisCluster</a>)
</p>
<div>
<p>DorisClusterStatus defines the observed state of DorisCluster</p>
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
<code>feStatus</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ComponentStatus">
ComponentStatus
</a>
</em>
</td>
<td>
<p>describe fe cluster status, record running, creating and failed pods.</p>
</td>
</tr>
<tr>
<td>
<code>beStatus</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ComponentStatus">
ComponentStatus
</a>
</em>
</td>
<td>
<p>describe be cluster status, recode running, creating and failed pods.</p>
</td>
</tr>
<tr>
<td>
<code>cnStatus</code><br/>
<em>
<a href="#doris.selectdb.com/v1.CnStatus">
CnStatus
</a>
</em>
</td>
<td>
<p>describe cn cluster status, record running, creating and failed pods.</p>
</td>
</tr>
<tr>
<td>
<code>brokerStatus</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ComponentStatus">
ComponentStatus
</a>
</em>
</td>
<td>
<p>describe broker cluster status, record running, creating and failed pods.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.DorisServicePort">DorisServicePort
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.ExportService">ExportService</a>)
</p>
<div>
<p>DorisServicePort for ServiceType=NodePort situation.</p>
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
<h3 id="doris.selectdb.com/v1.Endpoints">Endpoints
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.FeAddress">FeAddress</a>)
</p>
<div>
<p>Endpoints describe the address outside k8s.</p>
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
<code>:address</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>the ip or domain array.</p>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
int
</em>
</td>
<td>
<p>the fe port that for query. the field <code>query_port</code> defines in fe config.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.ExportService">ExportService
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.BaseSpec">BaseSpec</a>)
</p>
<div>
<p>ExportService consisting of expose ports for user access to software service.</p>
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
<code>servicePorts</code><br/>
<em>
<a href="#doris.selectdb.com/v1.DorisServicePort">
[]DorisServicePort
</a>
</em>
</td>
<td>
<p>ServicePort config service for NodePort access mode.</p>
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
<h3 id="doris.selectdb.com/v1.ExternalMetricSource">ExternalMetricSource
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.MetricSpec">MetricSpec</a>)
</p>
<div>
<p>ExternalMetricSource indicates how to scale on a metric not associated with
any Kubernetes object (for example length of queue in cloud
messaging service, or QPS from loadbalancer running outside of cluster).</p>
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
<code>metric</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricIdentifier">
MetricIdentifier
</a>
</em>
</td>
<td>
<p>metric identifies the target metric by name and selector</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricTarget">
MetricTarget
</a>
</em>
</td>
<td>
<p>target specifies the target value for the given metric</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.FeAddress">FeAddress
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.BaseSpec">BaseSpec</a>)
</p>
<div>
<p>FeAddress specify the fe address, please set it when you deploy fe outside k8s or deploy components use crd except fe, if not set .</p>
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
<code>ServiceName</code><br/>
<em>
string
</em>
</td>
<td>
<p>the service name that proxy fe on k8s. the service must in same namespace with fe.</p>
</td>
</tr>
<tr>
<td>
<code>endpoints</code><br/>
<em>
<a href="#doris.selectdb.com/v1.Endpoints">
Endpoints
</a>
</em>
</td>
<td>
<p>the fe addresses if not deploy by crd, user can use k8s deploy fe observer.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.FeSpec">FeSpec
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.DorisClusterSpec">DorisClusterSpec</a>)
</p>
<div>
<p>FeSpec describes a template for creating copies of a fe software service.</p>
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
<code>electionNumber</code><br/>
<em>
int32
</em>
</td>
<td>
<p>the number of fe in election. electionNumber &lt;= replicas, left as observers. default value=3</p>
</td>
</tr>
<tr>
<td>
<code>BaseSpec</code><br/>
<em>
<a href="#doris.selectdb.com/v1.BaseSpec">
BaseSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>BaseSpec</code> are embedded into this type.)
</p>
<p>the foundation spec for creating be software services.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.HPAPolicy">HPAPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.AutoScalingPolicy">AutoScalingPolicy</a>)
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
<code>metrics</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricSpec">
[]MetricSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Metrics specifies how to scale based on a single metric
the struct copy from k8s.io/api/autoscaling/v2beta2/types.go. the redundancy code will hide the restriction about
HorizontalPodAutoscaler version and kubernetes releases matching issue.
the splice will have unsafe.Pointer convert, so be careful to edit the struct fileds.</p>
</td>
</tr>
<tr>
<td>
<code>behavior</code><br/>
<em>
<a href="#doris.selectdb.com/v1.HorizontalPodAutoscalerBehavior">
HorizontalPodAutoscalerBehavior
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HorizontalPodAutoscalerBehavior configures the scaling behavior of the target.
the struct copy from k8s.io/api/autoscaling/v2beta2/types.go. the redundancy code will hide the restriction about
HorizontalPodAutoscaler version and kubernetes releases matching issue.
the</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.HPAScalingPolicy">HPAScalingPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.HPAScalingRules">HPAScalingRules</a>)
</p>
<div>
<p>HPAScalingPolicy is a single policy which must hold true for a specified past interval.</p>
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
<a href="#doris.selectdb.com/v1.HPAScalingPolicyType">
HPAScalingPolicyType
</a>
</em>
</td>
<td>
<p>Type is used to specify the scaling policy.</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
int32
</em>
</td>
<td>
<p>Value contains the amount of change which is permitted by the policy.
It must be greater than zero</p>
</td>
</tr>
<tr>
<td>
<code>periodSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<p>PeriodSeconds specifies the window of time for which the policy should hold true.
PeriodSeconds must be greater than zero and less than or equal to 1800 (30 min).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.HPAScalingPolicyType">HPAScalingPolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.HPAScalingPolicy">HPAScalingPolicy</a>)
</p>
<div>
<p>HPAScalingPolicyType is the type of the policy which could be used while making scaling decisions.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Percent&#34;</p></td>
<td><p>PercentScalingPolicy is a policy used to specify a relative amount of change with respect to
the current number of pods.</p>
</td>
</tr><tr><td><p>&#34;Pods&#34;</p></td>
<td><p>PodsScalingPolicy is a policy used to specify a change in absolute number of pods.</p>
</td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.HPAScalingRules">HPAScalingRules
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.HorizontalPodAutoscalerBehavior">HorizontalPodAutoscalerBehavior</a>)
</p>
<div>
<p>HPAScalingRules configures the scaling behavior for one direction.
These Rules are applied after calculating DesiredReplicas from metrics for the HPA.
They can limit the scaling velocity by specifying scaling policies.
They can prevent flapping by specifying the stabilization window, so that the
number of replicas is not set instantly, instead, the safest value from the stabilization
window is chosen.</p>
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
<code>stabilizationWindowSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>StabilizationWindowSeconds is the number of seconds for which past recommendations should be
considered while scaling up or scaling down.
StabilizationWindowSeconds must be greater than or equal to zero and less than or equal to 3600 (one hour).
If not set, use the default values:
- For scale up: 0 (i.e. no stabilization is done).
- For scale down: 300 (i.e. the stabilization window is 300 seconds long).</p>
</td>
</tr>
<tr>
<td>
<code>selectPolicy</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ScalingPolicySelect">
ScalingPolicySelect
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>selectPolicy is used to specify which policy should be used.
If not set, the default value MaxPolicySelect is used.</p>
</td>
</tr>
<tr>
<td>
<code>policies</code><br/>
<em>
<a href="#doris.selectdb.com/v1.HPAScalingPolicy">
[]HPAScalingPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>policies is a list of potential scaling polices which can be used during scaling.
At least one policy must be specified, otherwise the HPAScalingRules will be discarded as invalid</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.HorizontalPodAutoscalerBehavior">HorizontalPodAutoscalerBehavior
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.HPAPolicy">HPAPolicy</a>)
</p>
<div>
<p>HorizontalPodAutoscalerBehavior configures the scaling behavior of the target
in both Up and Down directions (scaleUp and scaleDown fields respectively).</p>
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
<code>scaleUp</code><br/>
<em>
<a href="#doris.selectdb.com/v1.HPAScalingRules">
HPAScalingRules
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>scaleUp is scaling policy for scaling Up.
If not set, the default value is the higher of:
* increase no more than 4 pods per 60 seconds
* double the number of pods per 60 seconds
No stabilization is used.</p>
</td>
</tr>
<tr>
<td>
<code>scaleDown</code><br/>
<em>
<a href="#doris.selectdb.com/v1.HPAScalingRules">
HPAScalingRules
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>scaleDown is scaling policy for scaling Down.
If not set, the default value is to allow to scale down to minReplicas pods, with a
300 second stabilization window (i.e., the highest recommendation for
the last 300sec is used).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.HorizontalScaler">HorizontalScaler
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.CnStatus">CnStatus</a>)
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
<p>the deploy horizontal scaler name</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
<a href="#doris.selectdb.com/v1.AutoScalerVersion">
AutoScalerVersion
</a>
</em>
</td>
<td>
<p>the deploy horizontal version.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.MetricIdentifier">MetricIdentifier
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.ExternalMetricSource">ExternalMetricSource</a>, <a href="#doris.selectdb.com/v1.ObjectMetricSource">ObjectMetricSource</a>, <a href="#doris.selectdb.com/v1.PodsMetricSource">PodsMetricSource</a>)
</p>
<div>
<p>MetricIdentifier defines the name and optionally selector for a metric</p>
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
<p>name is the name of the given metric</p>
</td>
</tr>
<tr>
<td>
<code>selector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>selector is the string-encoded form of a standard kubernetes label selector for the given metric
When set, it is passed as an additional parameter to the metrics server for more specific metrics scoping.
When unset, just the metricName will be used to gather metrics.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.MetricSourceType">MetricSourceType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.MetricSpec">MetricSpec</a>)
</p>
<div>
<p>MetricSourceType indicates the type of metric.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;ContainerResource&#34;</p></td>
<td><p>ContainerResourceMetricSourceType is a resource metric known to Kubernetes, as
specified in requests and limits, describing a single container in each pod in the current
scale target (e.g. CPU or memory).  Such metrics are built in to
Kubernetes, and have special scaling options on top of those available
to normal per-pod metrics (the &ldquo;pods&rdquo; source).</p>
</td>
</tr><tr><td><p>&#34;External&#34;</p></td>
<td><p>ExternalMetricSourceType is a global metric that is not associated
with any Kubernetes object. It allows autoscaling based on information
coming from components running outside of cluster
(for example length of queue in cloud messaging service, or
QPS from loadbalancer running outside of cluster).</p>
</td>
</tr><tr><td><p>&#34;Object&#34;</p></td>
<td><p>ObjectMetricSourceType is a metric describing a kubernetes object
(for example, hits-per-second on an Ingress object).</p>
</td>
</tr><tr><td><p>&#34;Pods&#34;</p></td>
<td><p>PodsMetricSourceType is a metric describing each pod in the current scale
target (for example, transactions-processed-per-second).  The values
will be averaged together before being compared to the target value.</p>
</td>
</tr><tr><td><p>&#34;Resource&#34;</p></td>
<td><p>ResourceMetricSourceType is a resource metric known to Kubernetes, as
specified in requests and limits, describing each pod in the current
scale target (e.g. CPU or memory).  Such metrics are built in to
Kubernetes, and have special scaling options on top of those available
to normal per-pod metrics (the &ldquo;pods&rdquo; source).</p>
</td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.MetricSpec">MetricSpec
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.HPAPolicy">HPAPolicy</a>)
</p>
<div>
<p>MetricSpec specifies how to scale based on a single metric
(only <code>type</code> and one other matching field should be set at once).</p>
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
<a href="#doris.selectdb.com/v1.MetricSourceType">
MetricSourceType
</a>
</em>
</td>
<td>
<p>type is the type of metric source.  It should be one of &ldquo;ContainerResource&rdquo;, &ldquo;External&rdquo;,
&ldquo;Object&rdquo;, &ldquo;Pods&rdquo; or &ldquo;Resource&rdquo;, each mapping to a matching field in the object.
Note: &ldquo;ContainerResource&rdquo; type is available on when the feature-gate
HPAContainerMetrics is enabled</p>
</td>
</tr>
<tr>
<td>
<code>object</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ObjectMetricSource">
ObjectMetricSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>object refers to a metric describing a single kubernetes object
(for example, hits-per-second on an Ingress object).</p>
</td>
</tr>
<tr>
<td>
<code>pods</code><br/>
<em>
<a href="#doris.selectdb.com/v1.PodsMetricSource">
PodsMetricSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>pods refers to a metric describing each pod in the current scale target
(for example, transactions-processed-per-second).  The values will be
averaged together before being compared to the target value.</p>
</td>
</tr>
<tr>
<td>
<code>resource</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ResourceMetricSource">
ResourceMetricSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>resource refers to a resource metric (such as those specified in
requests and limits) known to Kubernetes describing each pod in the
current scale target (e.g. CPU or memory). Such metrics are built in to
Kubernetes, and have special scaling options on top of those available
to normal per-pod metrics using the &ldquo;pods&rdquo; source.</p>
</td>
</tr>
<tr>
<td>
<code>containerResource</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ContainerResourceMetricSource">
ContainerResourceMetricSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>container resource refers to a resource metric (such as those specified in
requests and limits) known to Kubernetes describing a single container in
each pod of the current scale target (e.g. CPU or memory). Such metrics are
built in to Kubernetes, and have special scaling options on top of those
available to normal per-pod metrics using the &ldquo;pods&rdquo; source.
This is an alpha feature and can be enabled by the HPAContainerMetrics feature flag.</p>
</td>
</tr>
<tr>
<td>
<code>external</code><br/>
<em>
<a href="#doris.selectdb.com/v1.ExternalMetricSource">
ExternalMetricSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>external refers to a global metric that is not associated
with any Kubernetes object. It allows autoscaling based on information
coming from components running outside of cluster
(for example length of queue in cloud messaging service, or
QPS from loadbalancer running outside of cluster).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.MetricTarget">MetricTarget
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.ContainerResourceMetricSource">ContainerResourceMetricSource</a>, <a href="#doris.selectdb.com/v1.ExternalMetricSource">ExternalMetricSource</a>, <a href="#doris.selectdb.com/v1.ObjectMetricSource">ObjectMetricSource</a>, <a href="#doris.selectdb.com/v1.PodsMetricSource">PodsMetricSource</a>, <a href="#doris.selectdb.com/v1.ResourceMetricSource">ResourceMetricSource</a>)
</p>
<div>
<p>MetricTarget defines the target value, average value, or average utilization of a specific metric</p>
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
<a href="#doris.selectdb.com/v1.MetricTargetType">
MetricTargetType
</a>
</em>
</td>
<td>
<p>type represents whether the metric type is Utilization, Value, or AverageValue</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<em>(Optional)</em>
<p>value is the target value of the metric (as a quantity).</p>
</td>
</tr>
<tr>
<td>
<code>averageValue</code><br/>
<em>
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<em>(Optional)</em>
<p>averageValue is the target value of the average of the
metric across all relevant pods (as a quantity)</p>
</td>
</tr>
<tr>
<td>
<code>averageUtilization</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>averageUtilization is the target value of the average of the
resource metric across all relevant pods, represented as a percentage of
the requested value of the resource for the pods.
Currently only valid for Resource metric source type</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.MetricTargetType">MetricTargetType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.MetricTarget">MetricTarget</a>)
</p>
<div>
<p>MetricTargetType specifies the type of metric being targeted, and should be either
&ldquo;Value&rdquo;, &ldquo;AverageValue&rdquo;, or &ldquo;Utilization&rdquo;</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;AverageValue&#34;</p></td>
<td><p>AverageValueMetricType declares a MetricTarget is an</p>
</td>
</tr><tr><td><p>&#34;Utilization&#34;</p></td>
<td><p>UtilizationMetricType declares a MetricTarget is an AverageUtilization value</p>
</td>
</tr><tr><td><p>&#34;Value&#34;</p></td>
<td><p>ValueMetricType declares a MetricTarget is a raw value</p>
</td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.ObjectMetricSource">ObjectMetricSource
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.MetricSpec">MetricSpec</a>)
</p>
<div>
<p>ObjectMetricSource indicates how to scale on a metric describing a
kubernetes object (for example, hits-per-second on an Ingress object).</p>
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
<code>describedObject</code><br/>
<em>
<a href="#doris.selectdb.com/v1.CrossVersionObjectReference">
CrossVersionObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricTarget">
MetricTarget
</a>
</em>
</td>
<td>
<p>target specifies the target value for the given metric</p>
</td>
</tr>
<tr>
<td>
<code>metric</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricIdentifier">
MetricIdentifier
</a>
</em>
</td>
<td>
<p>metric identifies the target metric by name and selector</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.PVCProvisioner">PVCProvisioner
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.PersistentVolume">PersistentVolume</a>)
</p>
<div>
<p>PVCProvisioner defines PVC provisioner</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Operator&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;StatefulSet&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.PersistentVolume">PersistentVolume
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.BaseSpec">BaseSpec</a>)
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
<code>mountPath</code><br/>
<em>
string
</em>
</td>
<td>
<p>the mount path for component service.</p>
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
<p>the volume name associate with</p>
</td>
</tr>
<tr>
<td>
<code>provisioner</code><br/>
<em>
<a href="#doris.selectdb.com/v1.PVCProvisioner">
PVCProvisioner
</a>
</em>
</td>
<td>
<p>defines pvc provisioner</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.PodsMetricSource">PodsMetricSource
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.MetricSpec">MetricSpec</a>)
</p>
<div>
<p>PodsMetricSource indicates how to scale on a metric describing each pod in
the current scale target (for example, transactions-processed-per-second).
The values will be averaged together before being compared to the target
value.</p>
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
<code>metric</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricIdentifier">
MetricIdentifier
</a>
</em>
</td>
<td>
<p>metric identifies the target metric by name and selector</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricTarget">
MetricTarget
</a>
</em>
</td>
<td>
<p>target specifies the target value for the given metric</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.ResourceMetricSource">ResourceMetricSource
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.MetricSpec">MetricSpec</a>)
</p>
<div>
<p>ResourceMetricSource indicates how to scale on a resource metric known to
Kubernetes, as specified in requests and limits, describing each pod in the
current scale target (e.g. CPU or memory).  The values will be averaged
together before being compared to the target.  Such metrics are built in to
Kubernetes, and have special scaling options on top of those available to
normal per-pod metrics using the &ldquo;pods&rdquo; source.  Only one &ldquo;target&rdquo; type
should be set.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcename-v1-core">
Kubernetes core/v1.ResourceName
</a>
</em>
</td>
<td>
<p>name is the name of the resource in question.</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#doris.selectdb.com/v1.MetricTarget">
MetricTarget
</a>
</em>
</td>
<td>
<p>target specifies the target value for the given metric</p>
</td>
</tr>
</tbody>
</table>
<h3 id="doris.selectdb.com/v1.ScalingPolicySelect">ScalingPolicySelect
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.HPAScalingRules">HPAScalingRules</a>)
</p>
<div>
<p>ScalingPolicySelect is used to specify which policy should be used while scaling in a certain direction</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Disabled&#34;</p></td>
<td><p>DisabledPolicySelect disables the scaling in this direction.</p>
</td>
</tr><tr><td><p>&#34;Max&#34;</p></td>
<td><p>MaxPolicySelect selects the policy with the highest possible change.</p>
</td>
</tr><tr><td><p>&#34;Min&#34;</p></td>
<td><p>MinPolicySelect selects the policy with the lowest possible change.</p>
</td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.ServiceRole">ServiceRole
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
</tr><tr><td><p>&#34;internal&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="doris.selectdb.com/v1.SystemInitialization">SystemInitialization
</h3>
<p>
(<em>Appears on:</em><a href="#doris.selectdb.com/v1.BaseSpec">BaseSpec</a>)
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
on git commit <code>efc7eb3</code>.
</em></p>
