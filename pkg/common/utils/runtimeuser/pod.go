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
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const userAnnotation = "doris.apache.org/runtime-user"
const migrationAnnotation = "doris.apache.org/root-volume-migration"

//go:embed permissions.sh
var permissionScript string

func mainIndex(p *corev1.PodSpec) int {
	for i, c := range p.Containers {
		switch c.Name {
		case "fe", "be", "compute", "metaservice", "cn", "broker":
			return i
		}
	}
	return -1
}

// Prepare resolves identity before changing any workload. The recorded identity
// is part of the template revision, rather than inferred from image tag strings.
func Prepare(ctx context.Context, c client.Client, owner client.Object, st *appsv1.StatefulSet, helper bool) error {
	return PrepareWithWorkloadGroup(ctx, c, owner, st, helper, false)
}

// PrepareWithWorkloadGroup preserves the explicit legacy BE/compute WG policy for root workloads.
func PrepareWithWorkloadGroup(ctx context.Context, c client.Client, owner client.Object, st *appsv1.StatefulSet, helper, legacyWorkloadGroup bool) error {
	p := &st.Spec.Template.Spec
	i := mainIndex(p)
	if i < 0 {
		return nil
	}
	main := &p.Containers[i]
	target, err := resolve(ctx, c, owner, p, *main)
	if err != nil {
		return err
	}
	var old appsv1.StatefulSet
	err = c.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &old)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	migrate := false
	if err == nil {
		j := mainIndex(&old.Spec.Template.Spec)
		if j >= 0 {
			previous := old.Spec.Template.Spec.Containers[j]
			var source identity
			if raw := old.Spec.Template.Annotations[userAnnotation]; raw != "" {
				source, err = parseIdentity(raw)
				if err != nil {
					return fmt.Errorf("invalid recorded runtime identity: %w", err)
				}
			} else {
				source, err = resolve(ctx, c, owner, &old.Spec.Template.Spec, previous)
				if err != nil {
					return err
				}
			}
			if source.UID != 0 && target != source {
				return fmt.Errorf("non-root UID/GID change from %v to %v requires an explicit storage migration", source, target)
			}
			migrate = source.UID == 0 && target.UID != 0
			// Keep the initializer while a partially migrated StatefulSet is retried.
			if old.Spec.Template.Annotations[migrationAnnotation] != "" {
				if previous.Image == main.Image {
					migrate = old.Spec.Template.Annotations[migrationAnnotation] == main.Image
				} else if old.Spec.Replicas != nil && (old.Status.ObservedGeneration < old.Generation || old.Status.UpdatedReplicas != *old.Spec.Replicas || old.Status.ReadyReplicas != *old.Spec.Replicas || old.Status.CurrentRevision != old.Status.UpdateRevision) {
					return fmt.Errorf("finish the existing PVC migration before changing image again")
				}
			}
		}
	}
	// Leave the legacy template intact when the resolved image runs as root.
	// The normal resource builder already honors skipDefaultSystemInit and sets WG env.
	if target.UID == 0 && !helper {
		if legacyWorkloadGroup {
			sc := main.SecurityContext.DeepCopy()
			if sc == nil {
				sc = &corev1.SecurityContext{}
			}
			sc.Privileged = ptr.To(true)
			main.SecurityContext = sc
		}
		return nil
	}
	if st.Spec.Template.Annotations == nil {
		st.Spec.Template.Annotations = map[string]string{}
	}
	raw, _ := json.Marshal(target)
	st.Spec.Template.Annotations[userAnnotation] = string(raw)
	if migrate {
		st.Spec.Template.Annotations[migrationAnnotation] = main.Image
	} else if old.Spec.Template.Annotations[migrationAnnotation] != "" {
		// StatefulSet updates preserve omitted annotation keys. An explicit empty
		// value clears the old migration state through that merge.
		st.Spec.Template.Annotations[migrationAnnotation] = ""
	} else {
		delete(st.Spec.Template.Annotations, migrationAnnotation)
	}
	if p.SecurityContext == nil {
		p.SecurityContext = &corev1.PodSecurityContext{}
	} else {
		p.SecurityContext = p.SecurityContext.DeepCopy()
	}
	p.SecurityContext.RunAsUser = ptr.To(target.UID)
	p.SecurityContext.RunAsGroup = ptr.To(target.GID)
	p.SecurityContext.RunAsNonRoot = ptr.To(target.UID != 0)
	p.SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	externalClaim := false
	for _, volume := range p.Volumes {
		if volume.PersistentVolumeClaim != nil && !templateClaim(st, volume.Name) {
			externalClaim = true
			break
		}
	}
	// Automatic fsGroup must not recursively change group ownership of shared
	// external claims. Explicit user-provided fsGroup remains their choice.
	if target.UID != 0 && !externalClaim {
		p.SecurityContext.FSGroup = ptr.To(target.GID)
		policy := corev1.FSGroupChangeOnRootMismatch
		if migrate {
			policy = corev1.FSGroupChangeAlways
		}
		p.SecurityContext.FSGroupChangePolicy = &policy
	}
	sc := main.SecurityContext.DeepCopy()
	if sc == nil {
		sc = &corev1.SecurityContext{}
	}
	if sc.Privileged != nil && *sc.Privileged {
		return fmt.Errorf("Doris main container must not be privileged; use enablePrivilegedHelper")
	}
	sc.RunAsUser = ptr.To(target.UID)
	sc.RunAsGroup = ptr.To(target.GID)
	sc.RunAsNonRoot = ptr.To(target.UID != 0)
	sc.Privileged = ptr.To(false)
	sc.AllowPrivilegeEscalation = ptr.To(false)
	if target.UID != 0 {
		sc.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
	}
	main.SecurityContext = sc
	// The single opt-in also gates the existing privileged node initializer.
	// Preserve its root identity when the Pod itself now defaults to non-root.
	inits := []corev1.Container{}
	for _, init := range p.InitContainers {
		if init.Name == "default-init" {
			if !helper {
				continue
			}
			init.SecurityContext = &corev1.SecurityContext{
				Privileged: ptr.To(true), RunAsUser: ptr.To(int64(0)),
				RunAsGroup: ptr.To(int64(0)), RunAsNonRoot: ptr.To(false),
			}
		}
		if !helper && init.SecurityContext != nil && init.SecurityContext.Privileged != nil && *init.SecurityContext.Privileged {
			return fmt.Errorf("custom privileged init %s requires enablePrivilegedHelper", init.Name)
		}
		inits = append(inits, init)
	}
	p.InitContainers = inits
	setEnv(main, "ENABLE_WORKLOAD_GROUP", "false")
	if !helper {
		setEnv(main, "SKIP_CHECK_ULIMIT", "true")
	}
	if target.UID != 0 {
		paths := []corev1.VolumeMount{}
		for _, m := range main.VolumeMounts {
			if persistent(st, m.Name) && !m.ReadOnly {
				paths = append(paths, m)
			}
		}
		if len(paths) > 0 {
			check := corev1.Container{Name: "check-volume-permissions", Image: main.Image, ImagePullPolicy: main.ImagePullPolicy,
				Command: []string{"/bin/bash", "-ec", shellCommand(permissionScript), "permissions", "verify"}, SecurityContext: sc.DeepCopy(), VolumeMounts: paths, Resources: main.Resources}
			for _, m := range paths {
				check.Command = append(check.Command, m.MountPath)
			}
			if migrate {
				// Shared/external claims may have other writers. Check them as
				// the target user, but never recursively chown another workload's data.
				check.Command[4] = "verify-deep"
				repairPaths := []corev1.VolumeMount{}
				for _, m := range paths {
					if templateClaim(st, m.Name) {
						repairPaths = append(repairPaths, m)
					}
				}
				repair := *check.DeepCopy()
				repair.Name = "migrate-volume-permissions"
				repair.Command = []string{"/bin/bash", "-ec", shellCommand(permissionScript), "permissions", "repair", strconv.FormatInt(target.UID, 10), strconv.FormatInt(target.GID, 10)}
				repair.VolumeMounts = repairPaths
				for _, m := range repairPaths {
					repair.Command = append(repair.Command, m.MountPath)
				}
				repair.SecurityContext = &corev1.SecurityContext{RunAsUser: ptr.To(int64(0)), RunAsGroup: ptr.To(int64(0)), RunAsNonRoot: ptr.To(false), Privileged: ptr.To(false), AllowPrivilegeEscalation: ptr.To(false), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}, Add: []corev1.Capability{"CHOWN", "FOWNER", "DAC_OVERRIDE"}}}
				if len(repairPaths) > 0 {
					p.InitContainers = append([]corev1.Container{repair, check}, p.InitContainers...)
				} else {
					p.InitContainers = append([]corev1.Container{check}, p.InitContainers...)
				}
			} else {
				p.InitContainers = append([]corev1.Container{check}, p.InitContainers...)
			}
		}
	}
	return nil
}

func persistent(st *appsv1.StatefulSet, name string) bool {
	for _, v := range st.Spec.VolumeClaimTemplates {
		if v.Name == name {
			return true
		}
	}
	for _, v := range st.Spec.Template.Spec.Volumes {
		if v.Name == name {
			return v.PersistentVolumeClaim != nil && !v.PersistentVolumeClaim.ReadOnly
		}
	}
	return false
}
func setEnv(c *corev1.Container, name, value string) {
	env := []corev1.EnvVar{}
	for _, v := range c.Env {
		if v.Name != name {
			env = append(env, v)
		}
	}
	c.Env = append(env, corev1.EnvVar{Name: name, Value: value})
}

// Kubelet expands dollar escapes in command/args before invoking the shell.
func shellCommand(script string) string { return strings.ReplaceAll(script, "$", "$$") }

func templateClaim(st *appsv1.StatefulSet, name string) bool {
	for _, claim := range st.Spec.VolumeClaimTemplates {
		if claim.Name == name {
			return true
		}
	}
	return false
}
