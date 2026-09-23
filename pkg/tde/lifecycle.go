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

	ddcv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	tdev1 "github.com/apache/doris-operator/api/tde"
	hashutil "github.com/apache/doris-operator/pkg/common/utils/hash"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnforceDCRFELifecycleGate(ctx context.Context, c client.Client, dcr *dorisv1.DorisCluster) (bool, error) {
	if !feLifecycleChangeBlocked(dcr.Spec.TDE, dcr.Spec.FeSpec, dcr.Status.TDE) {
		return false, nil
	}
	status := dcr.Status.TDE.DeepCopy()
	metaSetLifecycleBlocked(status, dcr.Generation)
	dcr.Status.TDE = status
	return true, persistDCRStatus(ctx, c, dcr, status)
}

func EnforceDDCFELifecycleGate(ctx context.Context, c client.Client, ddc *ddcv1.DorisDisaggregatedCluster) (bool, error) {
	if !feLifecycleChangeBlocked(ddc.Spec.TDE, ddc.Spec.FeSpec, ddc.Status.TDE) {
		return false, nil
	}
	status := ddc.Status.TDE.DeepCopy()
	metaSetLifecycleBlocked(status, ddc.Generation)
	ddc.Status.TDE = status
	return true, persistDDCStatus(ctx, c, ddc, status)
}

func feLifecycleChangeBlocked(config *tdev1.TDEConfig, feSpec interface{}, status *tdev1.TDEStatus) bool {
	if status == nil || status.Current == nil || status.ActiveConfigHash == "" || status.ActiveFESpecHash == "" {
		return false
	}
	if hashutil.HashObject(feSpec) == status.ActiveFESpecHash {
		return false
	}
	return tdev1.BlocksFELifecycle(status) || hashutil.HashObject(config) != status.ActiveConfigHash
}

func metaSetLifecycleBlocked(status *tdev1.TDEStatus, generation int64) {
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               conditionReconcileBlocked,
		Status:             metav1.ConditionTrue,
		Reason:             "FELifecycleChangeBlocked",
		Message:            "spec.feSpec cannot change while a TDE operation or configuration sync is pending, or in the same update as spec.tde",
		ObservedGeneration: generation,
	})
}
