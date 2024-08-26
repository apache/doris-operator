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

type DorisDisaggregatedClusterSpec struct {
	//VaultConfigmap specify the configmap that have configuration of file object information. example S3.
	//configmap have to config, please reference the doc.
	InstanceConfigMap string `json:"instanceConfigMap,omitempty"`

	//MetaService describe the metaservice that cluster want to storage metadata.
	DisMS DisMS `json:"disMS,omitempty"`

	//FeSpec describe the fe specification of doris disaggregated cluster.
	FeSpec FeSpec `json:"feSpec,omitempty"`

	//ComputeClusters describe a list of ComputeCluster, ComputeCluster is a group of compute node to do same thing.
	ComputeClusters []ComputeCluster `json:"computeClusters,omitempty"`
}

type DisMS struct {
	//Namespace specify the namespace of metaservice deployed.
	Namespace string `json:"namespace,omitempty"`
	//Name specify the name of metaservice resource.
	Name string `json:"name,omitempty"`
}

type FeSpec struct {
	CommonSpec `json:",inline"`
}

// ComputeCluster describe the specification that a group of compute node.
type ComputeCluster struct {
	//Name is the identifier of computeCluster, name can be used specify what computeCluster to run sql. if not set, will use `computeCluster` and the index in array to set.ep: computeCluster-1.
	Name string `json:"name,omitempty"`

	//ClusterId is the identifier of computeCluster, this will distinguish all computeCluster in meta.
	ClusterId string `json:"clusterId,omitempty"`

	CommonSpec `json:",inline"`
}

type CommonSpec struct {
	//Replicas represent the number of desired Pod.
	// fe default is 2. fe is master-slave architecture only one is master.
	Replicas *int32 `json:"replicas,omitempty"`

	//Image is the Disaggregated docker image to deploy. please reference the selectdb repository to find.
	Image string `json:"image,omitempty"`
	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// pod start timeout, unit is second
	StartTimeout int32 `json:"startTimeout,omitempty"`

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

	// VolumeClaimTemplate allows customizing the persistent volume claim for the pod.
	PersistentVolume *PersistentVolume `json:"persistentVolume,omitempty"`

	//when set true, the log will store in disk that created by volumeClaimTemplate
	NoStoreLog bool `json:"noStoreLog,omitempty"`

	// (Optional) Tolerations for scheduling pods onto some dedicated nodes
	//+optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// export metaservice for accessing from outside k8s.
	Service *ExportService `json:"service,omitempty"`

	// ConfigMaps describe all configmap that need to be mounted.
	ConfigMaps []ConfigMap `json:"configMaps,omitempty"`

	//secrets describe all secret that need to be mounted.
	//Secrets []Secret `json:"secrets,omitempty"`

	// specify what's node to deploy compute cluster pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	//serviceAccount for compute node access cloud service.
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
	// file if specified. This is only valid for non-hostNetwork pods.
	// +optional
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`

	//Security context for pod.
	//+optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	//Security context for all containers running in the pod (unless they override it).
	//+optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"` //+optional

	//EnvVars is a slice of environment variables that are added to the pods, the default is empty.
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`

	//SystemInitialization for fe, be setting system parameters.
	SystemInitialization *SystemInitialization `json:"systemInitialization,omitempty"`
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

	// specify mountPaths, if not config, operator will refer from be.conf `cache_file_path`.
	// when mountPaths=[]{"/opt/path1", "/opt/path2"}, will create two pvc mount the two paths. also, operator will mount the cache_file_path config in be.conf .
	// if mountPaths have duplicated path in cache_file_path, operator will only create one pvc.
	MountPaths []string `json:"mountPaths,omitempty"`

	//if config true, the log will mount a pvc to store logs. the pvc size is definitely 200Gi, as the log recycling system will regular recycling.
	LogNotStore bool `json:"logNotStore,omitempty"`

	//Annotation for PVC pods. Users can adapt the storage authentication and pv binding of the cloud platform through configuration.
	//It only takes effect in the first configuration and cannot be added or modified later.
	Annotations map[string]string `json:"annotations,omitempty"`
}

type Secret struct {
	//specify the secret need to be mounted in deployed namespace.
	Name string `json:"name,omitempty"`
	//display the path of secret be mounted in pod.
	MountPath string `json:"mountPath,omitempty"`
}

type ConfigMap struct {
	//Name specify the configmap need to be mounted in pod in deployed namespace.
	Name string `json:"name,omitempty"`

	//display the path of configMap be mounted in pod. the component start conf please mount to /etc/doris, ep: fe-configmap contains 'fe.conf', mountPath must be '/etc/doris'.
	// key in configMap's data is file name.
	MountPath string `json:"mountPath,omitempty"`
}

type ExportService struct {
	//type of service,the possible value for the service type are : ClusterIP, NodePort, LoadBalancer,ExternalName.
	//More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	//Annotations for using function on different cloud platform.
	Annotations map[string]string `json:"annotations,omitempty"`

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
	InstanceId string `json:"instanceId,omitempty"`

	// the ms address for store meta of disaggregated cluster.
	MsEndpoint string `json:"msEndpoint,omitempty"`

	//the token for access ms service.
	MsToken string `json:"msToken,omitempty"`

	//FEStatus describe the fe status.
	FEStatus FEStatus `json:"feStatus,omitempty"`

	ClusterHealth ClusterHealth `json:"clusterHealth,omitempty"`

	//ComputeClusterStatuses reflect a list of computeCluster status.
	ComputeClusterStatuses []ComputeClusterStatus `json:"computeClusterStatuses,omitempty"`
}
type Health string

var (
	Green  Health = "green"
	Yellow Health = "yellow"
	Red    Health = "red"
)

type ClusterHealth struct {
	//represents the cluster overall status.
	Health Health `json:"health,omitempty"`
	//represents the fe available or not.
	FeAvailable bool `json:"feAvailable,omitempty"`
	//the number of compute cluster.
	CCCount int32 `json:"ccCount,omitempty"`
	//the available numbers of compute cluster.
	CCAvailableCount int32 `json:"ccAvailableCount,omitempty"`
	//the full available numbers of compute cluster, represents all pod in compute cluster are ready.
	CCFullAvailableCount int32 `json:"ccFullAvailableCount,omitempty"`
}

type Phase string

const (
	Ready Phase = "Ready"
	//Upgrading represents the spec of the service changed, service in smoothing upgrade.
	Upgrading Phase = "Upgrading"
	//Failed represents service failed to start, can't be accessed.
	Failed Phase = "Failed"
	//Creating represents service in creating stage.
	Reconciling Phase = "Reconciling"
)

type AvailableStatus string

const (
	//Available represents the service is available.
	Available AvailableStatus = "Available"

	//UnAvailable represents the service not available for using.
	UnAvailable AvailableStatus = "UnAvailable"
)

type ComputeClusterStatus struct {
	//Phase represent the stage of reconciling.
	Phase Phase `json:"phase,omitempty"`
	// the statefulset of control this compute cluster pods.
	StatefulsetName string `json:"statefulsetName,omitempty"`
	// the service that can access the compute cluster pods.
	ServiceName string `json:"serviceName,omitempty"`
	//represents the compute cluster.
	ComputeClusterName string `json:"ComputeClusterName,omitempty"`
	//AvailableStatus represents the compute cluster available or not.
	AvailableStatus AvailableStatus `json:"availableStatus,omitempty"`
	//ClusterId display  the clusterId of compute cluster in meta.
	ClusterId string `json:"clusterId,omitempty"`

	// replicas is the number of Pods created by the StatefulSet controller.
	Replicas int32 `json:"replicas,omitempty"`

	// Total number of available pods (ready for at least minReadySeconds) targeted by this statefulset.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
}

type FEStatus struct {
	//Phase represent the stage of reconciling.
	Phase Phase `json:"phase,omitempty"`
	//AvailableStatus represents the fe available or not.
	AvailableStatus AvailableStatus `json:"availableStatus,omitempty"`
	//ClusterId display  the clusterId of fe in meta.
	ClusterId string `json:"clusterId,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ddc
// +kubebuilder:printcolumn:name="ClusterHealth",type=string,JSONPath=`.status.clusterHealth.health`
// +kubebuilder:printcolumn:name="FEPhase",type=string,JSONPath=`.status.feStatus.phase`
// +kubebuilder:printcolumn:name="CCCount",type=integer,JSONPath=`.status.clusterHealth.ccCount`
// +kubebuilder:printcolumn:name="CCAvailableCount",type=integer,JSONPath=`.status.clusterHealth.ccAvailableCount`
// +kubebuilder:printcolumn:name="CCFullAvailableCount",type=integer,JSONPath=`.status.clusterHealth.ccFullAvailableCount`
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
