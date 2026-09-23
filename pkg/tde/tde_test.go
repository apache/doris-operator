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
	"context"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	tdev1 "github.com/apache/doris-operator/api/tde"
	hashutil "github.com/apache/doris-operator/pkg/common/utils/hash"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildRotateSQLLocalToLocal(t *testing.T) {
	source := localStatus("old-key", "old-uid")
	target := localStatus("new-key", "new-uid")
	sql := BuildRotateSQL(source, target)
	for _, expected := range []string{"ADMIN ROTATE TDE ROOT KEY", `"doris_tde_key_provider" = "local"`, source.RootKeyRef.ResolvedPath, target.RootKeyRef.ResolvedPath} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("SQL %q does not contain %q", sql, expected)
		}
	}
}

func TestBuildRotateSQLLocalToAwsKms(t *testing.T) {
	source := localStatus("old-key", "old-uid")
	target := kmsStatus(tdev1.ProviderAwsKms, "aws-key", "credentials", "credentials-uid")
	sql := BuildRotateSQL(source, target)
	for _, expected := range []string{
		`"doris_tde_key_provider" = "aws_kms"`,
		`"doris_tde_key_id" = "aws-key"`,
		`"doris_tde_key_endpoint" = "https://kms.example.com"`,
		`"doris_tde_key_region" = "region-1"`,
		source.RootKeyRef.ResolvedPath,
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("SQL %q does not contain %q", sql, expected)
		}
	}
	if strings.Contains(sql, "doris_tde_key_new_key_file") {
		t.Fatalf("KMS rotate SQL unexpectedly contains a local target key: %s", sql)
	}
}

func TestMergeFEConfigReplacesManagedValues(t *testing.T) {
	spec := localSpec("root-key")
	merged := mergeFEConfig("query_port=9030\ndoris_tde_key_provider=aws_kms\ndoris_tde_algorithm=SM4\n", providerStatusFromSpec(spec, nil), spec)
	if strings.Count(merged, "doris_tde_key_provider=") != 1 {
		t.Fatalf("provider was not replaced: %s", merged)
	}
	if !strings.Contains(merged, "doris_tde_key_provider=local") || !strings.Contains(merged, "query_port=9030") {
		t.Fatalf("unexpected merged config: %s", merged)
	}
}

func TestProviderForRuntimeUsesRotationTargetAfterSQLApplied(t *testing.T) {
	spec := localSpec("root-v2")
	status := &tdev1.TDEStatus{
		Current: localStatus("root-v1", "uid-1"),
		Operation: &tdev1.OperationStatus{
			Type:     tdev1.OperationRotateRootKey,
			Stage:    tdev1.StageSyncingConfiguration,
			SQLState: tdev1.SQLApplied,
			Source:   localStatus("root-v1", "uid-1"),
			Target:   localStatus("root-v2", "uid-2"),
		},
	}

	provider := providerForRuntime(spec, status)
	if provider == nil || provider.RootKeyRef == nil || provider.RootKeyRef.SecretName != "root-v2" {
		t.Fatalf("SQL-applied rotation must render the target provider, got %#v", provider)
	}
}

func TestProviderForRuntimeWaitsForFEReplayBeforeTargetConfig(t *testing.T) {
	spec := localSpec("root-v2")
	status := &tdev1.TDEStatus{Current: localStatus("root-v1", "uid-1"), Operation: &tdev1.OperationStatus{
		Type: tdev1.OperationRotateRootKey, Stage: tdev1.StageWaitingForFEReplay, SQLState: tdev1.SQLApplied,
		Source: localStatus("root-v1", "uid-1"), Target: localStatus("root-v2", "uid-2"),
	}}

	provider := providerForRuntime(spec, status)
	if provider == nil || provider.RootKeyRef == nil || provider.RootKeyRef.SecretName != "root-v1" {
		t.Fatalf("source config must remain active until FE replay completes, got %#v", provider)
	}
	providers := providersForPod(spec, status)
	if len(providers) != 2 {
		t.Fatalf("source and target materials must remain mounted while waiting for FE replay, got %#v", providers)
	}
}

func TestPrepareDCRConfigCreatesEffectiveConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dorisv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	base := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "fe-config", Namespace: "test"}, Data: map[string]string{"fe.conf": "query_port=9030\n"}}
	replicas := int32(1)
	dcr := &dorisv1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "test", UID: types.UID("cluster-uid")},
		Spec: dorisv1.DorisClusterSpec{TDE: localSpec("root-key"), FeSpec: &dorisv1.FeSpec{BaseSpec: dorisv1.BaseSpec{Replicas: &replicas,
			ConfigMapInfo: dorisv1.ConfigMapInfo{ConfigMapName: "fe-config"}}}}}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(base, dcr).Build()
	working, err := PrepareDCRConfig(context.Background(), client, dcr)
	if err != nil {
		t.Fatal(err)
	}
	if working.Spec.FeSpec.ConfigMapInfo.ConfigMapName == "fe-config" {
		t.Fatal("effective ConfigMap was not selected")
	}
	var effective corev1.ConfigMap
	if err := client.Get(context.Background(), types.NamespacedName{Name: working.Spec.FeSpec.ConfigMapInfo.ConfigMapName, Namespace: "test"}, &effective); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(effective.Data["fe.conf"], "doris_tde_root_key_file=") {
		t.Fatalf("TDE config missing: %s", effective.Data["fe.conf"])
	}
}

func TestApplyPodOverlayKeepsBothLocalKeysBeforeRotate(t *testing.T) {
	spec := localSpec("new-key")
	status := &tdev1.TDEStatus{Operation: &tdev1.OperationStatus{Type: tdev1.OperationRotateRootKey, Stage: tdev1.StageRollingOutMaterial,
		SQLState: tdev1.SQLNotStarted, Source: localStatus("old-key", "old-uid"), Target: localStatus("new-key", "new-uid")}}
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe"}}}}
	ApplyPodOverlay(template, "fe", spec, status)
	if len(template.Spec.Volumes) != 2 || len(template.Spec.Containers[0].VolumeMounts) != 2 {
		t.Fatalf("expected source and target mounts, got %#v", template.Spec)
	}
}

func TestApplyPodOverlayHashIsStable(t *testing.T) {
	spec := localSpec("root-key")
	status := &tdev1.TDEStatus{Operation: &tdev1.OperationStatus{Type: tdev1.OperationEnableTDE, Target: localStatus("root-key", "root-uid")}}
	var expected string
	for i := 0; i < 20; i++ {
		template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe"}}}}
		ApplyPodOverlay(template, "fe", spec.DeepCopy(), status.DeepCopy())
		if i == 0 {
			expected = template.Annotations[configAnnotation]
		}
		if template.Annotations[configAnnotation] != expected {
			t.Fatalf("TDE config hash changed: want %s, got %s", expected, template.Annotations[configAnnotation])
		}
		if template.Annotations[materialAnnotation] != "root-key:root-uid" {
			t.Fatalf("initial material UID missing: %q", template.Annotations[materialAnnotation])
		}
	}
}

func TestApplyPodOverlayInjectsKmsCredentials(t *testing.T) {
	spec := kmsSpec(tdev1.ProviderAwsKms, "aws-key", "credentials")
	status := &tdev1.TDEStatus{Current: kmsStatus(tdev1.ProviderAwsKms, "aws-key", "credentials", "credentials-uid")}
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe"}}}}
	ApplyPodOverlay(template, "fe", spec, status)

	envs := template.Spec.Containers[0].Env
	if len(envs) != 2 || envs[0].Name != accessKeyEnv || envs[0].ValueFrom.SecretKeyRef.Name != "credentials" ||
		envs[0].ValueFrom.SecretKeyRef.Key != "accessKey" || envs[1].Name != secretKeyEnv ||
		envs[1].ValueFrom.SecretKeyRef.Key != "secretKey" {
		t.Fatalf("unexpected KMS environment: %#v", envs)
	}
	if template.Annotations[materialAnnotation] != "credentials:credentials-uid" {
		t.Fatalf("unexpected material annotation: %q", template.Annotations[materialAnnotation])
	}
}

func TestResolveProviderValidatesKmsEnvironmentSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	immutable := true
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "test", UID: types.UID("credentials-uid")},
		Immutable: &immutable, Data: map[string][]byte{"accessKey": []byte("test-ak"), "secretKey": []byte("test-sk")}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	spec := kmsSpec(tdev1.ProviderAwsKms, "aws-key", "credentials")

	originalCheck := kmsReadinessCheck
	defer func() { kmsReadinessCheck = originalCheck }()
	called := false
	kmsReadinessCheck = func(_ context.Context, provider tdev1.ProviderType, kms *tdev1.KmsProviderStatus, accessKey, secretKey []byte) error {
		called = true
		if provider != tdev1.ProviderAwsKms || kms.KeyID != "aws-key" || string(accessKey) != "test-ak" || string(secretKey) != "test-sk" {
			t.Fatalf("unexpected KMS readiness input: provider=%s kms=%#v", provider, kms)
		}
		return nil
	}

	provider, _, err := resolveProvider(context.Background(), c, "test", spec)
	if err != nil {
		t.Fatal(err)
	}
	if !called || provider.Kms.Auth.CredentialSecretUID != "credentials-uid" {
		t.Fatalf("KMS readiness was not checked or UID was not captured: %#v", provider)
	}
}

func TestResolveProviderSkipsOperatorIdentityForInstanceRole(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	spec := kmsSpec(tdev1.ProviderAliyunKms, "aliyun-key", "")
	spec.Provider.Kms.Auth = tdev1.KmsAuthSpec{Type: tdev1.KmsAuthInstanceRole}

	originalCheck := kmsReadinessCheck
	defer func() { kmsReadinessCheck = originalCheck }()
	kmsReadinessCheck = func(context.Context, tdev1.ProviderType, *tdev1.KmsProviderStatus, []byte, []byte) error {
		t.Fatal("InstanceRole must not be validated with the Operator Pod identity")
		return nil
	}

	provider, _, err := resolveProvider(context.Background(), c, "test", spec)
	if err != nil {
		t.Fatal(err)
	}
	if provider.Kms.Auth.Type != tdev1.KmsAuthInstanceRole {
		t.Fatalf("unexpected provider: %#v", provider)
	}
}

func TestLocalVolumeNameAcceptsDNSSubdomainSecret(t *testing.T) {
	name := localVolumeName("root.key.v2", "root.key")
	if strings.Contains(name, ".") {
		t.Fatalf("volume name must be a DNS label: %q", name)
	}
}

func TestValidateTransitionRejectsUnsafeKmsChanges(t *testing.T) {
	tests := []struct {
		name    string
		status  *tdev1.TDEStatus
		target  *tdev1.ProviderStatus
		spec    *tdev1.TDEConfig
		message string
	}{
		{
			name:    "direct cross KMS rotation",
			status:  &tdev1.TDEStatus{Current: kmsStatus(tdev1.ProviderAwsKms, "aws-key", "credentials", "uid-1")},
			target:  kmsStatus(tdev1.ProviderAliyunKms, "aliyun-key", "credentials", "uid-1"),
			spec:    kmsSpec(tdev1.ProviderAliyunKms, "aliyun-key", "credentials"),
			message: "direct AwsKms/AliyunKms",
		},
		{
			name:    "credential change without request",
			status:  &tdev1.TDEStatus{Current: kmsStatus(tdev1.ProviderAwsKms, "aws-key", "credentials-v1", "uid-1")},
			target:  kmsStatus(tdev1.ProviderAwsKms, "aws-key", "credentials-v2", "uid-2"),
			spec:    kmsSpec(tdev1.ProviderAwsKms, "aws-key", "credentials-v2"),
			message: "credentialRotation.requestId",
		},
		{
			name: "reused root request",
			status: &tdev1.TDEStatus{Current: localStatus("root-v1", "uid-1"), Operation: &tdev1.OperationStatus{
				Type: tdev1.OperationRotateRootKey, RequestID: "rotate-1", Stage: tdev1.StageCompleted, Target: localStatus("root-v2", "uid-2"),
			}},
			target: localStatus("root-v3", "uid-3"),
			spec: func() *tdev1.TDEConfig {
				spec := localSpec("root-v3")
				spec.RootKeyRotation = &tdev1.RotationRequest{RequestID: "rotate-1"}
				return spec
			}(),
			message: "already used",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransition(tt.status, tt.target, tt.spec)
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("expected error containing %q, got %v", tt.message, err)
			}
		})
	}
}

func TestValidateTransitionRejectsActiveOperationMutation(t *testing.T) {
	status := &tdev1.TDEStatus{Current: localStatus("root-v1", "uid-1"), Operation: &tdev1.OperationStatus{
		Type: tdev1.OperationRotateRootKey, RequestID: "rotate-1", Stage: tdev1.StageRollingOutMaterial,
		Target: localStatus("root-v2", "uid-2"),
	}}
	spec := localSpec("root-v3")
	spec.RootKeyRotation = &tdev1.RotationRequest{RequestID: "rotate-2"}
	err := validateTransition(status, localStatus("root-v3", "uid-3"), spec)
	if err == nil || !strings.Contains(err.Error(), "cannot change") {
		t.Fatalf("expected active operation mutation to be rejected, got %v", err)
	}
}

func TestValidateTransitionRejectsRequestIDFromHistory(t *testing.T) {
	status := &tdev1.TDEStatus{Current: localStatus("root-v2", "uid-2"), UsedRequestIDs: []string{"rotate-1"}}
	spec := localSpec("root-v3")
	spec.RootKeyRotation = &tdev1.RotationRequest{RequestID: "rotate-1"}

	err := validateTransition(status, localStatus("root-v3", "uid-3"), spec)
	if err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("expected historical requestId reuse to be rejected, got %v", err)
	}
}

func TestValidateTransitionBlocksUnknownOutcomeRetry(t *testing.T) {
	status := &tdev1.TDEStatus{
		Current: localStatus("root-v1", "uid-1"),
		Operation: &tdev1.OperationStatus{
			Type:      tdev1.OperationRotateRootKey,
			RequestID: "rotate-1",
			Stage:     tdev1.StageFailed,
			SQLState:  tdev1.SQLOutcomeUnknown,
			Target:    localStatus("root-v2", "uid-2"),
		},
		Conditions: []metav1.Condition{{Type: conditionRotationOutcomeUnknown, Status: metav1.ConditionTrue}},
	}
	spec := localSpec("root-v2")
	spec.RootKeyRotation = &tdev1.RotationRequest{RequestID: "rotate-2"}
	err := validateTransition(status, localStatus("root-v2", "uid-2"), spec)
	if err == nil || !strings.Contains(err.Error(), "outcome is unknown") {
		t.Fatalf("expected unknown outcome to block a new request, got %v", err)
	}
}

func TestReconcileRecoveryConfirmApplied(t *testing.T) {
	status := &tdev1.TDEStatus{
		State:   tdev1.StateReconciling,
		Current: localStatus("root-v1", "uid-1"),
		Operation: &tdev1.OperationStatus{
			Type: tdev1.OperationRotateRootKey, RequestID: "rotate-1", Stage: tdev1.StageFailed, SQLState: tdev1.SQLOutcomeUnknown,
			Source: localStatus("root-v1", "uid-1"), Target: localStatus("root-v2", "uid-2"),
		},
		Conditions: []metav1.Condition{{Type: conditionRotationOutcomeUnknown, Status: metav1.ConditionTrue}},
	}
	spec := localSpec("root-v2")
	spec.RootKeyRotation = &tdev1.RotationRequest{RequestID: "rotate-1"}
	spec.Recovery = &tdev1.RecoveryRequest{RequestID: "recover-1", RotationRequestID: "rotate-1", Decision: tdev1.RecoveryConfirmApplied}

	handled, result := reconcileRecovery(status, localStatus("root-v2", "uid-2"), spec, true, 3)
	if !handled || !result.Requeue || status.Operation.SQLState != tdev1.SQLApplied || status.Operation.Stage != tdev1.StageSyncingConfiguration {
		t.Fatalf("unexpected recovery result: handled=%v result=%+v status=%#v", handled, result, status.Operation)
	}
	if tdev1.RotationOutcomeUnknown(status) || status.Operation.Recovery == nil || status.Operation.Recovery.RequestID != "recover-1" {
		t.Fatalf("unknown condition or recovery audit was not updated: %#v", status)
	}
}

func TestReconcileRecoveryConfirmNotApplied(t *testing.T) {
	status := &tdev1.TDEStatus{
		State:   tdev1.StateReconciling,
		Current: localStatus("root-v1", "uid-1"),
		Operation: &tdev1.OperationStatus{
			Type: tdev1.OperationRotateRootKey, RequestID: "rotate-1", Stage: tdev1.StageFailed, SQLState: tdev1.SQLOutcomeUnknown,
			Source: localStatus("root-v1", "uid-1"), Target: localStatus("root-v2", "uid-2"),
		},
		Conditions: []metav1.Condition{{Type: conditionRotationOutcomeUnknown, Status: metav1.ConditionTrue}},
	}
	spec := localSpec("root-v1")
	spec.Recovery = &tdev1.RecoveryRequest{RequestID: "recover-1", RotationRequestID: "rotate-1", Decision: tdev1.RecoveryConfirmNotApplied}

	handled, result := reconcileRecovery(status, localStatus("root-v1", "uid-1"), spec, false, 3)
	if !handled || !result.Requeue || status.Operation.SQLState != tdev1.SQLNotApplied || status.Operation.Stage != tdev1.StageSyncingConfiguration {
		t.Fatalf("unexpected recovery start: handled=%v result=%+v status=%#v", handled, result, status.Operation)
	}
	providers := providersForPod(spec, status)
	if len(providers) != 1 || providers[0].RootKeyRef.SecretName != "root-v1" {
		t.Fatalf("ConfirmNotApplied must remove target material, got %#v", providers)
	}
	handled, result = reconcileRecovery(status, localStatus("root-v1", "uid-1"), spec, true, 3)
	if !handled || !result.IsZero() || status.Operation.Stage != tdev1.StageCompleted || status.State != tdev1.StateActive {
		t.Fatalf("unexpected recovery completion: handled=%v result=%+v status=%#v", handled, result, status)
	}
}

func TestRootRotationCancellationCompletesWithoutSQL(t *testing.T) {
	status := &tdev1.TDEStatus{Current: localStatus("root-v1", "uid-1"), Operation: &tdev1.OperationStatus{
		Type: tdev1.OperationRotateRootKey, RequestID: "rotate-1", Stage: tdev1.StageRollingOutMaterial,
		SQLState: tdev1.SQLNotStarted, Source: localStatus("root-v1", "uid-1"), Target: localStatus("root-v2", "uid-2"),
	}}
	spec := localSpec("root-v1")
	if !rootRotationCancellationRequested(status, localStatus("root-v1", "uid-1"), spec) {
		t.Fatal("safe pre-SQL cancellation was not recognized")
	}
	status.Operation.SQLState = tdev1.SQLSubmitting
	if rootRotationCancellationRequested(status, localStatus("root-v1", "uid-1"), spec) {
		t.Fatal("cancellation was accepted after SQL submission started")
	}
}

func TestFELifecycleRuntimeGate(t *testing.T) {
	spec := localSpec("root-v2")
	feSpecV1 := &dorisv1.FeSpec{BaseSpec: dorisv1.BaseSpec{Image: "fe:v1"}}
	feSpecV2 := feSpecV1.DeepCopy()
	feSpecV2.Image = "fe:v2"
	status := &tdev1.TDEStatus{
		Current:          localStatus("root-v1", "uid-1"),
		State:            tdev1.StateActive,
		ActiveConfigHash: hashutil.HashObject(localSpec("root-v1")),
		ActiveFESpecHash: hashutil.HashObject(feSpecV1),
	}
	if !feLifecycleChangeBlocked(spec, feSpecV2, status) {
		t.Fatal("simultaneous TDE and FE changes were not blocked")
	}
	if feLifecycleChangeBlocked(localSpec("root-v1"), feSpecV2, status) {
		t.Fatal("an FE-only change while TDE is stable was blocked")
	}
	status.Operation = &tdev1.OperationStatus{Type: tdev1.OperationRotateRootKey, Stage: tdev1.StageRollingOutMaterial}
	if !feLifecycleChangeBlocked(localSpec("root-v1"), feSpecV2, status) {
		t.Fatal("an FE change during root key rotation was not blocked")
	}
}

func TestReconcileStartsRootRotationBeforeConnecting(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	immutable := true
	oldSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "old-key", Namespace: "test", UID: types.UID("old-uid")},
		Immutable: &immutable, Data: map[string][]byte{"root.key": []byte("MDEyMzQ1Njc4OWFiY2RlZg==")}}
	newSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "new-key", Namespace: "test", UID: types.UID("new-uid")},
		Immutable: &immutable, Data: map[string][]byte{"root.key": []byte("ZmVkY2JhOTg3NjU0MzIxMA==")}}
	replicas := int32(1)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-fe", Namespace: "test", Generation: 1},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "fe"}}},
		Status:     appsv1.StatefulSetStatus{ObservedGeneration: 1, CurrentRevision: "old", UpdateRevision: "old", ReadyReplicas: 1, UpdatedReplicas: 1},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-fe-0", Namespace: "test", Labels: map[string]string{
		"app": "fe", appsv1.ControllerRevisionHashLabelKey: "old"}}, Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(oldSecret, newSecret, sts, pod).Build()
	spec := localSpec("new-key")
	spec.RootKeyRotation = &tdev1.RotationRequest{RequestID: "rotate-1"}
	status := &tdev1.TDEStatus{State: tdev1.StateActive, Current: localStatus("old-key", "old-uid")}
	connectCalled := false

	result, err := reconcile(context.Background(), c, reconcileInput{
		object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "test"}},
		spec:   spec, status: &status, generation: 2, statefulSetName: "demo-fe",
		connect: func(context.Context) (*mysql.DB, error) {
			connectCalled = true
			return nil, fmt.Errorf("must not connect before rollout")
		},
		persistTDEStatus: func(context.Context, *tdev1.TDEStatus) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Requeue || connectCalled {
		t.Fatalf("new rotation must requeue before SQL connection: result=%+v connectCalled=%v", result, connectCalled)
	}
	if status.Operation == nil || status.Operation.Stage != tdev1.StageRollingOutMaterial || status.Operation.SQLState != tdev1.SQLNotStarted {
		t.Fatalf("unexpected operation status: %#v", status.Operation)
	}
}

func TestWorkloadReadyRejectsStaleTDETemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	replicas := int32(1)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-fe", Namespace: "test", Generation: 1},
		Spec: appsv1.StatefulSetSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "fe"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				materialAnnotation: "root-v1:uid-1", configAnnotation: "old-config",
			}}}},
		Status: appsv1.StatefulSetStatus{ObservedGeneration: 1, CurrentRevision: "ready", UpdateRevision: "ready", ReadyReplicas: 1, UpdatedReplicas: 1},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-fe-0", Namespace: "test", Labels: map[string]string{
		"app": "fe", appsv1.ControllerRevisionHashLabelKey: "ready"}, Annotations: map[string]string{
		materialAnnotation: "root-v1:uid-1", configAnnotation: "old-config",
	}}, Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts, pod).Build()
	spec := localSpec("root-v2")
	status := &tdev1.TDEStatus{Current: localStatus("root-v1", "uid-1"), Operation: &tdev1.OperationStatus{
		Type: tdev1.OperationRotateRootKey, RequestID: "rotate-1", Stage: tdev1.StageRollingOutMaterial,
		SQLState: tdev1.SQLNotStarted, Source: localStatus("root-v1", "uid-1"), Target: localStatus("root-v2", "uid-2"),
	}}

	ready, checks, err := workloadReady(context.Background(), c, "test", "demo-fe", spec, status)
	if err != nil {
		t.Fatal(err)
	}
	if ready || len(checks) != 1 || checks[0].ConfigConsistent || checks[0].MaterialReady {
		t.Fatalf("stale TDE template was considered ready: ready=%v checks=%#v", ready, checks)
	}
}

func TestMySQLErrorIsConfirmedServerRejection(t *testing.T) {
	message, rejected := confirmedSQLRejection(&mysqldriver.MySQLError{Number: 1105, Message: "key file missing"})
	if !rejected || message != "key file missing" {
		t.Fatal("MySQL server error was not recognized as a confirmed rejection")
	}
}

func TestSubmittingGracePeriodDefersOutcomeUnknown(t *testing.T) {
	now := time.Now()
	operation := &tdev1.OperationStatus{LastTransitionTime: metav1.NewTime(now.Add(-10 * time.Second))}
	remaining := submittingGraceRemaining(operation, now)
	if remaining <= 0 || remaining > submittingGracePeriod {
		t.Fatalf("unexpected submitting grace period remaining: %s", remaining)
	}
}

func TestExecuteRotateSQLHonorsTimeout(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	db := &mysql.DB{DB: sqlx.NewDb(rawDB, "sqlmock")}
	source := localStatus("root-v1", "uid-1")
	target := localStatus("root-v2", "uid-2")
	mock.ExpectExec(regexp.QuoteMeta(BuildRotateSQL(source, target))).WillDelayFor(100 * time.Millisecond).WillReturnResult(sqlmock.NewResult(0, 1))

	originalTimeout := rotationSQLTimeout
	rotationSQLTimeout = 10 * time.Millisecond
	defer func() { rotationSQLTimeout = originalTimeout }()
	if err := executeRotateSQL(context.Background(), db, source, target); err == nil {
		t.Fatal("rotate SQL did not honor its execution timeout")
	}
}

func TestFrontendsHaveReplayedCommitJournal(t *testing.T) {
	operation := &tdev1.OperationStatus{}
	frontends := []*mysql.Frontend{
		{IsMaster: true, Join: true, Alive: true, ReplayedJournalId: "102"},
		{Join: true, Alive: true, ReplayedJournalId: "101"},
	}
	replayed, err := frontendsHaveReplayed(frontends, operation)
	if err != nil {
		t.Fatal(err)
	}
	if replayed || operation.CommitJournalID != "102" {
		t.Fatalf("lagging follower was accepted or commit journal was not captured: replayed=%v operation=%#v", replayed, operation)
	}
	frontends[1].ReplayedJournalId = "102"
	replayed, err = frontendsHaveReplayed(frontends, operation)
	if err != nil || !replayed {
		t.Fatalf("fully replayed FE set was rejected: replayed=%v err=%v", replayed, err)
	}
}

func TestAllFrontendsReplayedIgnoresFutureColumns(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	db := &mysql.DB{DB: sqlx.NewDb(rawDB, "sqlmock")}
	columns := []string{"Name", "Host", "EditLogPort", "HttpPort", "QueryPort", "RpcPort", "ArrowFlightSqlPort", "Role", "IsMaster",
		"ClusterId", "Join", "Alive", "ReplayedJournalId", "LastStartTime", "LastHeartbeat", "IsHelper", "ErrMsg", "Version", "CurrentConnected",
		"LiveSince", "LocalResourceGroup"}
	values := []driver.Value{"fe-0", "fe-0.internal.svc", 9010, 8030, 9030, 9020, -1, "FOLLOWER", true,
		"1", true, true, "92", "2026-09-23 12:00:00", "2026-09-23 12:00:01", true, "", "doris", "Yes",
		"2026-09-23 12:00:00", ""}
	mock.ExpectQuery("SHOW FRONTENDS").WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))

	operation := &tdev1.OperationStatus{}
	replayed, err := allFrontendsReplayed(context.Background(), db, operation)
	if err != nil || !replayed || operation.CommitJournalID != "92" {
		t.Fatalf("future SHOW FRONTENDS columns blocked replay detection: replayed=%v commit=%q err=%v", replayed, operation.CommitJournalID, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func localSpec(secret string) *tdev1.TDEConfig {
	return &tdev1.TDEConfig{ManagementPolicy: tdev1.ManagementPolicyManaged, DefaultAlgorithm: "AES256",
		Provider: tdev1.ProviderSpec{Type: tdev1.ProviderLocal, Local: &tdev1.LocalProviderSpec{SecretKeyRef: tdev1.SecretKeyReference{Name: secret, Key: "root.key"}}}}
}

func localStatus(secret, uid string) *tdev1.ProviderStatus {
	return &tdev1.ProviderStatus{Provider: tdev1.ProviderLocal, DefaultAlgorithm: "AES256", RootKeyRef: &tdev1.RootKeyRefStatus{
		SecretName: secret, Key: "root.key", SecretUID: uid, ResolvedPath: localKeyPath(secret, "root.key")}}
}

func kmsSpec(provider tdev1.ProviderType, keyID, secret string) *tdev1.TDEConfig {
	return &tdev1.TDEConfig{ManagementPolicy: tdev1.ManagementPolicyManaged, DefaultAlgorithm: "AES256", Provider: tdev1.ProviderSpec{
		Type: provider, Kms: &tdev1.KmsProviderSpec{KeyID: keyID, Endpoint: "https://kms.example.com", Region: "region-1",
			Auth: tdev1.KmsAuthSpec{Type: tdev1.KmsAuthEnvironmentSecret, CredentialSecretRef: &tdev1.KmsCredentialSecretReference{
				Name: secret, AccessKeyKey: "accessKey", SecretKeyKey: "secretKey",
			}}},
	}}
}

func kmsStatus(provider tdev1.ProviderType, keyID, secret, uid string) *tdev1.ProviderStatus {
	return &tdev1.ProviderStatus{Provider: provider, DefaultAlgorithm: "AES256", Kms: &tdev1.KmsProviderStatus{
		KeyID: keyID, Endpoint: "https://kms.example.com", Region: "region-1", Auth: tdev1.KmsAuthStatus{
			Type: tdev1.KmsAuthEnvironmentSecret, CredentialSecretName: secret, CredentialSecretUID: uid,
			AccessKeyKey: "accessKey", SecretKeyKey: "secretKey",
		},
	}}
}
