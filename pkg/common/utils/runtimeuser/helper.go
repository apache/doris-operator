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

package runtimeuser

import (
	_ "embed"
	"fmt"
	"path"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

//go:embed helper.sh
var helperScript string

//go:embed start.sh
var startScript string

// AttachHelper is called after configuration initializers, so the helper's
// prepared config includes persistent runtime configuration and user ConfigMaps.
func AttachHelper(st *appsv1.StatefulSet, enabled bool) error {
	if !enabled {
		return nil
	}
	p := &st.Spec.Template.Spec
	i := mainIndex(p)
	if i < 0 {
		return nil
	}
	main := &p.Containers[i]
	if main.Name != "be" && main.Name != "compute" && main.Name != "cn" {
		return nil
	}
	for _, v := range p.Volumes {
		if v.Name == "doris-helper" || v.Name == "doris-helper-config" || v.Name == "doris-host-cgroup" {
			return fmt.Errorf("volume name %s is reserved", v.Name)
		}
	}
	// Native sidecar startup gates main startup only on control-directory setup,
	// not delegation: the main process must exist before delegation is possible.
	p.ShareProcessNamespace = ptr.To(true)
	p.Volumes = append(p.Volumes,
		corev1.Volume{Name: "doris-helper", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}},
		corev1.Volume{Name: "doris-helper-config", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		corev1.Volume{Name: "doris-host-cgroup", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/sys/fs/cgroup", Type: ptr.To(corev1.HostPathDirectory)}}})
	source := "/etc/doris"
	for _, e := range main.Env {
		if e.Name == "CONFIGMAP_MOUNT_PATH" {
			if e.ValueFrom != nil {
				return fmt.Errorf("helper needs a static CONFIGMAP_MOUNT_PATH")
			}
			source = e.Value
		}
	}
	config := corev1.Container{Name: "prepare-helper-config", Image: main.Image, ImagePullPolicy: main.ImagePullPolicy, SecurityContext: main.SecurityContext.DeepCopy(), Resources: main.Resources,
		Command: []string{"/bin/bash", "-ec", shellCommand(`set -euo pipefail
source=$1
if [[ -d "$source" ]]; then
 for f in "$source"/*; do [[ -f "$f" ]] || continue; cp -L "$f" /helper-config/; done
fi
if [[ ! -f /helper-config/be.conf ]]; then cp /opt/apache-doris/be/conf/be.conf /helper-config/be.conf; fi
chmod u+w /helper-config/be.conf
sed -i '/^[[:space:]]*doris_cgroup_cpu_path[[:space:]]*=/d' /helper-config/be.conf
printf '\ndoris_cgroup_cpu_path = /doris-helper/cgroup\n' >> /helper-config/be.conf`), "prepare-helper-config", source},
		VolumeMounts: []corev1.VolumeMount{{Name: "doris-helper-config", MountPath: "/helper-config"}},
	}
	for _, m := range main.VolumeMounts {
		if path.Clean(m.MountPath) == path.Clean(source) {
			m.ReadOnly = true
			config.VolumeMounts = append(config.VolumeMounts, m)
		}
	}
	p.InitContainers = append(p.InitContainers, config)
	h := corev1.Container{Name: "doris-cgroup-helper", Image: main.Image, ImagePullPolicy: main.ImagePullPolicy,
		RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways), Command: []string{"/bin/bash", "-ec", shellCommand(helperScript)},
		SecurityContext: &corev1.SecurityContext{RunAsUser: ptr.To(int64(0)), RunAsGroup: ptr.To(int64(0)), RunAsNonRoot: ptr.To(false), Privileged: ptr.To(true)},
		Env:             []corev1.EnvVar{{Name: "POD_UID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.uid"}}}, {Name: "TARGET_UID", Value: strconv.FormatInt(*main.SecurityContext.RunAsUser, 10)}, {Name: "TARGET_GID", Value: strconv.FormatInt(*main.SecurityContext.RunAsGroup, 10)}},
		Resources:       corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("32Mi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("64Mi")}},
		VolumeMounts:    []corev1.VolumeMount{{Name: "doris-helper", MountPath: "/doris-helper", MountPropagation: ptr.To(corev1.MountPropagationBidirectional)}, {Name: "doris-host-cgroup", MountPath: "/host-cgroup"}},
		StartupProbe:    &corev1.Probe{ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"test", "-d", "/doris-helper/request"}}}, PeriodSeconds: 1, FailureThreshold: 120},
		ReadinessProbe:  &corev1.Probe{ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"/bin/bash", "-ec", `test -s /doris-helper/ready && mountpoint -q /doris-helper/cgroup`}}}, PeriodSeconds: 2, FailureThreshold: 1},
	}
	p.InitContainers = append(p.InitContainers, h)
	main.VolumeMounts = append(main.VolumeMounts, corev1.VolumeMount{Name: "doris-helper", MountPath: "/doris-helper", MountPropagation: ptr.To(corev1.MountPropagationHostToContainer)}, corev1.VolumeMount{Name: "doris-helper-config", MountPath: "/var/lib/doris/helper-config"})
	setEnv(main, "CONFIGMAP_MOUNT_PATH", "/var/lib/doris/helper-config")
	original := append(append([]string{}, main.Command...), main.Args...)
	main.Command = []string{"/bin/bash", "-ec", shellCommand(startScript), "doris-helper-start"}
	main.Args = original
	return nil
}
