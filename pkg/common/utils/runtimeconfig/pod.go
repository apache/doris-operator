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

package runtimeconfig

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	RuntimeDir   = "/var/lib/doris/runtime-config"
	StoreDir     = ".doris-runtime-config"
	configVolume = "runtime-config-prepared"
	initName     = "prepare-runtime-config"
)

//go:embed prepare.sh
var prepareScript string

type storage struct {
	Volume    string `json:"volume"`
	Claim     string `json:"claim"`
	MountPath string `json:"mountPath"`
	SubPath   string `json:"subPath,omitempty"`
}

func component(name string) string {
	switch strings.ToLower(name) {
	case "fe":
		return "fe"
	case "be", "compute":
		return "be"
	case "metaservice":
		return "ms"
	}
	return ""
}

func supportedContainer(containers []corev1.Container) bool {
	for _, c := range containers {
		if component(c.Name) != "" {
			return true
		}
	}
	return false
}

// Prepare only changes the Pod template, never the user's ConfigMap or PVC
// templates. Auto mode leaves workloads without safe persistent storage alone.
func Prepare(ctx context.Context, c client.Client, recorder record.EventRecorder, owner client.Object, st *appsv1.StatefulSet, config map[string]interface{}) error {
	policy := owner.GetAnnotations()[PolicyAnnotation]
	if policy != Auto && policy != Enabled {
		return nil
	}
	var existing appsv1.StatefulSet
	err := c.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &existing)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	recorded := existing.Annotations[StorageAnnotation]
	// An automatically skipped workload must not be silently migrated later.
	if policy == Auto && err == nil && (recorded == "" || recorded == "unavailable") {
		if recorded != "" {
			setStorage(st, recorded)
		}
		return nil
	}
	for i := range st.Spec.Template.Spec.Containers {
		main := &st.Spec.Template.Spec.Containers[i]
		kind := component(main.Name)
		if kind == "" {
			continue
		}
		key := "custom_config_dir"
		if kind == "ms" {
			key = "custom_conf_path"
		}
		configured, _ := config[key].(string)
		if strings.TrimSpace(configured) != "" {
			// Explicit paths belong to the user. Verify durable backing rather than
			// replacing paths, creating directories, or changing their loading rules.
			target := strings.TrimSpace(configured)
			for _, m := range main.VolumeMounts {
				if persistent(st, m.Name) && !m.ReadOnly && within(target, m.MountPath) {
					setStorage(st, "external")
					return nil
				}
			}
			home := "/opt/apache-doris/" + kind
			defaultPath := home + "/conf"
			if kind == "ms" {
				defaultPath += "/doris_cloud.conf"
			}
			expanded := strings.ReplaceAll(target, "${DORIS_HOME}", home)
			if path.Clean(expanded) != defaultPath {
				return fmt.Errorf("%s: explicit %s=%s is not backed by a writable PVC", st.Name, key, target)
			}
		}
		if recorded == "external" {
			return fmt.Errorf("%s: removing a user-managed runtime configuration path requires migration", st.Name)
		}
		chosen, ok := selectStorage(st, main, kind, config, recorded)
		if !ok {
			if recorded != "" && recorded != "unavailable" {
				return fmt.Errorf("%s: previously selected runtime configuration PVC is missing or changed; migrate before changing storage", st.Name)
			}
			msg := fmt.Sprintf("%s: runtime configuration requires a writable log PVC, FE metadata PVC, or DCR BE storage PVC; cache roots and emptyDir are not safe", st.Name)
			if policy == Enabled {
				return fmt.Errorf("%s", msg)
			}
			setStorage(st, "unavailable")
			if recorder != nil {
				recorder.Event(owner, corev1.EventTypeWarning, "RuntimeConfigStorageUnavailable", msg)
			}
			return nil
		}
		if recorded != "" && recorded != "unavailable" && recorded != "external" {
			var old storage
			if err := json.Unmarshal([]byte(recorded), &old); err != nil || old != chosen {
				return fmt.Errorf("%s: runtime configuration storage changed; migrate it before changing volume mounts", st.Name)
			}
		}
		migratedReplicas := int32(0)
		var approval migration
		if policy == Enabled && json.Unmarshal([]byte(owner.GetAnnotations()[MigrationAnnotation]), &approval) == nil {
			migratedReplicas = approval.Workloads[st.Name].Replicas
		}
		if err := preparePod(st, main, kind, chosen, migratedReplicas); err != nil {
			return err
		}
		raw, _ := json.Marshal(chosen)
		setStorage(st, string(raw))
		return nil
	}
	return nil
}

func setStorage(st *appsv1.StatefulSet, value string) {
	if st.Annotations == nil {
		st.Annotations = map[string]string{}
	}
	st.Annotations[StorageAnnotation] = value
}

func within(target, root string) bool {
	return path.IsAbs(target) && (path.Clean(target) == path.Clean(root) || strings.HasPrefix(path.Clean(target), path.Clean(root)+"/"))
}

func persistent(st *appsv1.StatefulSet, name string) bool {
	for _, v := range st.Spec.Template.Spec.Volumes {
		if v.Name == name && v.PersistentVolumeClaim != nil && !v.PersistentVolumeClaim.ReadOnly {
			return true
		}
	}
	for _, v := range st.Spec.VolumeClaimTemplates {
		if v.Name == name {
			return true
		}
	}
	return false
}

func selectStorage(st *appsv1.StatefulSet, main *corev1.Container, kind string, config map[string]interface{}, recorded string) (storage, bool) {
	type candidate struct {
		storage
		rank int
	}
	var candidates []candidate
	var old storage
	_ = json.Unmarshal([]byte(recorded), &old)
	for _, m := range main.VolumeMounts {
		if !persistent(st, m.Name) || m.ReadOnly || m.SubPathExpr != "" {
			continue
		}
		s := storage{Volume: m.Name, Claim: claimIdentity(st, m.Name), MountPath: m.MountPath, SubPath: m.SubPath}
		if old.Volume != "" {
			if old == s && !cacheRoot(config, m.MountPath) {
				return s, true
			}
			continue
		}
		rank := 0
		if path.Base(m.MountPath) == "log" || equalsConfigPath(config, "log_dir", m.MountPath) || equalsConfigPath(config, "LOG_DIR", m.MountPath) {
			rank = 1
		}
		if kind == "fe" && (path.Base(m.MountPath) == "doris-meta" || equalsConfigPath(config, "meta_dir", m.MountPath)) {
			rank = 2
		}
		if strings.EqualFold(main.Name, "be") && (path.Base(m.MountPath) == "storage" || storageRoot(config, m.MountPath)) {
			rank = 3
		}
		// file_cache_path roots may be recursively removed, including custom directories.
		if cacheRoot(config, m.MountPath) {
			rank = 0
		}
		if rank != 0 {
			candidates = append(candidates, candidate{s, rank})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank < candidates[j].rank
		}
		return candidates[i].MountPath < candidates[j].MountPath
	})
	if len(candidates) == 0 {
		return storage{}, false
	}
	return candidates[0].storage, true
}

func equalsConfigPath(config map[string]interface{}, key, p string) bool {
	v, _ := config[key].(string)
	return v != "" && path.Clean(v) == path.Clean(p)
}
func storageRoot(config map[string]interface{}, p string) bool {
	v, _ := config["storage_root_path"].(string)
	for _, root := range strings.Split(v, ";") {
		if path.Clean(strings.Split(strings.TrimSpace(root), ",")[0]) == path.Clean(p) {
			return true
		}
	}
	return false
}
func cacheRoot(config map[string]interface{}, p string) bool {
	v, _ := config["file_cache_path"].(string)
	var roots []struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(v), &roots) == nil {
		for _, r := range roots {
			r.Path = strings.ReplaceAll(r.Path, "${DORIS_HOME}", "/opt/apache-doris/be")
			r.Path = strings.ReplaceAll(r.Path, "$DORIS_HOME", "/opt/apache-doris/be")
			if within(p, r.Path) || within(r.Path, p) {
				return true
			}
		}
	}
	return false
}

func preparePod(st *appsv1.StatefulSet, main *corev1.Container, kind string, s storage, migratedReplicas int32) error {
	pod := &st.Spec.Template.Spec
	for _, c := range pod.InitContainers {
		if c.Name == initName {
			return fmt.Errorf("%s: init container name %s is reserved", st.Name, initName)
		}
	}
	for _, m := range main.VolumeMounts {
		if path.Clean(m.MountPath) == RuntimeDir {
			return fmt.Errorf("%s: mount path %s is reserved; configure an explicit custom path to manage it yourself", st.Name, RuntimeDir)
		}
	}
	home := "/opt/apache-doris/" + kind
	base := kind + ".conf"
	if kind == "ms" {
		base = "doris_cloud.conf"
	}
	source := "/etc/doris"
	envKey := "CONFIGMAP_MOUNT_PATH"
	if kind == "ms" {
		envKey = "CONFIGMAP_PATH"
	}
	for _, e := range main.Env {
		if e.Name == envKey && e.Value != "" {
			source = e.Value
		}
	}
	for _, v := range pod.Volumes {
		if v.Name == configVolume {
			return fmt.Errorf("%s: volume name %s is reserved", st.Name, configVolume)
		}
	}
	init := corev1.Container{
		Name: initName, Image: main.Image, ImagePullPolicy: main.ImagePullPolicy,
		Command: []string{"/bin/bash", "-ec", prepareScript},
		Env: []corev1.EnvVar{
			{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
			{Name: "RUNTIME_COMPONENT", Value: kind}, {Name: "RUNTIME_BASE", Value: base}, {Name: "RUNTIME_HOME", Value: home},
			{Name: "RUNTIME_MIGRATED_REPLICAS", Value: fmt.Sprint(migratedReplicas)},
		},
		SecurityContext: main.SecurityContext, Resources: main.Resources,
		VolumeMounts: []corev1.VolumeMount{
			{Name: s.Volume, MountPath: "/runtime-store", SubPath: s.SubPath},
			{Name: configVolume, MountPath: "/runtime-prepared"},
		},
	}
	foundSource := false
	for i := range main.VolumeMounts {
		m := &main.VolumeMounts[i]
		if path.Clean(m.MountPath) == path.Clean(source) {
			src := *m
			src.MountPath = "/runtime-source"
			src.ReadOnly = true
			init.VolumeMounts = append(init.VolumeMounts, src)
			m.Name = configVolume
			m.ReadOnly = false
			m.SubPath = ""
			m.SubPathExpr = ""
			foundSource = true
		} else if path.Clean(m.MountPath) == home+"/conf" {
			return fmt.Errorf("%s: direct conf mounts require an explicit PVC-backed custom configuration path", st.Name)
		}
	}
	if !foundSource {
		main.VolumeMounts = append(main.VolumeMounts, corev1.VolumeMount{Name: configVolume, MountPath: source})
	}
	pod.Volumes = append(pod.Volumes, corev1.Volume{Name: configVolume, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
	main.VolumeMounts = append(main.VolumeMounts, corev1.VolumeMount{Name: s.Volume, MountPath: RuntimeDir, SubPathExpr: path.Join(s.SubPath, StoreDir, "$(POD_NAME)")})
	// The initializer must not run the image's normal entrypoint.
	pod.InitContainers = append(pod.InitContainers, init)
	return nil
}

func claimIdentity(st *appsv1.StatefulSet, name string) string {
	for _, v := range st.Spec.VolumeClaimTemplates {
		if v.Name == name {
			return "template:" + name
		}
	}
	for _, v := range st.Spec.Template.Spec.Volumes {
		if v.Name == name && v.PersistentVolumeClaim != nil {
			return "claim:" + v.PersistentVolumeClaim.ClaimName
		}
	}
	return ""
}
