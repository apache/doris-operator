// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="FdbStatus",type=string,JSONPath=`.status.fdbStatus.availableStatus`
// +kubebuilder:printcolumn:name="MSStatus",type=string,JSONPath=`.status.msStatus.phase`
// +kubebuilder:printcolumn:name="RecyclerStatus",type=string,JSONPath=`.status.recyclerStatus.phase`
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=ddm
// DorisDisaggregatedMetaService is the Schema for the DorisDisaggregatedMetaServices API
type DorisDisaggregatedMetaService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DorisDisaggregatedMetaServiceSpec   `json:"spec,omitempty"`
	Status            DorisDisaggregatedMetaServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// DorisDisaggregatedMetaServiceList contains a list of DorisDisaggregatedMetaService
type DorisDisaggregatedMetaServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DorisDisaggregatedMetaService `json:"items"`
}

// describe the meta specification of disaggregated cluster
type DorisDisaggregatedMetaServiceSpec struct {
	//describe the fdb spec, most configurations are already build-in.
	FDB *FoundationDB `json:"fdb,omitempty"`
	//the specification of metaservice, metaservice is the component of doris disaggregated cluster.
	MS *MetaService `json:"ms,omitempty"`
	//the specification of recycler, recycler is the component of doris disaggregated cluster.
	Recycler *Recycler `json:"recycler,omitempty"`
}

type FoundationDB struct {
	//Image is the fdb docker image to deploy. please reference the selectdb repository to find.
	//usually no need config, operator will use default image.
	Image string `json:"image,omitempty"`

	//SidecarImage is the fdb sidecar image to deploy. pelease reference the selectdb repository to find.
	SidecarImage string `json:"sidecarImage,omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	//defines the specification of resource cpu and mem. ep: {"requests":{"cpu": 4, "memory": "8Gi"},"limits":{"cpu":4,"memory":"8Gi"}}
	// usually not need config, operator will set default {"requests": {"cpu": 4, "memory": "8Gi"}, "limits": {"cpu": 4, "memory": "8Gi"}}
	corev1.ResourceRequirements `json:",inline"`

	// VolumeClaimTemplate allows customizing the persistent volume claim for the pod.
	VolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`

	//Labels for organize and categorize objects
	Labels map[string]string `json:"labels,omitempty"`

	//Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata.
	Annotations map[string]string `json:"annotations,omitempty"`

	// (Optional) If specified, the pod's nodeSelector，displayName="Map of nodeSelectors to match when scheduling pods on nodes"
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	//+optional
	// Affinity is a group of affinity scheduling rules.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// (Optional) Tolerations for scheduling pods onto some dedicated nodes
	//+optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

type BaseSpec struct {
	//Image is the metaservice docker image to deploy. the image can pull from dockerhub selectdb repository.
	Image string `json:"image,omitempty"`

	ServiceAccount string `json:"serviceAccount,omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	//defines the specification of resource cpu and mem. ep: {"requests":{"cpu": 4, "memory": "8Gi"},"limits":{"cpu":4,"memory":"8Gi"}}
	corev1.ResourceRequirements `json:",inline"`

	//Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata.
	Annotations map[string]string `json:"annotations,omitempty"`

	//+optional
	// export metaservice for accessing from outside k8s.
	Service *ExportService `json:"service,omitempty"`

	//+optional
	// Affinity is a group of affinity scheduling rules.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// (Optional) Tolerations for scheduling pods onto some dedicated nodes
	//+optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// ConfigMaps describe all configmaps that need to be mounted.
	ConfigMaps []ConfigMap `json:"configMaps,omitempty"`

	// (Optional) If specified, the pod's nodeSelector，displayName="Map of nodeSelectors to match when scheduling pods on nodes"
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	//+optional
	//EnvVars is a slice of environment variables that are added to the pods, the default is empty.
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`

	//+optional
	// Labels for user selector or classify pods
	Labels map[string]string `json:"labels,omitempty"`

	// HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
	// file if specified. This is only valid for non-hostNetwork pods.
	// +optional
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`

	PersistentVolume *PersistentVolume `json:"persistentVolume,omitempty"`

	//Security context for pod.
	//+optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	//Security context for all containers running in the pod (unless they override it).
	//+optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`
}

type MetaService struct {
	//the foundation spec for creating cn software services.
	//BaseSpec `json:"baseSpec,omitempty"`
	BaseSpec `json:",inline"`
}

type Recycler struct {
	//the foundation spec for creating cn software services.
	//BaseSpec `json:"baseSpec,omitempty"`
	BaseSpec `json:",inline"`
}

// PersistentVolume defines volume information and container mount information.
type PersistentVolume struct {
	// PersistentVolumeClaimSpec is a list of claim spec about storage that pods are required.
	// +kubebuilder:validation:Optional
	corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"`

	//Annotation for PVC pods. Users can adapt the storage authentication and pv binding of the cloud platform through configuration.
	//It only takes effect in the first configuration and cannot be added or modified later.
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ConfigMap struct {
	//Name specify the configmap in deployed namespace that need to be mounted in pod.
	Name string `json:"name,omitempty"`

	//MountPath specify the position of configmap be mounted.
	MountPath string `json:"mountPath,omitempty"`
}

type ExportService struct {
	//type of service,the possible value for the service type are : ClusterIP, NodePort, LoadBalancer,ExternalName.
	//More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	//PortMaps specify node port for target port in pod, when the service type=NodePort.
	PortMaps []PortMap `json:"portMaps,omitempty"`

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

type DorisDisaggregatedMetaServiceStatus struct {
	//describe the fdb status information: contains access address, fdb available or not, etc...
	FDBStatus FDBStatus `json:"fdbStatus,omitempty"`
	//describe the ms status.
	MSStatus BaseStatus `json:"msStatus,omitempty"`
	//descirbe the recycler status.
	RecyclerStatus BaseStatus `json:"recyclerStatus,omitempty"`
}

type FDBStatus struct {
	//FDBAddress describe the address for fdbclient using.
	FDBAddress string `json:"FDBAddress,omitempty"`
	//FDBResourceName specify the name of the kind `FoundationDBCluster` resource.
	FDBResourceName string `json:"fdbResourceName,omitempty"`
	//AvailableStatus represents the fdb available or not.
	AvailableStatus AvailableStatus `json:"availableStatus,omitempty"`
}

type BaseStatus struct {
	//Phase represent the stage of reconciling.
	Phase MetaServicePhase `json:"phase,omitempty"`

	//AvailableStatus represents the metaservice available or not.
	AvailableStatus AvailableStatus `json:"availableStatus,omitempty"`
}

type AvailableStatus string

const (
	//Available represents the service is available.
	Available AvailableStatus = "Available"

	//UnAvailable represents the service not available for using.
	UnAvailable AvailableStatus = "UnAvailable"
)

type MetaServicePhase string

const (
	//Ready represents the service is ready for accepting requests.
	Ready MetaServicePhase = "Ready"
	//Upgrading represents the spec of the service changed, service in smoothing upgrade.
	Upgrading MetaServicePhase = "Upgrading"
	//Failed represents service failed to start, can't be accessed.
	Failed MetaServicePhase = "Failed"
	//Creating represents service in creating stage.
	Creating MetaServicePhase = "Creating"
)

func init() {
	SchemeBuilder.Register(&DorisDisaggregatedMetaService{}, &DorisDisaggregatedMetaServiceList{})
}
