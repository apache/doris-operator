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
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	ddcv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	tdev1 "github.com/apache/doris-operator/api/tde"
	hashutil "github.com/apache/doris-operator/pkg/common/utils/hash"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	mysqldriver "github.com/go-sql-driver/mysql"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	conditionMaterialReady          = "MaterialReady"
	conditionRotationOutcomeUnknown = "RotationOutcomeUnknown"
	conditionRotationRejected       = "RotationRejected"
	conditionConfigSyncPending      = "ConfigSyncPending"
	conditionReconcileBlocked       = "ReconcileBlocked"
	submittingGracePeriod           = time.Minute
	metadataQueryTimeout            = 15 * time.Second
)

var rotationSQLTimeout = time.Minute

type reconcileInput struct {
	object           client.Object
	spec             *tdev1.TDEConfig
	status           **tdev1.TDEStatus
	generation       int64
	statefulSetName  string
	connect          func(context.Context) (*mysql.DB, error)
	persistTDEStatus func(context.Context, *tdev1.TDEStatus) error
	activeConfigHash string
	activeFESpecHash string
}

func ReconcileDCR(ctx context.Context, c client.Client, dcr *dorisv1.DorisCluster) (ctrl.Result, error) {
	if dcr.Spec.TDE == nil {
		return ctrl.Result{}, nil
	}
	return reconcile(ctx, c, reconcileInput{
		object: dcr, spec: dcr.Spec.TDE, status: &dcr.Status.TDE, generation: dcr.Generation,
		statefulSetName:  dorisv1.GenerateComponentStatefulSetName(dcr, dorisv1.Component_FE),
		connect:          func(ctx context.Context) (*mysql.DB, error) { return connectDCR(ctx, c, dcr) },
		persistTDEStatus: func(ctx context.Context, status *tdev1.TDEStatus) error { return persistDCRStatus(ctx, c, dcr, status) },
		activeConfigHash: hashutil.HashObject(dcr.Spec.TDE), activeFESpecHash: hashutil.HashObject(dcr.Spec.FeSpec),
	})
}

func ReconcileDDC(ctx context.Context, c client.Client, ddc *ddcv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	if ddc.Spec.TDE == nil {
		return ctrl.Result{}, nil
	}
	return reconcile(ctx, c, reconcileInput{
		object: ddc, spec: ddc.Spec.TDE, status: &ddc.Status.TDE, generation: ddc.Generation,
		statefulSetName:  ddc.GetFEStatefulsetName(),
		connect:          func(ctx context.Context) (*mysql.DB, error) { return connectDDC(ctx, c, ddc) },
		persistTDEStatus: func(ctx context.Context, status *tdev1.TDEStatus) error { return persistDDCStatus(ctx, c, ddc, status) },
		activeConfigHash: hashutil.HashObject(ddc.Spec.TDE), activeFESpecHash: hashutil.HashObject(ddc.Spec.FeSpec),
	})
}

func reconcile(ctx context.Context, c client.Client, in reconcileInput) (ctrl.Result, error) {
	if in.spec.ManagementPolicy == tdev1.ManagementPolicyObserveOnly {
		if *in.status == nil {
			*in.status = &tdev1.TDEStatus{}
		}
		(*in.status).ObservedGeneration = in.generation
		(*in.status).State = tdev1.StateUnknown
		return ctrl.Result{}, nil
	}
	if errs := tdev1.Validate(in.spec); len(errs) != 0 {
		return ctrl.Result{}, errors.Join(errs...)
	}
	if *in.status == nil {
		*in.status = &tdev1.TDEStatus{}
	}
	status := *in.status
	status.ObservedGeneration = in.generation
	if in.spec.Recovery != nil && !tdev1.RotationOutcomeUnknown(status) &&
		(status.Operation == nil || status.Operation.Recovery == nil || status.Operation.Recovery.RequestID != in.spec.Recovery.RequestID) {
		setRecoveryBlocked(status, in.generation, "UnexpectedRecoveryRequest", "recovery is only valid for an existing RotationOutcomeUnknown operation")
		return ctrl.Result{}, nil
	}

	target, targetKey, err := resolveProvider(ctx, c, in.object.GetNamespace(), in.spec)
	if err != nil {
		setMaterialFailure(status, in.spec.Provider.Type, err)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	meta.RemoveStatusCondition(&status.Conditions, conditionMaterialReady)
	if rootRotationCancellationRequested(status, target, in.spec) {
		status.Operation.SQLState = tdev1.SQLNotApplied
		completeOperation(status)
		markActive(status, in)
		status.ObservedGeneration = in.generation
		meta.RemoveStatusCondition(&status.Conditions, conditionConfigSyncPending)
		meta.RemoveStatusCondition(&status.Conditions, conditionReconcileBlocked)
		return ctrl.Result{Requeue: true}, nil
	}

	ready, checks, err := workloadReady(ctx, c, in.object.GetNamespace(), in.statefulSetName, in.spec, status)
	status.FEChecks = checks
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if status.Current == nil {
		newOperation := ensureOperation(status, tdev1.OperationEnableTDE, "", in.generation, nil, target)
		status.State = tdev1.StateConfiguredUninitialized
		if newOperation {
			return ctrl.Result{Requeue: true}, nil
		}
		if !ready {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		initialized, err := metadataInitialized(ctx, in.connect)
		if err != nil || !initialized {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		status.MetadataInitialized = true
		status.Current = target.DeepCopy()
		completeOperation(status)
		markActive(status, in)
		return ctrl.Result{}, nil
	}

	sourceKey, err := validateCurrentProvider(ctx, c, in.object.GetNamespace(), status.Current)
	if err != nil {
		setMaterialFailure(status, status.Current.Provider, err)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if handled, result := reconcileRecovery(status, target, in.spec, ready, in.generation); handled {
		if status.State == tdev1.StateActive {
			markActive(status, in)
		}
		return result, nil
	}
	if err := validateTransition(status, target, in.spec); err != nil {
		status.State = tdev1.StateReconciling
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionReconcileBlocked, Status: metav1.ConditionTrue,
			Reason: "InvalidTransition", Message: err.Error(), ObservedGeneration: in.generation})
		return ctrl.Result{}, nil
	}
	meta.RemoveStatusCondition(&status.Conditions, conditionReconcileBlocked)

	if sameRootStatus(status.Current, target) {
		if !sameAuthStatus(status.Current, target) {
			requestID := ""
			if in.spec.CredentialRotation != nil {
				requestID = in.spec.CredentialRotation.RequestID
			}
			newOperation := ensureOperation(status, tdev1.OperationRotateKmsCredential, requestID, in.generation, status.Current, target)
			status.State = tdev1.StateReconciling
			if newOperation {
				return ctrl.Result{Requeue: true}, nil
			}
			if !ready {
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			status.Current = target.DeepCopy()
			completeOperation(status)
		}
		if status.Current.DefaultAlgorithm != target.DefaultAlgorithm {
			newOperation := ensureOperation(status, tdev1.OperationUpdateAlgorithm, "", in.generation, status.Current, target)
			status.State = tdev1.StateReconciling
			if newOperation {
				return ctrl.Result{Requeue: true}, nil
			}
			if !ready {
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			status.Current.DefaultAlgorithm = target.DefaultAlgorithm
			completeOperation(status)
		}
		markActive(status, in)
		status.ObservedGeneration = in.generation
		return ctrl.Result{}, nil
	}

	if sourceKey != nil && targetKey != nil && len(sourceKey) == len(targetKey) && subtle.ConstantTimeCompare(sourceKey, targetKey) == 1 {
		setMaterialFailure(status, target.Provider, fmt.Errorf("source and target Local root keys must differ"))
		return ctrl.Result{}, nil
	}

	requestID := in.spec.RootKeyRotation.RequestID
	newOperation := status.Operation == nil || status.Operation.Type != tdev1.OperationRotateRootKey || status.Operation.RequestID != requestID
	if newOperation {
		status.Operation = &tdev1.OperationStatus{
			Type: tdev1.OperationRotateRootKey, RequestID: requestID, SpecGeneration: in.generation,
			Stage: tdev1.StageRollingOutMaterial, SQLState: tdev1.SQLNotStarted,
			Source: status.Current.DeepCopy(), Target: target.DeepCopy(), StartedAt: metav1.Now(), LastTransitionTime: metav1.Now(),
		}
		recordRequestID(status, requestID)
		meta.RemoveStatusCondition(&status.Conditions, conditionRotationRejected)
		status.State = tdev1.StateReconciling
		return ctrl.Result{Requeue: true}, nil
	}
	status.State = tdev1.StateReconciling
	operation := status.Operation
	if operation.SQLState == tdev1.SQLSubmitting && !newOperation {
		remaining := submittingGraceRemaining(operation, time.Now())
		if remaining > 0 {
			return ctrl.Result{RequeueAfter: remaining}, nil
		}
		operation.SQLState = tdev1.SQLOutcomeUnknown
		operation.Stage = tdev1.StageFailed
		operation.LastTransitionTime = metav1.Now()
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionRotationOutcomeUnknown, Status: metav1.ConditionTrue,
			Reason: "OperatorRestartedWhileSubmitting", Message: "rotate SQL outcome is unknown; automatic retry is disabled", ObservedGeneration: in.generation})
		return ctrl.Result{}, nil
	}
	if operation.SQLState == tdev1.SQLOutcomeUnknown {
		return ctrl.Result{}, nil
	}
	if operation.SQLState == tdev1.SQLNotStarted {
		if !ready {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		db, err := in.connect(ctx)
		if err != nil {
			operation.Stage = tdev1.StageRollingOutMaterial
			operation.LastTransitionTime = metav1.Now()
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		defer db.Close()
		operation.Stage = tdev1.StageRotatingRootKey
		operation.SQLState = tdev1.SQLSubmitting
		operation.LastTransitionTime = metav1.Now()
		if err := in.persistTDEStatus(ctx, status.DeepCopy()); err != nil {
			return ctrl.Result{}, err
		}
		err = executeRotateSQL(ctx, db, operation.Source, operation.Target)
		if err != nil {
			if message, rejected := confirmedSQLRejection(err); rejected {
				operation.SQLState = tdev1.SQLRejected
				operation.Stage = tdev1.StageFailed
				operation.LastTransitionTime = metav1.Now()
				meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionRotationRejected, Status: metav1.ConditionTrue,
					Reason: "RotateSQLRejected", Message: message, ObservedGeneration: in.generation})
				return ctrl.Result{}, nil
			}
			operation.SQLState = tdev1.SQLOutcomeUnknown
			operation.Stage = tdev1.StageFailed
			operation.LastTransitionTime = metav1.Now()
			meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionRotationOutcomeUnknown, Status: metav1.ConditionTrue,
				Reason: "RotateSQLResultUnknown", Message: "rotate SQL did not return a confirmed success; automatic retry is disabled", ObservedGeneration: in.generation})
			return ctrl.Result{}, nil
		}
		operation.SQLState = tdev1.SQLApplied
		operation.Stage = tdev1.StageWaitingForFEReplay
		operation.LastTransitionTime = metav1.Now()
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionConfigSyncPending, Status: metav1.ConditionTrue,
			Reason: "WaitingForFEReplay", Message: "root key was rotated; waiting for all FE nodes to replay the committed journal", ObservedGeneration: in.generation})
		return ctrl.Result{Requeue: true}, nil
	}
	if operation.SQLState == tdev1.SQLApplied {
		if operation.Stage == tdev1.StageWaitingForFEReplay {
			db, err := in.connect(ctx)
			if err != nil {
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
			allReplayed, err := allFrontendsReplayed(ctx, db, operation)
			db.Close()
			if err != nil || !allReplayed {
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			operation.Stage = tdev1.StageSyncingConfiguration
			operation.LastTransitionTime = metav1.Now()
			meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionConfigSyncPending, Status: metav1.ConditionTrue,
				Reason: "FEReplayCompleted", Message: "all FE nodes replayed the root key rotation; waiting for target configuration rollout", ObservedGeneration: in.generation})
			return ctrl.Result{Requeue: true}, nil
		}
		if !ready {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		status.Current = operation.Target.DeepCopy()
		status.MetadataInitialized = true
		completeOperation(status)
		meta.RemoveStatusCondition(&status.Conditions, conditionConfigSyncPending)
		meta.RemoveStatusCondition(&status.Conditions, conditionRotationOutcomeUnknown)
		meta.RemoveStatusCondition(&status.Conditions, conditionRotationRejected)
		markActive(status, in)
	}
	return ctrl.Result{}, nil
}

func rootRotationCancellationRequested(status *tdev1.TDEStatus, target *tdev1.ProviderStatus, spec *tdev1.TDEConfig) bool {
	return status != nil && status.Current != nil && status.Operation != nil &&
		status.Operation.Type == tdev1.OperationRotateRootKey && status.Operation.Stage != tdev1.StageCompleted &&
		(status.Operation.SQLState == tdev1.SQLNotStarted || status.Operation.SQLState == tdev1.SQLRejected) && spec.RootKeyRotation == nil &&
		sameRootStatus(status.Current, target) && sameAuthStatus(status.Current, target) &&
		status.Current.DefaultAlgorithm == target.DefaultAlgorithm
}

func reconcileRecovery(status *tdev1.TDEStatus, target *tdev1.ProviderStatus, spec *tdev1.TDEConfig, ready bool, generation int64) (bool, ctrl.Result) {
	operation := status.Operation
	if operation == nil || operation.Type != tdev1.OperationRotateRootKey {
		return false, ctrl.Result{}
	}
	if tdev1.RotationOutcomeUnknown(status) {
		recovery := spec.Recovery
		if recovery == nil {
			return true, ctrl.Result{}
		}
		if recovery.RotationRequestID != operation.RequestID || recovery.RequestID == "" {
			setRecoveryBlocked(status, generation, "RecoveryRequestMismatch", "recovery request does not match the unknown root key rotation")
			return true, ctrl.Result{}
		}
		if operation.Recovery != nil && operation.Recovery.RequestID == recovery.RequestID {
			setRecoveryBlocked(status, generation, "RecoveryRequestReused", "recovery requestId was already used")
			return true, ctrl.Result{}
		}
		if tdev1.RequestIDUsed(status, recovery.RequestID) {
			setRecoveryBlocked(status, generation, "RecoveryRequestReused", "recovery requestId was already used")
			return true, ctrl.Result{}
		}
		switch recovery.Decision {
		case tdev1.RecoveryConfirmApplied:
			if !sameRootStatus(operation.Target, target) || !sameAuthStatus(operation.Target, target) {
				setRecoveryBlocked(status, generation, "RecoveryTargetMismatch", "ConfirmApplied requires the captured rotation target")
				return true, ctrl.Result{}
			}
			operation.SQLState = tdev1.SQLApplied
		case tdev1.RecoveryConfirmNotApplied:
			if !sameRootStatus(operation.Source, target) || !sameAuthStatus(operation.Source, target) {
				setRecoveryBlocked(status, generation, "RecoverySourceMismatch", "ConfirmNotApplied requires restoring the captured rotation source")
				return true, ctrl.Result{}
			}
			operation.SQLState = tdev1.SQLNotApplied
			status.Current = operation.Source.DeepCopy()
		default:
			setRecoveryBlocked(status, generation, "InvalidRecoveryDecision", "recovery decision must be ConfirmApplied or ConfirmNotApplied")
			return true, ctrl.Result{}
		}
		operation.Recovery = &tdev1.RecoveryStatus{RequestID: recovery.RequestID, Decision: recovery.Decision, ResolvedAt: metav1.Now()}
		recordRequestID(status, recovery.RequestID)
		operation.Stage = tdev1.StageSyncingConfiguration
		operation.LastTransitionTime = metav1.Now()
		status.State = tdev1.StateReconciling
		meta.RemoveStatusCondition(&status.Conditions, conditionRotationOutcomeUnknown)
		meta.RemoveStatusCondition(&status.Conditions, conditionReconcileBlocked)
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionConfigSyncPending, Status: metav1.ConditionTrue,
			Reason: "RotationOutcomeResolved", Message: "waiting for FE configuration rollout after manual recovery", ObservedGeneration: generation})
		return true, ctrl.Result{Requeue: true}
	}
	if operation.Recovery != nil && operation.Recovery.Decision == tdev1.RecoveryConfirmNotApplied &&
		operation.SQLState == tdev1.SQLNotApplied && operation.Stage == tdev1.StageSyncingConfiguration {
		status.State = tdev1.StateReconciling
		if !ready {
			return true, ctrl.Result{RequeueAfter: 5 * time.Second}
		}
		completeOperation(status)
		meta.RemoveStatusCondition(&status.Conditions, conditionConfigSyncPending)
		status.State = tdev1.StateActive
		return true, ctrl.Result{}
	}
	return false, ctrl.Result{}
}

func markActive(status *tdev1.TDEStatus, in reconcileInput) {
	status.State = tdev1.StateActive
	status.ActiveConfigHash = in.activeConfigHash
	status.ActiveFESpecHash = in.activeFESpecHash
}

func setRecoveryBlocked(status *tdev1.TDEStatus, generation int64, reason, message string) {
	status.State = tdev1.StateReconciling
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionReconcileBlocked, Status: metav1.ConditionTrue,
		Reason: reason, Message: message, ObservedGeneration: generation})
}

func submittingGraceRemaining(operation *tdev1.OperationStatus, now time.Time) time.Duration {
	return submittingGracePeriod - now.Sub(operation.LastTransitionTime.Time)
}

func confirmedSQLRejection(err error) (string, bool) {
	var serverError *mysqldriver.MySQLError
	if !errors.As(err, &serverError) {
		return "", false
	}
	return serverError.Message, true
}

func resolveProvider(ctx context.Context, c client.Client, namespace string, spec *tdev1.TDEConfig) (*tdev1.ProviderStatus, []byte, error) {
	uids := map[string]string{}
	var key []byte
	var accessKey, secretKey []byte
	switch spec.Provider.Type {
	case tdev1.ProviderLocal:
		ref := spec.Provider.Local.SecretKeyRef
		secret, decoded, err := getLocalKey(ctx, c, namespace, ref.Name, ref.Key, "")
		if err != nil {
			return nil, nil, err
		}
		uids[ref.Name] = string(secret.UID)
		key = decoded
	case tdev1.ProviderAwsKms, tdev1.ProviderAliyunKms:
		if spec.Provider.Kms.Auth.Type == tdev1.KmsAuthEnvironmentSecret {
			ref := spec.Provider.Kms.Auth.CredentialSecretRef
			secret, err := getImmutableSecret(ctx, c, namespace, ref.Name)
			if err != nil {
				return nil, nil, err
			}
			if len(secret.Data[ref.AccessKeyKey]) == 0 || len(secret.Data[ref.SecretKeyKey]) == 0 {
				return nil, nil, fmt.Errorf("KMS credential Secret %s is missing required keys", ref.Name)
			}
			uids[ref.Name] = string(secret.UID)
			accessKey, secretKey = secret.Data[ref.AccessKeyKey], secret.Data[ref.SecretKeyKey]
		}
	}
	provider := providerStatusFromSpec(spec, uids)
	if provider.Kms != nil && provider.Kms.Auth.Type == tdev1.KmsAuthEnvironmentSecret {
		if err := kmsReadinessCheck(ctx, provider.Provider, provider.Kms, accessKey, secretKey); err != nil {
			return nil, nil, err
		}
	}
	return provider, key, nil
}

func validateCurrentProvider(ctx context.Context, c client.Client, namespace string, current *tdev1.ProviderStatus) ([]byte, error) {
	if current == nil {
		return nil, fmt.Errorf("current TDE provider is missing")
	}
	if current.Provider == tdev1.ProviderLocal {
		if current.RootKeyRef == nil {
			return nil, fmt.Errorf("current Local key reference is missing")
		}
		_, key, err := getLocalKey(ctx, c, namespace, current.RootKeyRef.SecretName, current.RootKeyRef.Key, current.RootKeyRef.SecretUID)
		return key, err
	}
	if current.Kms != nil && current.Kms.Auth.Type == tdev1.KmsAuthEnvironmentSecret {
		secret, err := getImmutableSecret(ctx, c, namespace, current.Kms.Auth.CredentialSecretName)
		if err != nil {
			return nil, err
		}
		if string(secret.UID) != current.Kms.Auth.CredentialSecretUID {
			return nil, fmt.Errorf("current KMS credential Secret UID changed")
		}
		accessKey, secretKey := secret.Data[current.Kms.Auth.AccessKeyKey], secret.Data[current.Kms.Auth.SecretKeyKey]
		if len(accessKey) == 0 || len(secretKey) == 0 {
			return nil, fmt.Errorf("current KMS credential Secret is missing required keys")
		}
		if err := kmsReadinessCheck(ctx, current.Provider, current.Kms, accessKey, secretKey); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func getLocalKey(ctx context.Context, c client.Client, namespace, name, key, expectedUID string) (*corev1.Secret, []byte, error) {
	secret, err := getImmutableSecret(ctx, c, namespace, name)
	if err != nil {
		return nil, nil, err
	}
	if expectedUID != "" && string(secret.UID) != expectedUID {
		return nil, nil, fmt.Errorf("Local root key Secret %s UID changed", name)
	}
	file, ok := secret.Data[key]
	if !ok {
		return nil, nil, fmt.Errorf("Local root key Secret %s does not contain key %s", name, key)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(file)))
	if err != nil {
		return nil, nil, fmt.Errorf("Local root key Secret %s key %s is not valid Base64", name, key)
	}
	if len(decoded) != 16 && len(decoded) != 32 {
		return nil, nil, fmt.Errorf("Local root key must decode to 16 or 32 bytes")
	}
	return secret, decoded, nil
}

func getImmutableSecret(ctx context.Context, c client.Client, namespace, name string) (*corev1.Secret, error) {
	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &secret); err != nil {
		return nil, err
	}
	if secret.Immutable == nil || !*secret.Immutable {
		return nil, fmt.Errorf("Secret %s must be immutable", name)
	}
	return &secret, nil
}

func workloadReady(ctx context.Context, c client.Client, namespace, name string, spec *tdev1.TDEConfig, status *tdev1.TDEStatus) (bool, []tdev1.FECheck, error) {
	var sts appsv1.StatefulSet
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &sts); err != nil {
		return false, nil, err
	}
	desired := int32(1)
	if sts.Spec.Replicas != nil {
		desired = *sts.Spec.Replicas
	}
	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return false, nil, err
	}
	expectedTemplate := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "fe"}}}}
	ApplyPodOverlay(&expectedTemplate, "fe", spec, status)
	expectedMaterial := expectedTemplate.Annotations[materialAnnotation]
	expectedConfig := expectedTemplate.Annotations[configAnnotation]
	templateConsistent := sts.Spec.Template.Annotations[materialAnnotation] == expectedMaterial &&
		sts.Spec.Template.Annotations[configAnnotation] == expectedConfig
	var pods corev1.PodList
	if err := c.List(ctx, &pods, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return false, nil, err
	}
	sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].Name < pods.Items[j].Name })
	checks := make([]tdev1.FECheck, 0, len(pods.Items))
	for i := range pods.Items {
		pod := &pods.Items[i]
		ready := k8s.PodIsReady(&pod.Status)
		consistent := templateConsistent && pod.Labels[appsv1.ControllerRevisionHashLabelKey] == sts.Status.UpdateRevision &&
			pod.Annotations[materialAnnotation] == expectedMaterial && pod.Annotations[configAnnotation] == expectedConfig
		checks = append(checks, tdev1.FECheck{PodName: pod.Name, PodReady: ready, ConfigConsistent: consistent, MaterialReady: ready && consistent})
	}
	ready := templateConsistent && sts.Status.ObservedGeneration >= sts.Generation && sts.Status.CurrentRevision != "" && sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
		sts.Status.ReadyReplicas == desired && sts.Status.UpdatedReplicas == desired && int32(len(checks)) == desired
	return ready, checks, nil
}

func ensureOperation(status *tdev1.TDEStatus, operationType tdev1.OperationType, requestID string, generation int64, source, target *tdev1.ProviderStatus) bool {
	if status.Operation != nil && status.Operation.Type == operationType && status.Operation.RequestID == requestID && status.Operation.SpecGeneration == generation {
		return false
	}
	now := metav1.Now()
	status.Operation = &tdev1.OperationStatus{Type: operationType, RequestID: requestID, SpecGeneration: generation,
		Stage: tdev1.StageRollingOutMaterial, SQLState: tdev1.SQLNotStarted, Source: source.DeepCopy(), Target: target.DeepCopy(), StartedAt: now, LastTransitionTime: now}
	recordRequestID(status, requestID)
	return true
}

func recordRequestID(status *tdev1.TDEStatus, requestID string) {
	if status == nil || requestID == "" || tdev1.RequestIDUsed(status, requestID) {
		return
	}
	status.UsedRequestIDs = append(status.UsedRequestIDs, requestID)
}

func completeOperation(status *tdev1.TDEStatus) {
	if status.Operation == nil {
		return
	}
	status.Operation.Stage = tdev1.StageCompleted
	status.Operation.LastTransitionTime = metav1.Now()
}

func setMaterialFailure(status *tdev1.TDEStatus, provider tdev1.ProviderType, err error) {
	if provider == tdev1.ProviderLocal {
		status.State = tdev1.StateLocalKeyNotReady
	} else {
		status.State = tdev1.StateKmsNotReady
	}
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{Type: conditionMaterialReady, Status: metav1.ConditionFalse,
		Reason: "MaterialValidationFailed", Message: err.Error(), ObservedGeneration: status.ObservedGeneration})
}

func sameRootStatus(a, b *tdev1.ProviderStatus) bool {
	if a == nil || b == nil || a.Provider != b.Provider {
		return false
	}
	if a.Provider == tdev1.ProviderLocal {
		return a.RootKeyRef != nil && b.RootKeyRef != nil && *a.RootKeyRef == *b.RootKeyRef
	}
	return a.Kms != nil && b.Kms != nil && a.Kms.KeyID == b.Kms.KeyID && a.Kms.Endpoint == b.Kms.Endpoint && a.Kms.Region == b.Kms.Region
}

func sameAuthStatus(a, b *tdev1.ProviderStatus) bool {
	if a == nil || b == nil || a.Provider == tdev1.ProviderLocal || b.Provider == tdev1.ProviderLocal {
		return true
	}
	return a.Kms != nil && b.Kms != nil && reflect.DeepEqual(a.Kms.Auth, b.Kms.Auth)
}

func validateTransition(status *tdev1.TDEStatus, target *tdev1.ProviderStatus, spec *tdev1.TDEConfig) error {
	if status == nil || target == nil || spec == nil {
		return nil
	}
	if tdev1.RotationOutcomeUnknown(status) {
		return fmt.Errorf("the previous root key rotation outcome is unknown; resolve it before changing or retrying TDE configuration")
	}
	if operation := status.Operation; operation != nil && operation.Stage != tdev1.StageCompleted && operation.Stage != tdev1.StageFailed {
		if !reflect.DeepEqual(operation.Target, target) {
			return fmt.Errorf("spec.tde provider or algorithm cannot change while operation %q is in stage %q", operation.RequestID, operation.Stage)
		}
		switch operation.Type {
		case tdev1.OperationRotateRootKey:
			if spec.RootKeyRotation == nil || spec.RootKeyRotation.RequestID != operation.RequestID {
				return fmt.Errorf("spec.tde.rootKeyRotation.requestId cannot change while operation %q is in stage %q", operation.RequestID, operation.Stage)
			}
		case tdev1.OperationRotateKmsCredential:
			if spec.CredentialRotation == nil || spec.CredentialRotation.RequestID != operation.RequestID {
				return fmt.Errorf("spec.tde.credentialRotation.requestId cannot change while operation %q is in stage %q", operation.RequestID, operation.Stage)
			}
		}
	}
	if status.Current == nil {
		return nil
	}
	current := status.Current
	rootChanged := !sameRootStatus(current, target)
	authChanged := !sameAuthStatus(current, target)
	if (current.Provider == tdev1.ProviderAwsKms && target.Provider == tdev1.ProviderAliyunKms) ||
		(current.Provider == tdev1.ProviderAliyunKms && target.Provider == tdev1.ProviderAwsKms) {
		return fmt.Errorf("direct AwsKms/AliyunKms rotation is unsupported; rotate through Local using two requests")
	}
	if rootChanged && authChanged {
		return fmt.Errorf("KMS credentials and root key cannot change in the same update")
	}
	if rootChanged {
		if spec.RootKeyRotation == nil || spec.RootKeyRotation.RequestID == "" {
			return fmt.Errorf("changing the TDE root key requires spec.tde.rootKeyRotation.requestId")
		}
		if operation := status.Operation; operation != nil && operation.Type == tdev1.OperationRotateRootKey &&
			operation.RequestID == spec.RootKeyRotation.RequestID && !sameRootStatus(operation.Target, target) {
			return fmt.Errorf("spec.tde.rootKeyRotation.requestId %q was already used for a different target", spec.RootKeyRotation.RequestID)
		}
		if tdev1.RequestIDUsed(status, spec.RootKeyRotation.RequestID) &&
			(status.Operation == nil || status.Operation.Type != tdev1.OperationRotateRootKey || status.Operation.RequestID != spec.RootKeyRotation.RequestID) {
			return fmt.Errorf("spec.tde.rootKeyRotation.requestId %q was already used", spec.RootKeyRotation.RequestID)
		}
		return nil
	}
	if authChanged {
		if spec.CredentialRotation == nil || spec.CredentialRotation.RequestID == "" {
			return fmt.Errorf("changing KMS credentials requires spec.tde.credentialRotation.requestId")
		}
		if operation := status.Operation; operation != nil && operation.Type == tdev1.OperationRotateKmsCredential &&
			operation.RequestID == spec.CredentialRotation.RequestID && !sameAuthStatus(operation.Target, target) {
			return fmt.Errorf("spec.tde.credentialRotation.requestId %q was already used for different credentials", spec.CredentialRotation.RequestID)
		}
		if tdev1.RequestIDUsed(status, spec.CredentialRotation.RequestID) &&
			(status.Operation == nil || status.Operation.Type != tdev1.OperationRotateKmsCredential || status.Operation.RequestID != spec.CredentialRotation.RequestID) {
			return fmt.Errorf("spec.tde.credentialRotation.requestId %q was already used", spec.CredentialRotation.RequestID)
		}
	}
	return nil
}

func metadataInitialized(ctx context.Context, connect func(context.Context) (*mysql.DB, error)) (bool, error) {
	db, err := connect(ctx)
	if err != nil {
		return false, err
	}
	defer db.Close()
	var count int
	queryCtx, cancel := context.WithTimeout(ctx, metadataQueryTimeout)
	defer cancel()
	if err := db.GetContext(queryCtx, &count, "SELECT COUNT(*) FROM information_schema.encryption_keys"); err != nil {
		return false, err
	}
	return count > 0, nil
}

func executeRotateSQL(ctx context.Context, db *mysql.DB, source, target *tdev1.ProviderStatus) error {
	sqlCtx, cancel := context.WithTimeout(ctx, rotationSQLTimeout)
	defer cancel()
	_, err := db.ExecContext(sqlCtx, BuildRotateSQL(source, target))
	return err
}

func allFrontendsReplayed(ctx context.Context, db *mysql.DB, operation *tdev1.OperationStatus) (bool, error) {
	queryCtx, cancel := context.WithTimeout(ctx, metadataQueryTimeout)
	defer cancel()
	var frontends []*mysql.Frontend
	if err := db.DB.Unsafe().SelectContext(queryCtx, &frontends, "SHOW FRONTENDS"); err != nil {
		klog.Errorf("TDE replay gate failed to query SHOW FRONTENDS: %v", err)
		return false, err
	}
	if len(frontends) == 0 {
		return false, nil
	}
	return frontendsHaveReplayed(frontends, operation)
}

func frontendsHaveReplayed(frontends []*mysql.Frontend, operation *tdev1.OperationStatus) (bool, error) {
	if operation.CommitJournalID == "" {
		for _, frontend := range frontends {
			if frontend.IsMaster {
				if _, err := strconv.ParseInt(frontend.ReplayedJournalId, 10, 64); err != nil {
					return false, fmt.Errorf("parse Master FE replayed journal id: %w", err)
				}
				operation.CommitJournalID = frontend.ReplayedJournalId
				break
			}
		}
	}
	commitID, err := strconv.ParseInt(operation.CommitJournalID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parse root key rotation commit journal id: %w", err)
	}
	for _, frontend := range frontends {
		if !frontend.Join || !frontend.Alive {
			return false, nil
		}
		replayedID, err := strconv.ParseInt(frontend.ReplayedJournalId, 10, 64)
		if err != nil || replayedID < commitID {
			return false, nil
		}
	}
	return true, nil
}

func BuildRotateSQL(source, target *tdev1.ProviderStatus) string {
	properties := map[string]string{}
	switch target.Provider {
	case tdev1.ProviderLocal:
		properties["doris_tde_key_provider"] = "local"
		properties["doris_tde_key_new_key_file"] = target.RootKeyRef.ResolvedPath
	case tdev1.ProviderAwsKms:
		properties["doris_tde_key_provider"] = "aws_kms"
	case tdev1.ProviderAliyunKms:
		properties["doris_tde_key_provider"] = "aliyun_kms"
	}
	if target.Kms != nil {
		properties["doris_tde_key_id"] = target.Kms.KeyID
		properties["doris_tde_key_endpoint"] = target.Kms.Endpoint
		properties["doris_tde_key_region"] = target.Kms.Region
	}
	if source != nil && source.Provider == tdev1.ProviderLocal && source.RootKeyRef != nil {
		properties["doris_tde_key_original_key_file"] = source.RootKeyRef.ResolvedPath
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, sqlString(key)+" = "+sqlString(properties[key]))
	}
	return "ADMIN ROTATE TDE ROOT KEY PROPERTIES(" + strings.Join(parts, ", ") + ")"
}

func sqlString(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func persistDCRStatus(ctx context.Context, c client.Client, dcr *dorisv1.DorisCluster, status *tdev1.TDEStatus) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var latest dorisv1.DorisCluster
		if err := c.Get(ctx, client.ObjectKeyFromObject(dcr), &latest); err != nil {
			return err
		}
		latest.Status.TDE = status.DeepCopy()
		return c.Status().Update(ctx, &latest)
	})
}

func persistDDCStatus(ctx context.Context, c client.Client, ddc *ddcv1.DorisDisaggregatedCluster, status *tdev1.TDEStatus) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var latest ddcv1.DorisDisaggregatedCluster
		if err := c.Get(ctx, client.ObjectKeyFromObject(ddc), &latest); err != nil {
			return err
		}
		latest.Status.TDE = status.DeepCopy()
		return c.Status().Update(ctx, &latest)
	})
}

func connectDCR(ctx context.Context, c client.Client, dcr *dorisv1.DorisCluster) (*mysql.DB, error) {
	secret, _ := k8s.GetSecret(ctx, c, dcr.Namespace, dcr.Spec.AuthSecret)
	user, password := dorisv1.GetClusterSecret(dcr, secret)
	config, err := k8s.GetConfig(ctx, c, &dcr.Spec.FeSpec.ConfigMapInfo, dcr.Namespace, dorisv1.Component_FE)
	if err != nil {
		return nil, err
	}
	tlsConfig, tlsSecret, err := dcrTLS(ctx, c, dcr, config)
	if err != nil {
		return nil, err
	}
	return mysql.NewDorisMasterSqlDB(mysql.DBConfig{User: user, Password: password,
		Host: dorisv1.GenerateExternalServiceName(dcr, dorisv1.Component_FE) + "." + dcr.Namespace,
		Port: strconv.Itoa(int(resource.GetPort(config, resource.QUERY_PORT))), Database: "mysql"}, tlsConfig, tlsSecret)
}

func connectDDC(ctx context.Context, c client.Client, ddc *ddcv1.DorisDisaggregatedCluster) (*mysql.DB, error) {
	user, password := "root", ""
	if ddc.Spec.AuthSecret != "" {
		secret, _ := k8s.GetSecret(ctx, c, ddc.Namespace, ddc.Spec.AuthSecret)
		user, password = resource.GetDorisLoginInformation(secret)
	} else if ddc.Spec.AdminUser != nil {
		user, password = ddc.Spec.AdminUser.Name, ddc.Spec.AdminUser.Password
	}
	config, err := ddcFEConfig(ctx, c, ddc)
	if err != nil {
		return nil, err
	}
	tlsConfig, tlsSecret, err := ddcTLS(ctx, c, ddc, config)
	if err != nil {
		return nil, err
	}
	return mysql.NewDorisMasterSqlDB(mysql.DBConfig{User: user, Password: password, Host: ddc.GetFEVIPAddresss(),
		Port: strconv.Itoa(int(resource.GetPort(config, resource.QUERY_PORT))), Database: "mysql"}, tlsConfig, tlsSecret)
}

func ddcFEConfig(ctx context.Context, c client.Client, ddc *ddcv1.DorisDisaggregatedCluster) (map[string]interface{}, error) {
	configMaps := make([]*corev1.ConfigMap, 0, len(ddc.Spec.FeSpec.ConfigMaps))
	for _, ref := range ddc.Spec.FeSpec.ConfigMaps {
		var cm corev1.ConfigMap
		if err := c.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: ref.Name}, &cm); err != nil {
			return nil, err
		}
		configMaps = append(configMaps, &cm)
	}
	return resource.ResolveConfigMaps(configMaps, dorisv1.Component_FE)
}

func dcrTLS(ctx context.Context, c client.Client, dcr *dorisv1.DorisCluster, config map[string]interface{}) (*mysql.TLSConfig, *corev1.Secret, error) {
	if resource.GetString(config, resource.ENABLE_TLS_KEY) == "" {
		return nil, nil, nil
	}
	tlsConfig, dir := tlsConfigFromFE(config)
	for _, ref := range dcr.Spec.FeSpec.Secrets {
		if ref.MountPath == dir {
			secret, err := k8s.GetSecret(ctx, c, dcr.Namespace, ref.SecretName)
			return tlsConfig, secret, err
		}
	}
	return nil, nil, fmt.Errorf("FE TLS Secret mounted at %s was not found", dir)
}

func ddcTLS(ctx context.Context, c client.Client, ddc *ddcv1.DorisDisaggregatedCluster, config map[string]interface{}) (*mysql.TLSConfig, *corev1.Secret, error) {
	if resource.GetString(config, resource.ENABLE_TLS_KEY) == "" {
		return nil, nil, nil
	}
	tlsConfig, dir := tlsConfigFromFE(config)
	for _, ref := range ddc.Spec.FeSpec.Secrets {
		if ref.MountPath == dir {
			secret, err := k8s.GetSecret(ctx, c, ddc.Namespace, ref.SecretName)
			return tlsConfig, secret, err
		}
	}
	return nil, nil, fmt.Errorf("FE TLS Secret mounted at %s was not found", dir)
}

func tlsConfigFromFE(config map[string]interface{}) (*mysql.TLSConfig, string) {
	ca := resource.GetString(config, resource.TLS_CA_CERTIFICATE_PATH_KEY)
	cert := resource.GetString(config, resource.TLS_CERTIFICATE_PATH_KEY)
	key := resource.GetString(config, resource.TLS_PRIVATE_KEY_PATH_KEY)
	return &mysql.TLSConfig{CAFileName: path.Base(ca), ClientCertFileName: path.Base(cert), ClientKeyFileName: path.Base(key)}, path.Dir(ca)
}
