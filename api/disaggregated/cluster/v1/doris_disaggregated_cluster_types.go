package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DorisDisaggregatedClusterSpec struct {
	//ClusterId is the identifier of Doris Disaggregated cluster, default is namespace_name.
	ClusterId string `json:"clusterId,omitempty"`
	//TODO: give the example config.
	//VaultConfigmap specify the configmap that have configuration of file object information. example S3.
	//configmap have to config, please reference the doc.
	VaultConfigmap string `json:"vaultConfigmap,omitempty"`

	//the user id, default = cluster name.
	UserId string `json:"userId,omitempty"`

	//MetaService describe the metaservice that cluster want to storage metadata.
	MetaService MetaService `json:"metaService,omitempty"`

	//FeSpec describe the fe specification of doris disaggregated cluster.
	FeSpec FeSpec `json:"feSpec,omitempty"`

	//ComputeGroups describe a list of computeGroup, computeGroup is a group of compute node to do same thing.
	ComputeGroups []ComputeGroup `json:"computeGroups,omitempty"`
}

type MetaService struct {
	//Namespace specify the namespace of metaservice deployed.
	Namespace string `json:"namespace,omitempty"`
	//Name specify the name of metaservice resource.
	Name string `json:"name,omitempty"`
	//MsPort specify the port of ms listen.
	MsPort int32 `json:"msPort,omitempty"`
}

type FeSpec struct {
	//Image is the fe of Disaggregated docker image to deploy. please reference the selectdb repository to find.
	Image string `json:"image,omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	//Replicas represent the number of fe. default is 2. fe is master-slave architecture only one is master.
	Replicas *int32 `json:"replicas,omitempty"`

	//defines the specification of resource cpu and mem. ep: {"requests":{"cpu": 4, "memory": "8Gi"},"limits":{"cpu":4,"memory":"8Gi"}}
	// usually not need config, operator will set default {"requests": {"cpu": 4, "memory": "8Gi"}, "limits": {"cpu": 4, "memory": "8Gi"}}
	corev1.ResourceRequirements `json:",inline"`

	//Labels for organize and categorize objects
	Labels map[string]string `json:"labels,omitempty"`

	//Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata.
	Annotations map[string]string `json:"annotations,omitempty"`

	// VolumeClaimTemplate allows customizing the persistent volume claim for the pod.
	VolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`

	//+optional
	// Affinity is a group of affinity scheduling rules.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// (Optional) Tolerations for scheduling pods onto some dedicated nodes
	//+optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// export metaservice for accessing from outside k8s.
	Service *ExportService `json:"service,omitempty"`

	// ConfigMaps describe all configmaps that need to be mounted.
	ConfigMaps []ConfigMap `json:"configMaps,omitempty"`
}

// ComputeGroup describe the specification that a group of compute node.
type ComputeGroup struct {
	//Name is the identifier of computeGroup, name can be used specify what computegroup to run sql. if not set, will use `computegroup` and the index in array to set.ep: computegroup-1.
	Name string `json:"name,omitempty"`

	//ClusterId is the identifier of computeGroup, this will distinguish all computeGroup in meta.
	ClusterId string `json:"clusterId,omitempty"`

	//CloudUniqueId represents the cloud code, if deployed in cloud platform. default cloudUniqueId=clusterId.
	CloudUniqueId string `json:"cloudUniqueId,omitempty"`

	//Image is the be of Disaggregated docker image to deploy. please reference the selectdb repository to find.
	Image string `json:"image,omitempty"`
	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	//Replicas represent the number of compute node.
	Replicas *int32 `json:"replicas,omitempty"`

	//defines the specification of resource cpu and mem. ep: {"requests":{"cpu": 4, "memory": "8Gi"},"limits":{"cpu":4,"memory":"8Gi"}}
	corev1.ResourceRequirements `json:",inline"`

	//Labels for organize and categorize objects
	Labels map[string]string `json:"labels,omitempty"`

	//Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata.
	Annotations map[string]string `json:"annotations,omitempty"`

	//+optional
	// Affinity is a group of affinity scheduling rules.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// (Optional) Tolerations for scheduling pods onto some dedicated nodes
	//+optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// export metaservice for accessing from outside k8s.
	Service *ExportService `json:"service,omitempty"`

	// ConfigMaps describe all configmaps that need to be mounted.
	ConfigMaps []ConfigMap `json:"configMaps,omitempty"`
}

type ConfigMap struct {
	//Name specify the configmap in deployed namespace that need to be mounted in pod.
	Name string `json:"name,omitempty"`

	//MountPath specify the position of configmap be mounted. the component start conf please mount to /etc/doris, ep: fe-configmap contains 'fe.conf', mountPath must be '/etc/doris'.
	// key in configMap's data is file name.
	MountPath string `json:"mountPath,omitempty"`
}

type ExportService struct {
	//type of service,the possible value for the service type are : ClusterIP, NodePort, LoadBalancer,ExternalName.
	//More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	//PortMaps specify node port for target port in pod, when the service type=NodePort.
	PortMaps []PortMap `json:"portMaps,omitempty"`
}

// PortMap for ServiceType=NodePort situation.
type PortMap struct {
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

type DorisDisaggregatedClusterStatus struct {
	//ClusterId display  the clusterId of DorisDisaggregatedCluster in meta.
	ClusterId string `json:"clusterId,omitempty"`
	//CloudUniqueId display the cloud code.
	CloudUniqueId string `json:"cloudUniqueId,omitempty"`

	//FEStatus describe the fe status.
	FEStatus FEStatus `json:"feStatus,omitempty"`

	//ComputeGroupStatuses reflect a list of computecgroup status.
	ComputeGroupStatuses []ComputeGroupStatus `json:"computeGroupStatuses,omitempty"`
}

type Phase string

const (
	Ready Phase = "Ready"
	//Upgrading represents the spec of the service changed, service in smoothing upgrade.
	Upgrading Phase = "Upgrading"
	//Failed represents service failed to start, can't be accessed.
	Failed Phase = "Failed"
	//Creating represents service in creating stage.
	Creating Phase = "Creating"
)

type AvailableStatus string

const (
	//Available represents the service is available.
	Available AvailableStatus = "Available"

	//UnAvailable represents the service not available for using.
	UnAvailable AvailableStatus = "UnAvailable"
)

type ComputeGroupStatus struct {
	//Phase represent the stage of reconciling.
	Phase Phase `json:"phase,omitempty"`
	//AvailableStatus represents the compute group available or not.
	AvailableStatus AvailableStatus `json:"availableStatus,omitempty"`
	//ClusterId display  the clusterId of compute group in meta.
	ClusterId string `json:"clusterId,omitempty"`
	//CloudUniqueId display the cloud code.
	CloudUniqueId string `json:"cloudUniqueId,omitempty"`
}

type FEStatus struct {
	//Phase represent the stage of reconciling.
	Phase Phase `json:"phase,omitempty"`
	//AvailableStatus represents the fe available or not.
	AvailableStatus AvailableStatus `json:"availableStatus,omitempty"`
	//ClusterId display  the clusterId of fe in meta.
	ClusterId string `json:"clusterId,omitempty"`
	//CloudUniqueId display the cloud code.
	CloudUniqueId string `json:"cloudUniqueId,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// DorisDisaggregatedCluster defined as CRD format, have type, metadata, spec, status, fields.
type DorisDisaggregatedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DorisDisaggregatedClusterSpec   `json:"spec,omitempty"`
	Status            DorisDisaggregatedClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// DorisDisaggregatedClusterList contains a list of DorisDisaggregatedCluster
type DorisDisaggregatedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DorisDisaggregatedCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DorisDisaggregatedCluster{}, &DorisDisaggregatedClusterList{})
}
