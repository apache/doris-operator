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

package resource

import (
	"encoding/json"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"strings"
)

func statefulSetStrategyEqual(desired, actual appsv1.StatefulSetUpdateStrategy) bool {
	// The CG business comparison deliberately clears strategy; its graceful
	// state machine compares and manages this field separately.
	if desired.Type == "" {
		return true
	}
	if actual.Type == "" {
		actual.Type = appsv1.RollingUpdateStatefulSetStrategyType
	}
	if desired.Type != actual.Type {
		return false
	}
	if desired.Type == appsv1.OnDeleteStatefulSetStrategyType {
		return true
	}
	partition := func(s appsv1.StatefulSetUpdateStrategy) int32 {
		if s.RollingUpdate == nil {
			return 0
		}
		return ptr.Deref(s.RollingUpdate.Partition, int32(0))
	}
	if partition(desired) != partition(actual) {
		return false
	}
	if desired.RollingUpdate != nil && desired.RollingUpdate.MaxUnavailable != nil {
		return actual.RollingUpdate != nil && equality.Semantic.DeepEqual(desired.RollingUpdate.MaxUnavailable, actual.RollingUpdate.MaxUnavailable)
	}
	return true
}
func normalizedPodTemplate(template, desired corev1.PodTemplateSpec) corev1.PodTemplateSpec {
	// JSON round-tripping removes empty slices/maps as the API does.
	data, _ := json.Marshal(template)
	var result corev1.PodTemplateSpec
	_ = json.Unmarshal(data, &result)
	labels, annotations := map[string]string{}, map[string]string{}
	for key := range desired.Labels {
		if value, ok := result.Labels[key]; ok {
			labels[key] = value
		}
	}
	for key := range desired.Annotations {
		if value, ok := result.Annotations[key]; ok {
			annotations[key] = value
		}
	}
	result.ObjectMeta = metav1.ObjectMeta{Labels: labels, Annotations: annotations}
	pod := &result.Spec
	if pod.RestartPolicy == "" {
		pod.RestartPolicy = corev1.RestartPolicyAlways
	}
	if pod.DNSPolicy == "" {
		pod.DNSPolicy = corev1.DNSClusterFirst
	}
	if pod.SchedulerName == "" {
		pod.SchedulerName = corev1.DefaultSchedulerName
	}
	if pod.TerminationGracePeriodSeconds == nil {
		pod.TerminationGracePeriodSeconds = ptr.To(int64(30))
	}
	if pod.SecurityContext == nil {
		pod.SecurityContext = &corev1.PodSecurityContext{}
	}
	if pod.EnableServiceLinks == nil {
		pod.EnableServiceLinks = ptr.To(true)
	}
	if pod.PreemptionPolicy == nil {
		pod.PreemptionPolicy = ptr.To(corev1.PreemptLowerPriority)
	}
	normalizeContainers(pod.Containers)
	normalizeContainers(pod.InitContainers)
	for i := range pod.Volumes {
		volume := &pod.Volumes[i]
		if volume.Secret != nil && volume.Secret.DefaultMode == nil {
			volume.Secret.DefaultMode = ptr.To(int32(420))
		}
		if volume.ConfigMap != nil && volume.ConfigMap.DefaultMode == nil {
			volume.ConfigMap.DefaultMode = ptr.To(int32(420))
		}
		if volume.DownwardAPI != nil {
			if volume.DownwardAPI.DefaultMode == nil {
				volume.DownwardAPI.DefaultMode = ptr.To(int32(420))
			}
			for j := range volume.DownwardAPI.Items {
				if ref := volume.DownwardAPI.Items[j].FieldRef; ref != nil && ref.APIVersion == "" {
					ref.APIVersion = "v1"
				}
			}
		}
		if volume.Projected != nil {
			if volume.Projected.DefaultMode == nil {
				volume.Projected.DefaultMode = ptr.To(int32(420))
			}
			for j := range volume.Projected.Sources {
				if source := volume.Projected.Sources[j].DownwardAPI; source != nil {
					for k := range source.Items {
						if ref := source.Items[k].FieldRef; ref != nil && ref.APIVersion == "" {
							ref.APIVersion = "v1"
						}
					}
				}
			}
		}
		if volume.HostPath != nil && volume.HostPath.Type == nil {
			volume.HostPath.Type = ptr.To(corev1.HostPathUnset)
		}
	}
	return result
}
func normalizeContainers(containers []corev1.Container) {
	for i := range containers {
		c := &containers[i]
		if c.ImagePullPolicy == "" {
			c.ImagePullPolicy = corev1.PullIfNotPresent
			image := c.Image
			if !strings.Contains(image, "@") {
				last := image[strings.LastIndex(image, "/")+1:]
				if !strings.Contains(last, ":") || strings.HasSuffix(last, ":latest") {
					c.ImagePullPolicy = corev1.PullAlways
				}
			}
		}
		if c.TerminationMessagePath == "" {
			c.TerminationMessagePath = corev1.TerminationMessagePathDefault
		}
		if c.TerminationMessagePolicy == "" {
			c.TerminationMessagePolicy = corev1.TerminationMessageReadFile
		}
		for j := range c.Ports {
			if c.Ports[j].Protocol == "" {
				c.Ports[j].Protocol = corev1.ProtocolTCP
			}
		}
		for j := range c.Env {
			if c.Env[j].ValueFrom != nil {
				if ref := c.Env[j].ValueFrom.FieldRef; ref != nil && ref.APIVersion == "" {
					ref.APIVersion = "v1"
				}
			}
		}
		for _, probe := range []*corev1.Probe{c.LivenessProbe, c.ReadinessProbe, c.StartupProbe} {
			if probe == nil {
				continue
			}
			if probe.TimeoutSeconds == 0 {
				probe.TimeoutSeconds = 1
			}
			if probe.PeriodSeconds == 0 {
				probe.PeriodSeconds = 10
			}
			if probe.SuccessThreshold == 0 {
				probe.SuccessThreshold = 1
			}
			if probe.FailureThreshold == 0 {
				probe.FailureThreshold = 3
			}
			if probe.HTTPGet != nil && probe.HTTPGet.Scheme == "" {
				probe.HTTPGet.Scheme = corev1.URISchemeHTTP
			}
		}
		if c.Lifecycle != nil {
			for _, handler := range []*corev1.LifecycleHandler{c.Lifecycle.PostStart, c.Lifecycle.PreStop} {
				if handler != nil && handler.HTTPGet != nil && handler.HTTPGet.Scheme == "" {
					handler.HTTPGet.Scheme = corev1.URISchemeHTTP
				}
			}
		}
	}
}
func normalizedClaimTemplates(claims []corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	if len(claims) == 0 {
		return nil
	}
	result := make([]corev1.PersistentVolumeClaim, len(claims))
	for i := range claims {
		result[i] = *claims[i].DeepCopy()
		result[i].TypeMeta = metav1.TypeMeta{}
		result[i].ObjectMeta = metav1.ObjectMeta{Name: claims[i].Name, Labels: claims[i].Labels, Annotations: claims[i].Annotations}
		result[i].Status = corev1.PersistentVolumeClaimStatus{}
		if result[i].Spec.VolumeMode == nil {
			result[i].Spec.VolumeMode = ptr.To(corev1.PersistentVolumeFilesystem)
		}
	}
	return result
}
