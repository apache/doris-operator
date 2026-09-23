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
	"fmt"
	"reflect"
	"sort"
	"strings"

	ddcv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	tdev1 "github.com/apache/doris-operator/api/tde"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	configMountPath = "/etc/doris"
	feConfigKey     = "fe.conf"
)

var managedConfigKeys = map[string]struct{}{
	"doris_tde_key_provider":                        {},
	"doris_tde_root_key_file":                       {},
	"doris_tde_key_id":                              {},
	"doris_tde_key_endpoint":                        {},
	"doris_tde_key_region":                          {},
	"doris_tde_algorithm":                           {},
	"doris_tde_rotate_master_key_interval_ms":       {},
	"doris_tde_check_rotate_master_key_interval_ms": {},
}

func PrepareDCRConfig(ctx context.Context, c client.Client, dcr *dorisv1.DorisCluster) (*dorisv1.DorisCluster, error) {
	if dcr.Spec.TDE == nil || dcr.Spec.TDE.ManagementPolicy != tdev1.ManagementPolicyManaged || dcr.Spec.FeSpec == nil {
		return dcr, nil
	}
	working := dcr.DeepCopy()
	baseName, sourceIndex, err := dcrCoreConfig(working.Spec.FeSpec.ConfigMapInfo)
	if err != nil {
		return nil, err
	}
	effectiveName := effectiveConfigMapName(dcr.Name, "dcr")
	if err := ensureEffectiveConfigMap(ctx, c, dcr.Namespace, baseName, effectiveName,
		metav1.NewControllerRef(dcr, schema.GroupVersionKind{Group: dorisv1.GroupVersion.Group, Version: dorisv1.GroupVersion.Version, Kind: "DorisCluster"}),
		working.Spec.TDE, working.Status.TDE); err != nil {
		return nil, err
	}
	if sourceIndex < 0 {
		working.Spec.FeSpec.ConfigMapInfo.ConfigMapName = effectiveName
	} else {
		working.Spec.FeSpec.ConfigMapInfo.ConfigMaps[sourceIndex].ConfigMapName = effectiveName
	}
	return working, nil
}

func PrepareDDCConfig(ctx context.Context, c client.Client, ddc *ddcv1.DorisDisaggregatedCluster) (*ddcv1.DorisDisaggregatedCluster, error) {
	if ddc.Spec.TDE == nil || ddc.Spec.TDE.ManagementPolicy != tdev1.ManagementPolicyManaged {
		return ddc, nil
	}
	working := ddc.DeepCopy()
	baseName, index, err := ddcCoreConfig(working.Spec.FeSpec.ConfigMaps)
	if err != nil {
		return nil, err
	}
	effectiveName := effectiveConfigMapName(ddc.Name, "ddc")
	if err := ensureEffectiveConfigMap(ctx, c, ddc.Namespace, baseName, effectiveName,
		metav1.NewControllerRef(ddc, schema.GroupVersionKind{Group: ddcv1.GroupVersion.Group, Version: ddcv1.GroupVersion.Version, Kind: "DorisDisaggregatedCluster"}),
		working.Spec.TDE, working.Status.TDE); err != nil {
		return nil, err
	}
	working.Spec.FeSpec.ConfigMaps[index].Name = effectiveName
	return working, nil
}

func ensureEffectiveConfigMap(ctx context.Context, c client.Client, namespace, baseName, effectiveName string,
	owner *metav1.OwnerReference, spec *tdev1.TDEConfig, status *tdev1.TDEStatus) error {
	var base corev1.ConfigMap
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: baseName}, &base); err != nil {
		return fmt.Errorf("get FE configmap %s: %w", baseName, err)
	}
	baseConfig, ok := base.Data[feConfigKey]
	if !ok {
		return fmt.Errorf("FE configmap %s does not contain %s", baseName, feConfigKey)
	}
	provider := providerForRuntime(spec, status)
	data := make(map[string]string, len(base.Data))
	for key, value := range base.Data {
		data[key] = value
	}
	data[feConfigKey] = mergeFEConfig(baseConfig, provider, spec)
	binaryData := make(map[string][]byte, len(base.BinaryData))
	for key, value := range base.BinaryData {
		binaryData[key] = append([]byte(nil), value...)
	}

	var current corev1.ConfigMap
	key := types.NamespacedName{Namespace: namespace, Name: effectiveName}
	err := c.Get(ctx, key, &current)
	if apierrors.IsNotFound(err) {
		current = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: effectiveName, Namespace: namespace, OwnerReferences: []metav1.OwnerReference{*owner}}}
		current.Data, current.BinaryData = data, binaryData
		return c.Create(ctx, &current)
	}
	if err != nil {
		return err
	}
	for i := range current.OwnerReferences {
		if current.OwnerReferences[i].Controller != nil && *current.OwnerReferences[i].Controller && current.OwnerReferences[i].UID != owner.UID {
			return fmt.Errorf("effective FE configmap %s is controlled by another object", effectiveName)
		}
	}
	if reflect.DeepEqual(current.Data, data) && reflect.DeepEqual(current.BinaryData, binaryData) && len(current.OwnerReferences) > 0 {
		return nil
	}
	current.Data, current.BinaryData = data, binaryData
	if len(current.OwnerReferences) == 0 {
		current.OwnerReferences = []metav1.OwnerReference{*owner}
	}
	return c.Update(ctx, &current)
}

func dcrCoreConfig(info dorisv1.ConfigMapInfo) (string, int, error) {
	if info.ConfigMapName != "" {
		return info.ConfigMapName, -1, nil
	}
	for i := range info.ConfigMaps {
		if info.ConfigMaps[i].MountPath == "" || info.ConfigMaps[i].MountPath == configMountPath {
			return info.ConfigMaps[i].ConfigMapName, i, nil
		}
	}
	return "", -1, fmt.Errorf("FE configmap mounted at %s is not configured", configMountPath)
}

func ddcCoreConfig(configMaps []ddcv1.ConfigMap) (string, int, error) {
	for i := range configMaps {
		if configMaps[i].MountPath == "" || configMaps[i].MountPath == configMountPath {
			return configMaps[i].Name, i, nil
		}
	}
	return "", -1, fmt.Errorf("FE configmap mounted at %s is not configured", configMountPath)
}

func effectiveConfigMapName(clusterName, clusterKind string) string {
	suffix := "-fe-tde-" + clusterKind
	if len(clusterName)+len(suffix) <= 63 {
		return clusterName + suffix
	}
	return strings.TrimRight(clusterName[:63-len(suffix)], "-") + suffix
}

func providerForRuntime(spec *tdev1.TDEConfig, status *tdev1.TDEStatus) *tdev1.ProviderStatus {
	if status != nil && status.Current == nil && status.Operation != nil && status.Operation.Type == tdev1.OperationEnableTDE &&
		status.Operation.Target != nil {
		return status.Operation.Target.DeepCopy()
	}
	if status != nil && status.Operation != nil && status.Operation.Type == tdev1.OperationRotateKmsCredential &&
		status.Operation.Stage != tdev1.StageCompleted && status.Operation.Target != nil {
		return status.Operation.Target.DeepCopy()
	}
	if status != nil && status.Operation != nil && status.Operation.Type == tdev1.OperationRotateRootKey {
		if status.Operation.SQLState == tdev1.SQLApplied && status.Operation.Stage != tdev1.StageWaitingForFEReplay && status.Operation.Target != nil {
			return status.Operation.Target.DeepCopy()
		}
		if status.Operation.Source != nil {
			return status.Operation.Source.DeepCopy()
		}
	}
	if status != nil && status.Current != nil {
		if !sameRootSpec(status.Current, spec) {
			return status.Current.DeepCopy()
		}
		current := status.Current.DeepCopy()
		current.DefaultAlgorithm = spec.DefaultAlgorithm
		return current
	}
	return providerStatusFromSpec(spec, nil)
}

func sameRootSpec(current *tdev1.ProviderStatus, spec *tdev1.TDEConfig) bool {
	if current == nil || spec == nil || current.Provider != spec.Provider.Type {
		return false
	}
	if current.Provider == tdev1.ProviderLocal {
		return current.RootKeyRef != nil && spec.Provider.Local != nil &&
			current.RootKeyRef.SecretName == spec.Provider.Local.SecretKeyRef.Name && current.RootKeyRef.Key == spec.Provider.Local.SecretKeyRef.Key
	}
	return current.Kms != nil && spec.Provider.Kms != nil && current.Kms.KeyID == spec.Provider.Kms.KeyID &&
		current.Kms.Endpoint == spec.Provider.Kms.Endpoint && current.Kms.Region == spec.Provider.Kms.Region
}

func mergeFEConfig(base string, provider *tdev1.ProviderStatus, spec *tdev1.TDEConfig) string {
	lines := strings.Split(base, "\n")
	out := make([]string, 0, len(lines)+8)
	for _, line := range lines {
		key := strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
		if _, managed := managedConfigKeys[key]; !managed {
			out = append(out, line)
		}
	}
	values := map[string]string{
		"doris_tde_algorithm": provider.DefaultAlgorithm,
	}
	switch provider.Provider {
	case tdev1.ProviderLocal:
		values["doris_tde_key_provider"] = "local"
		values["doris_tde_root_key_file"] = provider.RootKeyRef.ResolvedPath
	case tdev1.ProviderAwsKms:
		values["doris_tde_key_provider"] = "aws_kms"
	case tdev1.ProviderAliyunKms:
		values["doris_tde_key_provider"] = "aliyun_kms"
	}
	if provider.Kms != nil {
		values["doris_tde_key_id"] = provider.Kms.KeyID
		values["doris_tde_key_endpoint"] = provider.Kms.Endpoint
		values["doris_tde_key_region"] = provider.Kms.Region
	}
	if spec.MasterKeyRotation != nil {
		values["doris_tde_rotate_master_key_interval_ms"] = fmt.Sprintf("%d", spec.MasterKeyRotation.RotateIntervalMs)
		values["doris_tde_check_rotate_master_key_interval_ms"] = fmt.Sprintf("%d", spec.MasterKeyRotation.CheckIntervalMs)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(out) > 0 && out[len(out)-1] != "" {
		out = append(out, "")
	}
	out = append(out, "# Managed by doris-operator from spec.tde.")
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return strings.Join(out, "\n")
}
