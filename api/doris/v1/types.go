/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	AnnotationDebugKey   = "selectdb.com.doris/runmode"
	AnnotationDebugValue = "debug"
)

// DorisClusterSpec defines the desired state of DorisCluster
type DorisClusterSpec struct {
	//defines the fe cluster state that will be created by operator.
	FeSpec *FeSpec `json:"feSpec,omitempty"`

	//defines the be cluster state pod that will be created by operator.
	BeSpec *BeSpec `json:"beSpec,omitempty"`

	//defines the cn cluster state that will be created by operator.
	CnSpec *CnSpec `json:"cnSpec,omitempty"`

	//defines the broker state that will be created by operator.
	BrokerSpec *BrokerSpec `json:"brokerSpec,omitempty"`

	//administrator for register or drop component from fe cluster. adminUser for all component register and operator drop component.
	//+Deprecated, from 1.4.1 please use secret config username and password.
	AdminUser *AdminUser `json:"adminUser,omitempty"`

	// the name of secret that type is `kubernetes.io/basic-auth` and contains keys username, password for management doris node in cluster as fe, be register.
	// the password key is `password`. the username defaults to `root` and is omitempty.
	AuthSecret string `json:"authSecret,omitempty"`
}

// AdminUser describe administrator for manage components in specified cluster.
type AdminUser struct {
	//the user name for admin service's node.
	Name string `json:"name,omitempty"`

	//password, login to doris db.
	Password string `json:"password,omitempty"`
}

// FeSpec describes a template for creating copies of a fe software service.
type FeSpec struct {
	//the number of fe in election. electionNumber <= replicas, left as observers. default value=3
	ElectionNumber *int32 `json:"electionNumber,omitempty"`

	//the foundation spec for creating be software services.
	BaseSpec `json:",inline"`
}

// BeSpec describes a template for creating copies of a be software service.
type BeSpec struct {
	//the foundation spec for creating be software services.
	BaseSpec `json:",inline"`
}

// FeAddress specify the fe address, please set it when you deploy fe outside k8s or deploy components use crd except fe, if not set .
type FeAddress struct {
	//the service name that proxy fe on k8s. the service must in same namespace with fe.
	ServiceName string `json:"ServiceName,omitempty"`

	//the fe addresses if not deploy by crd, user can use k8s deploy fe observer.
	Endpoints Endpoints `json:"endpoints,omitempty"`
}

// Endpoints describe the address outside k8s.
type Endpoints struct {
	//the ip or domain array.
	Address []string `json:":address,omitempty"`

	// the fe port that for query. the field `query_port` defines in fe config.
	Port int `json:"port,omitempty"`
}

// CnSpec describes a template for creating copies of a cn software service. cn, the service for external table.
type CnSpec struct {
	//the foundation spec for creating cn software services.
	BaseSpec `json:",inline"`

	//AutoScalingPolicy auto scaling strategy
	AutoScalingPolicy *AutoScalingPolicy `json:"autoScalingPolicy,omitempty"`
}

// BrokerSpec describes a template for creating copies of a broker software service, if deploy broker we recommend you add affinity for deploy with be pod.
type BrokerSpec struct {
	//the foundation spec for creating cn software services.
	//BaseSpec `json:"baseSpec,omitempty"`
	BaseSpec `json:",inline"`

	// enable affinity with be , if kickoff affinity, the operator will set affinity on broker with be.
	// The affinity is preferred not required.
	// When the user custom affinity the switch does not take effect anymore.
	KickOffAffinityBe bool `json:"kickOffAffinityBe,omitempty"`
}

// BaseSpec describe the foundation spec of pod about doris components.
type BaseSpec struct {
	//annotation for fe pods. user can config monitor annotation for collect to monitor system.
	Annotations map[string]string `json:"annotations,omitempty"`

	//serviceAccount for cn access cloud service.
	ServiceAccount string `json:"serviceAccount,omitempty"`

	//expose doris components for accessing.
	//example: if you want to use `stream load` to load data into doris out k8s, you can use be service and config different service type for loading data.
	Service *ExportService `json:"service,omitempty"`

	//A special supplemental group that applies to all containers in a pod.
	// Some volume types allow the Kubelet to change the ownership of that volume
	// to be owned by the pod:
	FsGroup *int64 `json:"fsGroup,omitempty"`
	// specify register fe addresses
	FeAddress *FeAddress `json:"feAddress,omitempty"`

	//Replicas is the number of desired cn Pod.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:feSpecMinimum=3
	//+optional
	Replicas *int32 `json:"replicas,omitempty"`

	//Image for a doris cn deployment.
	Image string `json:"image"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`

	//the reference for cn configMap.
	//+optional
	ConfigMapInfo ConfigMapInfo `json:"configMapInfo,omitempty"`

	//defines the specification of resource cpu and mem.
	corev1.ResourceRequirements `json:",inline"`
	// (Optional) If specified, the pod's nodeSelector，displayName="Map of nodeSelectors to match when scheduling pods on nodes"
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	//+optional
	//cnEnvVars is a slice of environment variables that are added to the pods, the default is empty.
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`

	//+optional
	//If specified, the pod's scheduling constraints.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// (Optional) Tolerations for scheduling pods onto some dedicated nodes
	//+optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	//+optional
	// podLabels for user selector or classify pods
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
	// file if specified. This is only valid for non-hostNetwork pods.
	// +optional
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`

	PersistentVolumes []PersistentVolume `json:"persistentVolumes,omitempty"`

	//SystemInitialization for fe, be and cn setting system parameters.
	SystemInitialization *SystemInitialization `json:"systemInitialization,omitempty"`

	//Pod security context for cn pod.
	//+optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	//Container security context for cn container.
	//+optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`
}

type SystemInitialization struct {
	//Image for doris initialization, default is selectdb/alpine:latest.
	InitImage string `json:"initImage,omitempty"`

	// Entrypoint array. Not executed within a shell.
	Command []string `json:"command,omitempty"`

	// Arguments to the entrypoint.
	Args []string `json:"args,omitempty"`
}

// PersistentVolume defines volume information and container mount information.
type PersistentVolume struct {
	// PersistentVolumeClaimSpec is a list of claim spec about storage that pods are required.
	// +kubebuilder:validation:Optional
	corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"`

	//the mount path for component service.
	MountPath string `json:"mountPath,omitempty"`

	//the volume name associate with
	Name string `json:"name,omitempty"`

	//defines pvc provisioner
	PVCProvisioner PVCProvisioner `json:"provisioner,omitempty"`
}

// PVCProvisioner defines PVC provisioner
type PVCProvisioner string

// Possible values of PVC provisioner
const (
	PVCProvisionerUnspecified PVCProvisioner = ""
	PVCProvisionerStatefulSet PVCProvisioner = "StatefulSet"
	PVCProvisionerOperator    PVCProvisioner = "Operator"
)

// ConfigMapInfo specify configmap to mount for component.
type ConfigMapInfo struct {

	// ConfigMapName mapped the configuration files in the doris 'conf/' directory.
	// such as 'fe.conf', 'be.conf'. If HDFS access is involved, there may also be 'core-site.xml' and other files.
	// doris-operator mounts these configuration files in the '/etc/doris' directory by default.
	// links them to the 'conf/' directory of the doris component through soft links.
	ConfigMapName string `json:"configMapName,omitempty"`

	// Deprecated: This configuration has been abandoned and will be cleared in version 1.7.0.
	// It is currently forced to be 'fe.conf', 'be.conf', 'apache_hdfs_broker.conf'
	// It is no longer effective. the configuration content will not take effect.
	// +optional
	ResolveKey string `json:"resolveKey,omitempty"`

	// ConfigMaps can mount multiple configmaps to the specified path.
	// The mounting path of configmap cannot be repeated.
	// +optional
	ConfigMaps []MountConfigMapInfo `json:"configMaps,omitempty"`
}

type MountConfigMapInfo struct {
	// name of configmap that needs to mount.
	ConfigMapName string `json:"configMapName,omitempty"`

	// Current ConfigMap Mount Path.
	// If MountConfigMapInfo belongs to the same ConfigMapInfo, their MountPath cannot be repeated.
	MountPath string `json:"mountPath,omitempty"`
}

// ExportService consisting of expose ports for user access to software service.
type ExportService struct {
	//type of service,the possible value for the service type are : ClusterIP, NodePort, LoadBalancer,ExternalName.
	//More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	//ServicePort config service for NodePort access mode.
	ServicePorts []DorisServicePort `json:"servicePorts,omitempty"`

	//Annotations for using function on different cloud platform.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Only applies to Service Type: LoadBalancer.
	// This feature depends on whether the underlying cloud-provider supports specifying
	// the loadBalancerIP when a load balancer is created.
	// This field will be ignored if the cloud-provider does not support the feature.
	// This field was under-specified and its meaning varies across implementations,
	// and it cannot support dual-stack.
	// As of Kubernetes v1.24, users are encouraged to use implementation-specific annotations when available.
	// This field may be removed in a future API version.
	// +optional
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`
}

// DorisServicePort for ServiceType=NodePort situation.
type DorisServicePort struct {
	// The port on each node on which this service is exposed when type is
	// NodePort or LoadBalancer.  Usually assigned by the system. If a value is
	// specified, in-range, and not in use it will be used, otherwise the
	// operation will fail.  If not specified, a port will be allocated if this
	// Service requires one.  If this field is specified when creating a
	// Service which does not need it, creation will fail. This field will be
	// wiped when updating a Service to no longer need it (e.g. changing type
	// from NodePort to ClusterIP).
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport
	// need in 30000-32767
	// +optional
	NodePort int32 `json:"nodePort,omitempty"`

	// Number or name of the port to access on the pods targeted by the service.
	// Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.
	// If this is a string, it will be looked up as a named port in the
	// target Pod's container ports. If this is not specified, the value
	// of the 'port' field is used (an identity map).
	// This field is ignored for services with clusterIP=None, and should be
	// omitted or set equal to the 'port' field.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service
	// +optional
	TargetPort int32 `json:"targetPort,omitempty"`
}

// DorisClusterStatus defines the observed state of DorisCluster
type DorisClusterStatus struct {
	//describe fe cluster status, record running, creating and failed pods.
	FEStatus *ComponentStatus `json:"feStatus,omitempty"`

	//describe be cluster status, recode running, creating and failed pods.
	BEStatus *ComponentStatus `json:"beStatus,omitempty"`

	//describe cn cluster status, record running, creating and failed pods.
	CnStatus *CnStatus `json:"cnStatus,omitempty"`

	//describe broker cluster status, record running, creating and failed pods.
	BrokerStatus *ComponentStatus `json:"brokerStatus,omitempty"`
}

type CnStatus struct {
	ComponentStatus `json:",inline"`
	//HorizontalAutoscaler have the autoscaler information.
	HorizontalScaler *HorizontalScaler `json:"horizontalScaler,omitempty"`
}

type HorizontalScaler struct {
	//the deploy horizontal scaler name
	Name string `json:"name,omitempty"`

	//the deploy horizontal version.
	Version AutoScalerVersion `json:"version,omitempty"`
}

type ComponentStatus struct {
	// DorisComponentStatus represents the status of a doris component.
	//the name of fe service exposed for user.
	AccessService string `json:"accessService,omitempty"`

	//FailedInstances failed pod names.
	FailedMembers []string `json:"failedInstances,omitempty"`

	//CreatingInstances in creating pod names.
	CreatingMembers []string `json:"creatingInstances,omitempty"`

	//RunningInstances in running status pod names.
	RunningMembers []string `json:"runningInstances,omitempty"`

	ComponentCondition ComponentCondition `json:"componentCondition"`
}

type ComponentCondition struct {
	SubResourceName string `json:"subResourceName,omitempty"`
	// Phase of statefulset condition.
	Phase ComponentPhase `json:"phase"`
	// The last time this condition was updated.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// The reason for the condition's last transition.
	Reason string `json:"reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message"`
}

type ComponentPhase string

const (
	Reconciling      ComponentPhase = "reconciling"
	WaitScheduling   ComponentPhase = "waitScheduling"
	HaveMemberFailed ComponentPhase = "haveMemberFailed"
	Available        ComponentPhase = "available"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=dcr
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="FeStatus",type=string,JSONPath=`.status.feStatus.componentCondition.phase`
// +kubebuilder:printcolumn:name="BeStatus",type=string,JSONPath=`.status.beStatus.componentCondition.phase`
// +kubebuilder:printcolumn:name="CnStatus",type=string,JSONPath=`.status.cnStatus.componentCondition.phase`
// +kubebuilder:printcolumn:name="BrokerStatus",type=string,JSONPath=`.status.brokerStatus.componentCondition.phase`
// +kubebuilder:storageversion
// DorisCluster is the Schema for the dorisclusters API
type DorisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DorisClusterSpec   `json:"spec,omitempty"`
	Status DorisClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// DorisClusterList contains a list of DorisCluster
type DorisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DorisCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DorisCluster{}, &DorisClusterList{})
}
