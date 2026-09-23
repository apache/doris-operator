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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	tdev1 "github.com/apache/doris-operator/api/tde"
	corev1 "k8s.io/api/core/v1"
)

const (
	keyMountBase       = "/etc/selectdb/tde/keys"
	accessKeyEnv       = "DORIS_TDE_AK"
	secretKeyEnv       = "DORIS_TDE_SK"
	materialAnnotation = "selectdb.com/tde-material"
	configAnnotation   = "selectdb.com/tde-config-hash"
)

func ApplyPodOverlay(template *corev1.PodTemplateSpec, containerName string, spec *tdev1.TDEConfig, status *tdev1.TDEStatus) {
	if spec == nil || spec.ManagementPolicy != tdev1.ManagementPolicyManaged {
		return
	}
	providers := providersForPod(spec, status)
	var materialVersions []string
	for _, provider := range providers {
		if provider == nil || provider.Provider != tdev1.ProviderLocal || provider.RootKeyRef == nil {
			continue
		}
		volumeName := localVolumeName(provider.RootKeyRef.SecretName, provider.RootKeyRef.Key)
		if !hasVolume(template.Spec.Volumes, volumeName) {
			mode := int32(0440)
			template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
					SecretName: provider.RootKeyRef.SecretName,
					Items:      []corev1.KeyToPath{{Key: provider.RootKeyRef.Key, Path: "root.key", Mode: &mode}},
				}},
			})
		}
		for i := range template.Spec.Containers {
			if template.Spec.Containers[i].Name != containerName {
				continue
			}
			if !hasMount(template.Spec.Containers[i].VolumeMounts, volumeName) {
				template.Spec.Containers[i].VolumeMounts = append(template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name: volumeName, MountPath: path.Dir(provider.RootKeyRef.ResolvedPath), ReadOnly: true,
				})
			}
		}
		materialVersions = append(materialVersions, provider.RootKeyRef.SecretName+":"+provider.RootKeyRef.SecretUID)
	}
	active := providerForRuntime(spec, status)
	for _, provider := range providers {
		if provider != nil && provider.Provider != tdev1.ProviderLocal {
			active = provider
		}
	}
	if active != nil && active.Kms != nil && active.Kms.Auth.Type == tdev1.KmsAuthEnvironmentSecret {
		for i := range template.Spec.Containers {
			if template.Spec.Containers[i].Name != containerName {
				continue
			}
			container := &template.Spec.Containers[i]
			container.Env = removeEnv(container.Env, accessKeyEnv, secretKeyEnv)
			container.Env = append(container.Env,
				corev1.EnvVar{Name: accessKeyEnv, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: active.Kms.Auth.CredentialSecretName}, Key: active.Kms.Auth.AccessKeyKey,
				}}},
				corev1.EnvVar{Name: secretKeyEnv, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: active.Kms.Auth.CredentialSecretName}, Key: active.Kms.Auth.SecretKeyKey,
				}}},
			)
		}
		materialVersions = append(materialVersions, active.Kms.Auth.CredentialSecretName+":"+active.Kms.Auth.CredentialSecretUID)
	}
	if template.Annotations == nil {
		template.Annotations = map[string]string{}
	}
	template.Annotations[materialAnnotation] = strings.Join(materialVersions, ",")
	template.Annotations[configAnnotation] = configID(providerForRuntime(spec, status), spec.MasterKeyRotation)
}

func configID(provider *tdev1.ProviderStatus, rotation *tdev1.MasterKeyRotationSpec) string {
	data, _ := json.Marshal(struct {
		Provider *tdev1.ProviderStatus        `json:"provider,omitempty"`
		Rotation *tdev1.MasterKeyRotationSpec `json:"rotation,omitempty"`
	}{Provider: provider, Rotation: rotation})
	return shortHash(string(data))
}

func providersForPod(spec *tdev1.TDEConfig, status *tdev1.TDEStatus) []*tdev1.ProviderStatus {
	if status != nil && status.Operation != nil && status.Operation.Type == tdev1.OperationRotateRootKey && status.Operation.Stage != tdev1.StageCompleted {
		if status.Operation.SQLState == tdev1.SQLApplied {
			if status.Operation.Stage == tdev1.StageWaitingForFEReplay {
				return []*tdev1.ProviderStatus{status.Operation.Source, status.Operation.Target}
			}
			return []*tdev1.ProviderStatus{status.Operation.Target}
		}
		if status.Operation.SQLState == tdev1.SQLNotApplied {
			return []*tdev1.ProviderStatus{status.Operation.Source}
		}
		return []*tdev1.ProviderStatus{status.Operation.Source, status.Operation.Target}
	}
	return []*tdev1.ProviderStatus{providerForRuntime(spec, status)}
}

func providerStatusFromSpec(spec *tdev1.TDEConfig, secretUIDs map[string]string) *tdev1.ProviderStatus {
	if spec == nil {
		return nil
	}
	status := &tdev1.ProviderStatus{Provider: spec.Provider.Type, DefaultAlgorithm: spec.DefaultAlgorithm}
	switch spec.Provider.Type {
	case tdev1.ProviderLocal:
		if spec.Provider.Local == nil {
			return status
		}
		ref := spec.Provider.Local.SecretKeyRef
		status.RootKeyRef = &tdev1.RootKeyRefStatus{
			SecretName: ref.Name, Key: ref.Key, ResolvedPath: localKeyPath(ref.Name, ref.Key), SecretUID: secretUIDs[ref.Name],
		}
	case tdev1.ProviderAwsKms, tdev1.ProviderAliyunKms:
		if spec.Provider.Kms == nil {
			return status
		}
		kms := spec.Provider.Kms
		status.Kms = &tdev1.KmsProviderStatus{KeyID: kms.KeyID, Endpoint: kms.Endpoint, Region: kms.Region, Auth: tdev1.KmsAuthStatus{Type: kms.Auth.Type}}
		if kms.Auth.CredentialSecretRef != nil {
			ref := kms.Auth.CredentialSecretRef
			status.Kms.Auth.CredentialSecretName = ref.Name
			status.Kms.Auth.CredentialSecretUID = secretUIDs[ref.Name]
			status.Kms.Auth.AccessKeyKey = ref.AccessKeyKey
			status.Kms.Auth.SecretKeyKey = ref.SecretKeyKey
		}
	}
	return status
}

func localKeyPath(secretName, key string) string {
	return path.Join(keyMountBase, secretName+"-"+shortHash(key), "root.key")
}

func localVolumeName(secretName, key string) string {
	name := "tde-key-" + strings.ReplaceAll(secretName, ".", "-")
	if len(name) > 48 {
		name = name[:48]
	}
	return strings.TrimRight(name, "-") + "-" + shortHash(key)
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:4])
}

func hasVolume(volumes []corev1.Volume, name string) bool {
	for i := range volumes {
		if volumes[i].Name == name {
			return true
		}
	}
	return false
}

func hasMount(mounts []corev1.VolumeMount, name string) bool {
	for i := range mounts {
		if mounts[i].Name == name {
			return true
		}
	}
	return false
}

func removeEnv(envs []corev1.EnvVar, names ...string) []corev1.EnvVar {
	remove := make(map[string]struct{}, len(names))
	for _, name := range names {
		remove[name] = struct{}{}
	}
	result := envs[:0]
	for _, env := range envs {
		if _, ok := remove[env.Name]; !ok {
			result = append(result, env)
		}
	}
	return result
}

func materialID(provider *tdev1.ProviderStatus) string {
	if provider == nil {
		return ""
	}
	if provider.RootKeyRef != nil {
		return fmt.Sprintf("%s/%s@%s", provider.RootKeyRef.SecretName, provider.RootKeyRef.Key, provider.RootKeyRef.SecretUID)
	}
	if provider.Kms != nil {
		return fmt.Sprintf("%s/%s/%s", provider.Kms.KeyID, provider.Kms.Region, provider.Kms.Auth.CredentialSecretUID)
	}
	return string(provider.Provider)
}
