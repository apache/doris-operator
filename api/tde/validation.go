// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package tde

import (
	"fmt"
	"net/url"
	"reflect"
)

func Validate(config *TDEConfig) []error {
	if config == nil {
		return nil
	}
	var errs []error
	if config.ManagementPolicy != ManagementPolicyManaged && config.ManagementPolicy != ManagementPolicyObserveOnly {
		errs = append(errs, fmt.Errorf("spec.tde.managementPolicy must be Managed or ObserveOnly"))
	}
	if config.DefaultAlgorithm != "PLAINTEXT" && config.DefaultAlgorithm != "AES256" && config.DefaultAlgorithm != "SM4" {
		errs = append(errs, fmt.Errorf("spec.tde.defaultAlgorithm must be PLAINTEXT, AES256, or SM4"))
	}
	if config.Recovery != nil {
		if config.Recovery.RequestID == "" || config.Recovery.RotationRequestID == "" {
			errs = append(errs, fmt.Errorf("spec.tde.recovery.requestId and rotationRequestId are required"))
		}
		if config.Recovery.Decision != RecoveryConfirmApplied && config.Recovery.Decision != RecoveryConfirmNotApplied {
			errs = append(errs, fmt.Errorf("spec.tde.recovery.decision must be ConfirmApplied or ConfirmNotApplied"))
		}
	}
	switch config.Provider.Type {
	case ProviderLocal:
		if config.Provider.Local == nil || config.Provider.Local.SecretKeyRef.Name == "" || config.Provider.Local.SecretKeyRef.Key == "" {
			errs = append(errs, fmt.Errorf("spec.tde.provider.local.secretKeyRef.name and key are required for Local"))
		}
		if config.Provider.Kms != nil {
			errs = append(errs, fmt.Errorf("spec.tde.provider.kms must be empty for Local"))
		}
	case ProviderAwsKms, ProviderAliyunKms:
		kms := config.Provider.Kms
		if kms == nil || kms.KeyID == "" || kms.Endpoint == "" || kms.Region == "" {
			errs = append(errs, fmt.Errorf("spec.tde.provider.kms.keyId, endpoint, and region are required for %s", config.Provider.Type))
		} else if endpoint, err := url.Parse(kms.Endpoint); err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
			errs = append(errs, fmt.Errorf("spec.tde.provider.kms.endpoint must be a valid HTTPS URL"))
		}
		if config.Provider.Local != nil {
			errs = append(errs, fmt.Errorf("spec.tde.provider.local must be empty for %s", config.Provider.Type))
		}
		if kms != nil {
			switch kms.Auth.Type {
			case KmsAuthEnvironmentSecret:
				ref := kms.Auth.CredentialSecretRef
				if ref == nil || ref.Name == "" || ref.AccessKeyKey == "" || ref.SecretKeyKey == "" {
					errs = append(errs, fmt.Errorf("spec.tde.provider.kms.auth.credentialSecretRef fields are required for EnvironmentSecret"))
				}
			case KmsAuthInstanceRole:
				if kms.Auth.CredentialSecretRef != nil {
					errs = append(errs, fmt.Errorf("credentialSecretRef must be empty for InstanceRole"))
				}
			default:
				errs = append(errs, fmt.Errorf("spec.tde.provider.kms.auth.type must be EnvironmentSecret or InstanceRole"))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("spec.tde.provider.type must be Local, AwsKms, or AliyunKms"))
	}
	return errs
}

func ValidateCreate(config *TDEConfig) []error {
	errs := Validate(config)
	if config != nil && config.Recovery != nil {
		errs = append(errs, fmt.Errorf("spec.tde.recovery is only valid for an existing RotationOutcomeUnknown operation"))
	}
	return errs
}

func ValidateUpdate(oldConfig, newConfig *TDEConfig, status *TDEStatus) []error {
	errs := Validate(newConfig)
	if oldConfig == nil {
		return errs
	}
	if newConfig == nil {
		if OperationInProgress(status) || RotationOutcomeUnknown(status) || status != nil && status.Current != nil {
			errs = append(errs, fmt.Errorf("spec.tde cannot be removed after TDE reconciliation has started"))
		}
		return errs
	}
	if RotationOutcomeUnknown(status) {
		return append(errs, validateRecoveryUpdate(oldConfig, newConfig, status)...)
	}
	if rootRotationCancellation(newConfig, status) {
		if oldConfig.RootKeyRotation == nil || oldConfig.RootKeyRotation.RequestID != status.Operation.RequestID ||
			oldConfig.ManagementPolicy != newConfig.ManagementPolicy ||
			!reflect.DeepEqual(oldConfig.MasterKeyRotation, newConfig.MasterKeyRotation) ||
			!reflect.DeepEqual(oldConfig.CredentialRotation, newConfig.CredentialRotation) ||
			!reflect.DeepEqual(oldConfig.Recovery, newConfig.Recovery) {
			errs = append(errs, fmt.Errorf("canceling a root key rotation may only restore the captured source and clear rootKeyRotation"))
		}
		return errs
	}
	if status != nil && status.Current != nil && newConfig.ManagementPolicy != ManagementPolicyManaged {
		errs = append(errs, fmt.Errorf("spec.tde cannot leave Managed after TDE initialization"))
	}
	if !reflect.DeepEqual(oldConfig.Recovery, newConfig.Recovery) {
		clearingCompletedRecovery := oldConfig.Recovery != nil && newConfig.Recovery == nil && status != nil &&
			status.Operation != nil && status.Operation.Stage == StageCompleted && status.Operation.Recovery != nil &&
			status.Operation.Recovery.RequestID == oldConfig.Recovery.RequestID
		if !clearingCompletedRecovery {
			errs = append(errs, fmt.Errorf("spec.tde.recovery may only change while RotationOutcomeUnknown is true or be cleared after completion"))
		}
	}
	rootChanged := !sameRootIdentity(oldConfig.Provider, newConfig.Provider)
	authChanged := !sameAuth(oldConfig.Provider, newConfig.Provider)
	if rootChanged {
		if newConfig.RootKeyRotation == nil || newConfig.RootKeyRotation.RequestID == "" ||
			(oldConfig.RootKeyRotation != nil && oldConfig.RootKeyRotation.RequestID == newConfig.RootKeyRotation.RequestID) {
			errs = append(errs, fmt.Errorf("changing the TDE root key requires a new spec.tde.rootKeyRotation.requestId"))
		}
		if (oldConfig.Provider.Type == ProviderAwsKms && newConfig.Provider.Type == ProviderAliyunKms) ||
			(oldConfig.Provider.Type == ProviderAliyunKms && newConfig.Provider.Type == ProviderAwsKms) {
			errs = append(errs, fmt.Errorf("direct AwsKms/AliyunKms rotation is unsupported; rotate through Local using two requests"))
		}
		if newConfig.RootKeyRotation != nil && RequestIDUsed(status, newConfig.RootKeyRotation.RequestID) &&
			(status == nil || status.Operation == nil || status.Operation.Type != OperationRotateRootKey ||
				status.Operation.RequestID != newConfig.RootKeyRotation.RequestID) {
			errs = append(errs, fmt.Errorf("spec.tde.rootKeyRotation.requestId %q was already used", newConfig.RootKeyRotation.RequestID))
		}
	}
	if authChanged {
		if rootChanged {
			errs = append(errs, fmt.Errorf("KMS credentials and root key cannot change in the same update"))
		}
		if newConfig.CredentialRotation == nil || newConfig.CredentialRotation.RequestID == "" ||
			(oldConfig.CredentialRotation != nil && oldConfig.CredentialRotation.RequestID == newConfig.CredentialRotation.RequestID) {
			errs = append(errs, fmt.Errorf("changing KMS credentials requires a new spec.tde.credentialRotation.requestId"))
		}
		if newConfig.CredentialRotation != nil && RequestIDUsed(status, newConfig.CredentialRotation.RequestID) &&
			(status == nil || status.Operation == nil || status.Operation.Type != OperationRotateKmsCredential ||
				status.Operation.RequestID != newConfig.CredentialRotation.RequestID) {
			errs = append(errs, fmt.Errorf("spec.tde.credentialRotation.requestId %q was already used", newConfig.CredentialRotation.RequestID))
		}
	}
	if status != nil && status.Operation != nil && status.Operation.Stage != StageCompleted && status.Operation.Stage != StageFailed && !reflect.DeepEqual(oldConfig, newConfig) {
		errs = append(errs, fmt.Errorf("spec.tde cannot be changed while operation %q is in stage %q", status.Operation.RequestID, status.Operation.Stage))
	}
	return errs
}

func validateRecoveryUpdate(oldConfig, newConfig *TDEConfig, status *TDEStatus) []error {
	operation := status.Operation
	if operation == nil || operation.Type != OperationRotateRootKey || operation.SQLState != SQLOutcomeUnknown {
		return []error{fmt.Errorf("spec.tde cannot be changed while RotationOutcomeUnknown is true")}
	}
	recovery := newConfig.Recovery
	if recovery == nil {
		return []error{fmt.Errorf("spec.tde.recovery is required while RotationOutcomeUnknown is true")}
	}
	var errs []error
	if recovery.RotationRequestID != operation.RequestID {
		errs = append(errs, fmt.Errorf("spec.tde.recovery.rotationRequestId must match the unknown rotation request %q", operation.RequestID))
	}
	if operation.Recovery != nil && operation.Recovery.RequestID == recovery.RequestID {
		errs = append(errs, fmt.Errorf("spec.tde.recovery.requestId %q was already used", recovery.RequestID))
	}
	if RequestIDUsed(status, recovery.RequestID) && (operation.Recovery == nil || operation.Recovery.RequestID != recovery.RequestID) {
		errs = append(errs, fmt.Errorf("spec.tde.recovery.requestId %q was already used", recovery.RequestID))
	}
	switch recovery.Decision {
	case RecoveryConfirmApplied:
		oldWithoutRecovery := oldConfig.DeepCopy()
		oldWithoutRecovery.Recovery = nil
		newWithoutRecovery := newConfig.DeepCopy()
		newWithoutRecovery.Recovery = nil
		if !reflect.DeepEqual(oldWithoutRecovery, newWithoutRecovery) {
			errs = append(errs, fmt.Errorf("ConfirmApplied may only add spec.tde.recovery; the target TDE configuration must not change"))
		}
	case RecoveryConfirmNotApplied:
		if operation.Source == nil || !configMatchesProviderStatus(newConfig, operation.Source) {
			errs = append(errs, fmt.Errorf("ConfirmNotApplied must restore provider and algorithm to the captured source"))
		}
		if newConfig.RootKeyRotation != nil {
			errs = append(errs, fmt.Errorf("ConfirmNotApplied must clear spec.tde.rootKeyRotation"))
		}
		if !reflect.DeepEqual(oldConfig.MasterKeyRotation, newConfig.MasterKeyRotation) ||
			!reflect.DeepEqual(oldConfig.CredentialRotation, newConfig.CredentialRotation) ||
			oldConfig.ManagementPolicy != newConfig.ManagementPolicy {
			errs = append(errs, fmt.Errorf("ConfirmNotApplied cannot change unrelated TDE fields"))
		}
	}
	return errs
}

func configMatchesProviderStatus(config *TDEConfig, provider *ProviderStatus) bool {
	if config == nil || provider == nil || config.Provider.Type != provider.Provider || config.DefaultAlgorithm != provider.DefaultAlgorithm {
		return false
	}
	switch provider.Provider {
	case ProviderLocal:
		return config.Provider.Local != nil && provider.RootKeyRef != nil &&
			config.Provider.Local.SecretKeyRef.Name == provider.RootKeyRef.SecretName &&
			config.Provider.Local.SecretKeyRef.Key == provider.RootKeyRef.Key
	case ProviderAwsKms, ProviderAliyunKms:
		if config.Provider.Kms == nil || provider.Kms == nil {
			return false
		}
		kms, current := config.Provider.Kms, provider.Kms
		if kms.KeyID != current.KeyID || kms.Endpoint != current.Endpoint || kms.Region != current.Region || kms.Auth.Type != current.Auth.Type {
			return false
		}
		if kms.Auth.Type == KmsAuthInstanceRole {
			return kms.Auth.CredentialSecretRef == nil
		}
		return kms.Auth.CredentialSecretRef != nil &&
			kms.Auth.CredentialSecretRef.Name == current.Auth.CredentialSecretName &&
			kms.Auth.CredentialSecretRef.AccessKeyKey == current.Auth.AccessKeyKey &&
			kms.Auth.CredentialSecretRef.SecretKeyKey == current.Auth.SecretKeyKey
	default:
		return false
	}
}

func rootRotationCancellation(config *TDEConfig, status *TDEStatus) bool {
	return config != nil && config.RootKeyRotation == nil && status != nil && status.Operation != nil &&
		status.Operation.Type == OperationRotateRootKey && status.Operation.Stage != StageCompleted &&
		(status.Operation.SQLState == SQLNotStarted || status.Operation.SQLState == SQLRejected) && status.Operation.Source != nil &&
		configMatchesProviderStatus(config, status.Operation.Source)
}

func RotationOutcomeUnknown(status *TDEStatus) bool {
	if status == nil {
		return false
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == "RotationOutcomeUnknown" && status.Conditions[i].Status == "True" {
			return true
		}
	}
	return false
}

func OperationInProgress(status *TDEStatus) bool {
	return status != nil && status.Operation != nil && status.Operation.Stage != StageCompleted && status.Operation.Stage != StageFailed
}

func BlocksFELifecycle(status *TDEStatus) bool {
	return OperationInProgress(status) || RotationOutcomeUnknown(status) || conditionTrue(status, "ConfigSyncPending")
}

func conditionTrue(status *TDEStatus, conditionType string) bool {
	if status == nil {
		return false
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType && status.Conditions[i].Status == "True" {
			return true
		}
	}
	return false
}

func RequestIDUsed(status *TDEStatus, requestID string) bool {
	if status == nil || requestID == "" {
		return false
	}
	for _, used := range status.UsedRequestIDs {
		if used == requestID {
			return true
		}
	}
	return false
}

func sameRootIdentity(a, b ProviderSpec) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case ProviderLocal:
		return a.Local != nil && b.Local != nil && a.Local.SecretKeyRef == b.Local.SecretKeyRef
	case ProviderAwsKms, ProviderAliyunKms:
		return a.Kms != nil && b.Kms != nil && a.Kms.KeyID == b.Kms.KeyID && a.Kms.Endpoint == b.Kms.Endpoint && a.Kms.Region == b.Kms.Region
	default:
		return false
	}
}

func sameAuth(a, b ProviderSpec) bool {
	if a.Type == ProviderLocal || b.Type == ProviderLocal {
		return true
	}
	if a.Kms == nil || b.Kms == nil {
		return a.Kms == b.Kms
	}
	return reflect.DeepEqual(a.Kms.Auth, b.Kms.Auth)
}
