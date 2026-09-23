// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tde

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ManagementPolicy string

const (
	ManagementPolicyManaged     ManagementPolicy = "Managed"
	ManagementPolicyObserveOnly ManagementPolicy = "ObserveOnly"
)

type ProviderType string

const (
	ProviderLocal     ProviderType = "Local"
	ProviderAwsKms    ProviderType = "AwsKms"
	ProviderAliyunKms ProviderType = "AliyunKms"
)

type KmsAuthType string

const (
	KmsAuthEnvironmentSecret KmsAuthType = "EnvironmentSecret"
	KmsAuthInstanceRole      KmsAuthType = "InstanceRole"
)

type TDEConfig struct {
	// +kubebuilder:validation:Enum=Managed;ObserveOnly
	ManagementPolicy ManagementPolicy `json:"managementPolicy"`
	Provider         ProviderSpec     `json:"provider"`
	// +kubebuilder:validation:Enum=PLAINTEXT;AES256;SM4
	DefaultAlgorithm   string                 `json:"defaultAlgorithm"`
	MasterKeyRotation  *MasterKeyRotationSpec `json:"masterKeyRotation,omitempty"`
	RootKeyRotation    *RotationRequest       `json:"rootKeyRotation,omitempty"`
	CredentialRotation *RotationRequest       `json:"credentialRotation,omitempty"`
	Recovery           *RecoveryRequest       `json:"recovery,omitempty"`
}

type ProviderSpec struct {
	// +kubebuilder:validation:Enum=Local;AwsKms;AliyunKms
	Type  ProviderType       `json:"type"`
	Local *LocalProviderSpec `json:"local,omitempty"`
	Kms   *KmsProviderSpec   `json:"kms,omitempty"`
}

type LocalProviderSpec struct {
	SecretKeyRef SecretKeyReference `json:"secretKeyRef"`
}

type SecretKeyReference struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type KmsProviderSpec struct {
	KeyID    string      `json:"keyId"`
	Endpoint string      `json:"endpoint"`
	Region   string      `json:"region"`
	Auth     KmsAuthSpec `json:"auth"`
}

type KmsAuthSpec struct {
	// +kubebuilder:validation:Enum=EnvironmentSecret;InstanceRole
	Type                KmsAuthType                   `json:"type"`
	CredentialSecretRef *KmsCredentialSecretReference `json:"credentialSecretRef,omitempty"`
}

type KmsCredentialSecretReference struct {
	Name         string `json:"name"`
	AccessKeyKey string `json:"accessKeyKey"`
	SecretKeyKey string `json:"secretKeyKey"`
}

type MasterKeyRotationSpec struct {
	// +kubebuilder:validation:Minimum=1
	RotateIntervalMs int64 `json:"rotateIntervalMs"`
	// +kubebuilder:validation:Minimum=1
	CheckIntervalMs int64 `json:"checkIntervalMs"`
}

type RotationRequest struct {
	// +kubebuilder:validation:MinLength=1
	RequestID string `json:"requestId"`
}

type RecoveryDecision string

const (
	RecoveryConfirmApplied    RecoveryDecision = "ConfirmApplied"
	RecoveryConfirmNotApplied RecoveryDecision = "ConfirmNotApplied"
)

type RecoveryRequest struct {
	// +kubebuilder:validation:MinLength=1
	RequestID string `json:"requestId"`
	// +kubebuilder:validation:MinLength=1
	RotationRequestID string `json:"rotationRequestId"`
	// +kubebuilder:validation:Enum=ConfirmApplied;ConfirmNotApplied
	Decision RecoveryDecision `json:"decision"`
}

type TDEStatus struct {
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	State               TDEState           `json:"state,omitempty"`
	MetadataInitialized bool               `json:"metadataInitialized,omitempty"`
	Current             *ProviderStatus    `json:"current,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
	Operation           *OperationStatus   `json:"operation,omitempty"`
	UsedRequestIDs      []string           `json:"usedRequestIds,omitempty"`
	ActiveConfigHash    string             `json:"activeConfigHash,omitempty"`
	ActiveFESpecHash    string             `json:"activeFeSpecHash,omitempty"`
	FEChecks            []FECheck          `json:"feChecks,omitempty"`
}

type TDEState string

const (
	StateUnconfigured            TDEState = "Unconfigured"
	StateConfiguredUninitialized TDEState = "ConfiguredUninitialized"
	StateActive                  TDEState = "Active"
	StateReconciling             TDEState = "Reconciling"
	StateConfigInconsistent      TDEState = "ConfigInconsistent"
	StateUnsupportedProvider     TDEState = "UnsupportedProvider"
	StateLocalKeyNotReady        TDEState = "LocalKeyNotReady"
	StateKmsNotReady             TDEState = "KmsNotReady"
	StateUnknown                 TDEState = "Unknown"
)

type ProviderStatus struct {
	Provider         ProviderType       `json:"provider"`
	DefaultAlgorithm string             `json:"defaultAlgorithm,omitempty"`
	RootKeyRef       *RootKeyRefStatus  `json:"rootKeyRef,omitempty"`
	Kms              *KmsProviderStatus `json:"kms,omitempty"`
}

type RootKeyRefStatus struct {
	SecretName   string `json:"secretName"`
	Key          string `json:"key"`
	SecretUID    string `json:"secretUid"`
	ResolvedPath string `json:"resolvedPath"`
}

type KmsProviderStatus struct {
	KeyID    string        `json:"keyId"`
	Endpoint string        `json:"endpoint"`
	Region   string        `json:"region"`
	Auth     KmsAuthStatus `json:"auth"`
}

type KmsAuthStatus struct {
	Type                 KmsAuthType `json:"type"`
	CredentialSecretName string      `json:"credentialSecretName,omitempty"`
	CredentialSecretUID  string      `json:"credentialSecretUid,omitempty"`
	AccessKeyKey         string      `json:"accessKeyKey,omitempty"`
	SecretKeyKey         string      `json:"secretKeyKey,omitempty"`
}

type OperationType string

const (
	OperationEnableTDE           OperationType = "EnableTDE"
	OperationUpdateAlgorithm     OperationType = "UpdateAlgorithm"
	OperationRotateKmsCredential OperationType = "RotateKmsCredential"
	OperationRotateRootKey       OperationType = "RotateRootKey"
)

type OperationStage string

const (
	StagePreparingMaterial    OperationStage = "PreparingMaterial"
	StageRollingOutMaterial   OperationStage = "RollingOutMaterial"
	StageRotatingRootKey      OperationStage = "RotatingRootKey"
	StageWaitingForFEReplay   OperationStage = "WaitingForFEReplay"
	StageSyncingConfiguration OperationStage = "SyncingConfiguration"
	StageCompleted            OperationStage = "Completed"
	StageFailed               OperationStage = "Failed"
)

type SQLState string

const (
	SQLNotStarted     SQLState = "NotStarted"
	SQLSubmitting     SQLState = "Submitting"
	SQLOutcomeUnknown SQLState = "OutcomeUnknown"
	SQLRejected       SQLState = "Rejected"
	SQLApplied        SQLState = "Applied"
	SQLNotApplied     SQLState = "NotApplied"
)

type RecoveryStatus struct {
	RequestID  string           `json:"requestId"`
	Decision   RecoveryDecision `json:"decision"`
	ResolvedAt metav1.Time      `json:"resolvedAt"`
}

type OperationStatus struct {
	Type               OperationType   `json:"type"`
	RequestID          string          `json:"requestId,omitempty"`
	SpecGeneration     int64           `json:"specGeneration"`
	Stage              OperationStage  `json:"stage"`
	SQLState           SQLState        `json:"sqlState,omitempty"`
	Source             *ProviderStatus `json:"source,omitempty"`
	Target             *ProviderStatus `json:"target,omitempty"`
	Recovery           *RecoveryStatus `json:"recovery,omitempty"`
	CommitJournalID    string          `json:"commitJournalId,omitempty"`
	StartedAt          metav1.Time     `json:"startedAt"`
	LastTransitionTime metav1.Time     `json:"lastTransitionTime"`
}

type FECheck struct {
	PodName          string `json:"podName"`
	PodReady         bool   `json:"podReady"`
	ConfigConsistent bool   `json:"configConsistent"`
	MaterialReady    bool   `json:"materialReady"`
}

func (in *TDEConfig) DeepCopyInto(out *TDEConfig) {
	*out = *in
	if in.Provider.Local != nil {
		out.Provider.Local = new(LocalProviderSpec)
		*out.Provider.Local = *in.Provider.Local
	}
	if in.Provider.Kms != nil {
		out.Provider.Kms = new(KmsProviderSpec)
		*out.Provider.Kms = *in.Provider.Kms
		if in.Provider.Kms.Auth.CredentialSecretRef != nil {
			out.Provider.Kms.Auth.CredentialSecretRef = new(KmsCredentialSecretReference)
			*out.Provider.Kms.Auth.CredentialSecretRef = *in.Provider.Kms.Auth.CredentialSecretRef
		}
	}
	if in.MasterKeyRotation != nil {
		out.MasterKeyRotation = new(MasterKeyRotationSpec)
		*out.MasterKeyRotation = *in.MasterKeyRotation
	}
	if in.RootKeyRotation != nil {
		out.RootKeyRotation = new(RotationRequest)
		*out.RootKeyRotation = *in.RootKeyRotation
	}
	if in.CredentialRotation != nil {
		out.CredentialRotation = new(RotationRequest)
		*out.CredentialRotation = *in.CredentialRotation
	}
	if in.Recovery != nil {
		out.Recovery = new(RecoveryRequest)
		*out.Recovery = *in.Recovery
	}
}

func (in *TDEConfig) DeepCopy() *TDEConfig {
	if in == nil {
		return nil
	}
	out := new(TDEConfig)
	in.DeepCopyInto(out)
	return out
}

func (in *TDEStatus) DeepCopyInto(out *TDEStatus) {
	*out = *in
	if in.Current != nil {
		out.Current = in.Current.DeepCopy()
	}
	if in.Conditions != nil {
		out.Conditions = append([]metav1.Condition(nil), in.Conditions...)
	}
	if in.Operation != nil {
		out.Operation = new(OperationStatus)
		*out.Operation = *in.Operation
		out.Operation.Source = in.Operation.Source.DeepCopy()
		out.Operation.Target = in.Operation.Target.DeepCopy()
		if in.Operation.Recovery != nil {
			out.Operation.Recovery = new(RecoveryStatus)
			*out.Operation.Recovery = *in.Operation.Recovery
		}
	}
	if in.UsedRequestIDs != nil {
		out.UsedRequestIDs = append([]string(nil), in.UsedRequestIDs...)
	}
	if in.FEChecks != nil {
		out.FEChecks = append([]FECheck(nil), in.FEChecks...)
	}
}

func (in *TDEStatus) DeepCopy() *TDEStatus {
	if in == nil {
		return nil
	}
	out := new(TDEStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *ProviderStatus) DeepCopy() *ProviderStatus {
	if in == nil {
		return nil
	}
	out := new(ProviderStatus)
	*out = *in
	if in.RootKeyRef != nil {
		out.RootKeyRef = new(RootKeyRefStatus)
		*out.RootKeyRef = *in.RootKeyRef
	}
	if in.Kms != nil {
		out.Kms = new(KmsProviderStatus)
		*out.Kms = *in.Kms
	}
	return out
}
