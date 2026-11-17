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

package feheap

import (
	_ "embed"
	"fmt"
	"path"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	initName     = "prepare-fe-heap"
	configVolume = "fe-heap-prepared"
	feHome       = "/opt/apache-doris/fe"
)

//go:embed prepare.sh
var prepareScript string

// Prepare stages heap defaults before optional runtime configuration preparation.
// The latter consumes this directory and retains its own custom_config_dir changes.
// Heap adaptation also works independently, without persistent storage.
func Prepare(owner client.Object, st *appsv1.StatefulSet) error {
	if owner.GetAnnotations()[PolicyAnnotation] != Auto {
		return nil
	}
	pod := &st.Spec.Template.Spec
	for i := range pod.Containers {
		main := &pod.Containers[i]
		if main.Name != "fe" {
			continue
		}
		limit := main.Resources.Limits.Memory()
		if limit.Sign() <= 0 {
			return nil
		}
		halfMiB := limit.Value() / (2 * 1024 * 1024)
		if halfMiB < 1 {
			return fmt.Errorf("FE memory limit is too small to allocate a heap")
		}
		preserve8, preserve17 := false, false
		source := "/etc/doris"
		for _, env := range main.Env {
			switch env.Name {
			case "JAVA_OPTS":
				preserve8 = true
			case "JAVA_OPTS_FOR_JDK_17":
				preserve17 = true
			case "CONFIGMAP_MOUNT_PATH":
				// Dynamic/custom config locations are user-managed.
				if env.ValueFrom != nil {
					return nil
				}
				if env.Value != "" {
					source = env.Value
				}
			case "DORIS_ROOT":
				if env.ValueFrom != nil || (env.Value != "" && path.Clean(env.Value) != path.Dir(feHome)) {
					return nil
				}
			case "DORIS_HOME":
				if env.ValueFrom != nil || (env.Value != "" && path.Clean(env.Value) != feHome) {
					return nil
				}
			}
		}
		if preserve8 && preserve17 {
			return nil
		}
		for _, mount := range main.VolumeMounts {
			// Never rewrite directly mounted user-managed FE configuration.
			if mount.MountPath == "/opt/apache-doris" || mount.MountPath == feHome ||
				mount.MountPath == feHome+"/conf" || strings.HasPrefix(mount.MountPath, feHome+"/conf/") {
				return nil
			}
		}
		for _, c := range pod.InitContainers {
			if c.Name == initName {
				return fmt.Errorf("init container name %s is reserved", initName)
			}
		}
		for _, v := range pod.Volumes {
			if v.Name == configVolume {
				return fmt.Errorf("volume name %s is reserved", configVolume)
			}
		}
		init := corev1.Container{
			Name: initName, Image: main.Image, ImagePullPolicy: main.ImagePullPolicy,
			Command: []string{"/bin/bash", "-ec", prepareScript},
			Env: []corev1.EnvVar{
				{Name: "FE_HEAP_HOME", Value: feHome},
				{Name: "FE_HEAP_SOURCE", Value: "/heap-source"},
				{Name: "FE_HEAP_OUTPUT", Value: "/heap-prepared"},
				{Name: "FE_HEAP_JDK8_MIB", Value: strconv.FormatInt(halfMiB, 10)},
				{Name: "FE_HEAP_PRESERVE_JDK8", Value: strconv.FormatBool(preserve8)},
				{Name: "FE_HEAP_PRESERVE_JDK17", Value: strconv.FormatBool(preserve17)},
			},
			Resources: main.Resources, SecurityContext: main.SecurityContext,
			VolumeMounts: []corev1.VolumeMount{{Name: configVolume, MountPath: "/heap-prepared"}},
		}
		found := false
		for j := range main.VolumeMounts {
			mount := &main.VolumeMounts[j]
			if path.Clean(mount.MountPath) != path.Clean(source) {
				continue
			}
			original := *mount
			original.MountPath = "/heap-source"
			original.ReadOnly = true
			init.VolumeMounts = append(init.VolumeMounts, original)
			mount.Name = configVolume
			mount.ReadOnly = false
			mount.SubPath = ""
			mount.SubPathExpr = ""
			found = true
		}
		if !found {
			main.VolumeMounts = append(main.VolumeMounts, corev1.VolumeMount{Name: configVolume, MountPath: source})
		}
		pod.Volumes = append(pod.Volumes, corev1.Volume{Name: configVolume, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
		pod.InitContainers = append(pod.InitContainers, init)
		return nil
	}
	return nil
}
