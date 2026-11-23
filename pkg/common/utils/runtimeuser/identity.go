// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership. The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package runtimeuser

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type identity struct {
	UID int64 `json:"uid"`
	GID int64 `json:"gid"`
}

// resolve runs only id in the target image, without storage or API credentials.
// Using kubelet image resolution also supports node-local/offline images and
// imagePullSecrets; the controller does not need registry credentials or TLS CAs.
// Probe objects are owned by the CR and reused across reconciliation retries.
func resolve(ctx context.Context, c client.Client, owner client.Object, template *corev1.PodSpec, main corev1.Container) (identity, error) {
	sc := main.SecurityContext.DeepCopy()
	if sc == nil {
		sc = &corev1.SecurityContext{}
	}
	if template.SecurityContext != nil {
		if sc.RunAsUser == nil {
			sc.RunAsUser = template.SecurityContext.RunAsUser
		}
		if sc.RunAsGroup == nil {
			sc.RunAsGroup = template.SecurityContext.RunAsGroup
		}
		if sc.RunAsNonRoot == nil {
			sc.RunAsNonRoot = template.SecurityContext.RunAsNonRoot
		}
	}
	if sc.RunAsUser != nil && sc.RunAsGroup != nil {
		if sc.RunAsNonRoot != nil && *sc.RunAsNonRoot && *sc.RunAsUser == 0 {
			return identity{}, fmt.Errorf("runAsNonRoot conflicts with UID 0")
		}
		return identity{*sc.RunAsUser, *sc.RunAsGroup}, nil
	}
	sc.Privileged = ptr.To(false)
	sc.AllowPrivilegeEscalation = ptr.To(false)
	sc.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
	sc.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	key, _ := json.Marshal([]interface{}{owner.GetUID(), main.Image, sc, template.ImagePullSecrets, template.NodeSelector, template.ServiceAccountName})
	digest := sha256.Sum256(key)
	name := fmt.Sprintf("doris-image-user-%x", digest[:12])
	var probe corev1.Pod
	err := c.Get(ctx, types.NamespacedName{Namespace: owner.GetNamespace(), Name: name}, &probe)
	if apierrors.IsNotFound(err) {
		probe = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: owner.GetNamespace()}, Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: ptr.To(false),
			ImagePullSecrets: template.ImagePullSecrets, ServiceAccountName: template.ServiceAccountName,
			NodeSelector: template.NodeSelector, Tolerations: template.Tolerations,
			Containers: []corev1.Container{{Name: "identity", Image: main.Image, ImagePullPolicy: main.ImagePullPolicy, SecurityContext: sc,
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("16Mi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("64Mi")}},
				Command:   []string{"/bin/bash", "-ec", `printf '{"uid":%s,"gid":%s}\n' "$(id -u)" "$(id -g)" > /dev/termination-log`},
			}},
		}}
		if err := controllerutil.SetControllerReference(owner, &probe, c.Scheme()); err != nil {
			return identity{}, err
		}
		if err := c.Create(ctx, &probe); err != nil && !apierrors.IsAlreadyExists(err) {
			return identity{}, err
		}
		return identity{}, fmt.Errorf("waiting for image identity probe %s", name)
	}
	if err != nil {
		return identity{}, err
	}
	if !metav1.IsControlledBy(&probe, owner) {
		return identity{}, fmt.Errorf("image probe %s is not owned by this cluster", name)
	}
	for _, status := range probe.Status.ContainerStatuses {
		if status.Name != "identity" {
			continue
		}
		if t := status.State.Terminated; t != nil {
			id, err := parseIdentity(t.Message)
			if t.ExitCode != 0 || err != nil {
				return identity{}, fmt.Errorf("image identity probe %s failed: %s", name, t.Message)
			}
			return id, nil
		}
		if w := status.State.Waiting; w != nil {
			return identity{}, fmt.Errorf("image identity probe %s: %s: %s", name, w.Reason, w.Message)
		}
	}
	return identity{}, fmt.Errorf("waiting for image identity probe %s", name)
}

func parseIdentity(raw string) (identity, error) {
	var value struct {
		UID *int64 `json:"uid"`
		GID *int64 `json:"gid"`
	}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return identity{}, err
	}
	if value.UID == nil || value.GID == nil || *value.UID < 0 || *value.GID < 0 {
		return identity{}, fmt.Errorf("both numeric UID and GID are required")
	}
	return identity{*value.UID, *value.GID}, nil
}
