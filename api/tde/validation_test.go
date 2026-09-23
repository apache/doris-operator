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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateLocal(t *testing.T) {
	config := localConfig("key-v1")
	if errs := Validate(config); len(errs) != 0 {
		t.Fatalf("valid Local config rejected: %v", errs)
	}
	config.Provider.Local.SecretKeyRef.Key = ""
	if errs := Validate(config); len(errs) == 0 {
		t.Fatal("missing Secret key was accepted")
	}
}

func TestValidatePlaintextAlgorithm(t *testing.T) {
	config := localConfig("key-v1")
	config.DefaultAlgorithm = "PLAINTEXT"
	if errs := Validate(config); len(errs) != 0 {
		t.Fatalf("PLAINTEXT algorithm rejected: %v", errs)
	}
}

func TestValidateRejectsUnsupportedAES128(t *testing.T) {
	config := localConfig("key-v1")
	config.DefaultAlgorithm = "AES128"
	if errs := Validate(config); len(errs) == 0 {
		t.Fatal("AES128 was accepted even though the current FE does not support it")
	}
}

func TestValidateCreateRejectsRecovery(t *testing.T) {
	config := localConfig("key-v1")
	config.Recovery = &RecoveryRequest{RequestID: "recover-1", RotationRequestID: "rotate-1", Decision: RecoveryConfirmApplied}
	if errs := ValidateCreate(config); len(errs) == 0 {
		t.Fatal("recovery was accepted on create")
	}
}

func TestValidateUpdateRequiresRotationRequest(t *testing.T) {
	oldConfig := localConfig("key-v1")
	newConfig := localConfig("key-v2")
	if errs := ValidateUpdate(oldConfig, newConfig, nil); len(errs) == 0 {
		t.Fatal("root key change without requestId was accepted")
	}
	newConfig.RootKeyRotation = &RotationRequest{RequestID: "rotate-1"}
	if errs := ValidateUpdate(oldConfig, newConfig, nil); len(errs) != 0 {
		t.Fatalf("root key change with requestId rejected: %v", errs)
	}
}

func TestValidateUpdateRejectsRequestIDFromHistory(t *testing.T) {
	oldConfig := localConfig("key-v2")
	newConfig := localConfig("key-v3")
	newConfig.RootKeyRotation = &RotationRequest{RequestID: "rotate-1"}
	status := &TDEStatus{
		Current:        &ProviderStatus{Provider: ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &RootKeyRefStatus{SecretName: "key-v2", Key: "root.key"}},
		UsedRequestIDs: []string{"rotate-1"},
	}

	errs := ValidateUpdate(oldConfig, newConfig, status)
	if len(errs) == 0 || !strings.Contains(errs[len(errs)-1].Error(), "already used") {
		t.Fatalf("expected reused requestId error, got %v", errs)
	}
}

func TestValidateUpdateRejectsDirectCrossKMSRotation(t *testing.T) {
	oldConfig := kmsConfig(ProviderAwsKms, "aws-key")
	newConfig := kmsConfig(ProviderAliyunKms, "aliyun-key")
	newConfig.RootKeyRotation = &RotationRequest{RequestID: "rotate-1"}
	if errs := ValidateUpdate(oldConfig, newConfig, nil); len(errs) == 0 {
		t.Fatal("direct AwsKms to AliyunKms rotation was accepted")
	}
}

func TestValidateUpdateRejectsRemovalAfterReconciliationStarts(t *testing.T) {
	oldConfig := localConfig("key-v1")
	status := &TDEStatus{Operation: &OperationStatus{Type: OperationEnableTDE, Stage: StageRollingOutMaterial}}
	if errs := ValidateUpdate(oldConfig, nil, status); len(errs) == 0 {
		t.Fatal("removing spec.tde during enable was accepted")
	}
}

func TestValidateUpdateAcceptsConfirmAppliedRecovery(t *testing.T) {
	oldConfig := localConfig("key-v2")
	oldConfig.RootKeyRotation = &RotationRequest{RequestID: "rotate-1"}
	newConfig := oldConfig.DeepCopy()
	newConfig.Recovery = &RecoveryRequest{RequestID: "recover-1", RotationRequestID: "rotate-1", Decision: RecoveryConfirmApplied}
	status := unknownRotationStatus()
	if errs := ValidateUpdate(oldConfig, newConfig, status); len(errs) != 0 {
		t.Fatalf("valid ConfirmApplied recovery rejected: %v", errs)
	}
}

func TestValidateUpdateAcceptsConfirmNotAppliedRecovery(t *testing.T) {
	oldConfig := localConfig("key-v2")
	oldConfig.RootKeyRotation = &RotationRequest{RequestID: "rotate-1"}
	newConfig := localConfig("key-v1")
	newConfig.Recovery = &RecoveryRequest{RequestID: "recover-1", RotationRequestID: "rotate-1", Decision: RecoveryConfirmNotApplied}
	status := unknownRotationStatus()
	if errs := ValidateUpdate(oldConfig, newConfig, status); len(errs) != 0 {
		t.Fatalf("valid ConfirmNotApplied recovery rejected: %v", errs)
	}
}

func TestValidateUpdateRejectsUnknownOutcomeWithoutRecovery(t *testing.T) {
	config := localConfig("key-v2")
	config.RootKeyRotation = &RotationRequest{RequestID: "rotate-1"}
	newConfig := config.DeepCopy()
	newConfig.RootKeyRotation.RequestID = "rotate-2"
	if errs := ValidateUpdate(config, newConfig, unknownRotationStatus()); len(errs) == 0 {
		t.Fatal("unknown rotation outcome accepted a new request without recovery")
	}
}

func TestValidateUpdateAllowsClearingCompletedRecovery(t *testing.T) {
	oldConfig := localConfig("key-v2")
	oldConfig.RootKeyRotation = &RotationRequest{RequestID: "rotate-1"}
	oldConfig.Recovery = &RecoveryRequest{RequestID: "recover-1", RotationRequestID: "rotate-1", Decision: RecoveryConfirmApplied}
	newConfig := oldConfig.DeepCopy()
	newConfig.Recovery = nil
	status := &TDEStatus{Current: &ProviderStatus{Provider: ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &RootKeyRefStatus{SecretName: "key-v2", Key: "root.key"}},
		Operation: &OperationStatus{Type: OperationRotateRootKey, RequestID: "rotate-1", Stage: StageCompleted,
			Recovery: &RecoveryStatus{RequestID: "recover-1", Decision: RecoveryConfirmApplied}}}

	if errs := ValidateUpdate(oldConfig, newConfig, status); len(errs) != 0 {
		t.Fatalf("clearing a completed recovery was rejected: %v", errs)
	}
}

func TestValidateUpdateAllowsRootRotationCancellationBeforeSQL(t *testing.T) {
	oldConfig := localConfig("key-v2")
	oldConfig.RootKeyRotation = &RotationRequest{RequestID: "rotate-1"}
	newConfig := localConfig("key-v1")
	status := &TDEStatus{Current: &ProviderStatus{Provider: ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &RootKeyRefStatus{SecretName: "key-v1", Key: "root.key"}},
		Operation: &OperationStatus{Type: OperationRotateRootKey, RequestID: "rotate-1", Stage: StageRollingOutMaterial, SQLState: SQLNotStarted,
			Source: &ProviderStatus{Provider: ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &RootKeyRefStatus{SecretName: "key-v1", Key: "root.key"}},
			Target: &ProviderStatus{Provider: ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &RootKeyRefStatus{SecretName: "key-v2", Key: "root.key"}}}}

	if errs := ValidateUpdate(oldConfig, newConfig, status); len(errs) != 0 {
		t.Fatalf("safe pre-SQL cancellation was rejected: %v", errs)
	}
	status.Operation.SQLState = SQLSubmitting
	if errs := ValidateUpdate(oldConfig, newConfig, status); len(errs) == 0 {
		t.Fatal("cancellation was accepted after SQL submission started")
	}
}

func TestBlocksFELifecycleOnlyForUnsafeTDETransitions(t *testing.T) {
	status := &TDEStatus{Current: &ProviderStatus{Provider: ProviderLocal}, State: StateLocalKeyNotReady}
	if BlocksFELifecycle(status) {
		t.Fatal("material validation failure without an active operation blocked FE lifecycle")
	}
	status.Operation = &OperationStatus{Type: OperationRotateRootKey, Stage: StageRollingOutMaterial}
	if !BlocksFELifecycle(status) {
		t.Fatal("active root key rotation did not block FE lifecycle")
	}
	status.Operation.Stage = StageFailed
	status.Conditions = []metav1.Condition{{Type: "RotationOutcomeUnknown", Status: metav1.ConditionTrue}}
	if !BlocksFELifecycle(status) {
		t.Fatal("unknown rotation outcome did not block FE lifecycle")
	}
	status.Conditions = []metav1.Condition{{Type: "ConfigSyncPending", Status: metav1.ConditionTrue}}
	if !BlocksFELifecycle(status) {
		t.Fatal("pending TDE configuration sync did not block FE lifecycle")
	}
}

func unknownRotationStatus() *TDEStatus {
	return &TDEStatus{
		Current: &ProviderStatus{Provider: ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &RootKeyRefStatus{SecretName: "key-v1", Key: "root.key"}},
		Operation: &OperationStatus{
			Type: OperationRotateRootKey, RequestID: "rotate-1", Stage: StageFailed, SQLState: SQLOutcomeUnknown,
			Source: &ProviderStatus{Provider: ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &RootKeyRefStatus{SecretName: "key-v1", Key: "root.key"}},
			Target: &ProviderStatus{Provider: ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &RootKeyRefStatus{SecretName: "key-v2", Key: "root.key"}},
		},
		Conditions: []metav1.Condition{{Type: "RotationOutcomeUnknown", Status: metav1.ConditionTrue}},
	}
}

func localConfig(secret string) *TDEConfig {
	return &TDEConfig{ManagementPolicy: ManagementPolicyManaged, DefaultAlgorithm: "AES256",
		Provider: ProviderSpec{Type: ProviderLocal, Local: &LocalProviderSpec{SecretKeyRef: SecretKeyReference{Name: secret, Key: "root.key"}}}}
}

func kmsConfig(provider ProviderType, keyID string) *TDEConfig {
	return &TDEConfig{ManagementPolicy: ManagementPolicyManaged, DefaultAlgorithm: "AES256", Provider: ProviderSpec{Type: provider,
		Kms: &KmsProviderSpec{KeyID: keyID, Endpoint: "https://kms.example.com", Region: "region-1", Auth: KmsAuthSpec{Type: KmsAuthEnvironmentSecret,
			CredentialSecretRef: &KmsCredentialSecretReference{Name: "credentials", AccessKeyKey: "accessKey", SecretKeyKey: "secretKey"}}}}}
}
