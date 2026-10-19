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

// Package runtimeconfig preserves mutable Doris configuration without changing
// user ConfigMaps, PVC templates, or the layout of image-provided conf directories.
package runtimeconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PolicyAnnotation    = "doris.apache.org/runtime-config-policy"
	MigrationAnnotation = "doris.apache.org/runtime-config-migration"
	StorageAnnotation   = "doris.apache.org/runtime-config-storage"
	Auto                = "auto"
	Legacy              = "legacy"
	Enabled             = "enabled"
	Disabled            = "disabled"
)

// EnsurePolicy records the default before any child workload is created. The
// cluster-level decision survives deletion/recreation of an individual workload.
func EnsurePolicy(ctx context.Context, c client.Client, obj client.Object) error {
	if !obj.GetDeletionTimestamp().IsZero() {
		return nil
	}
	var explicit *bool
	var hasStatus bool
	switch o := obj.(type) {
	case *v1.DorisCluster:
		if o.Spec.RuntimeConfigPersistence != nil {
			explicit = &o.Spec.RuntimeConfigPersistence.Enabled
		}
		hasStatus = !reflect.ValueOf(o.Status).IsZero()
	case *dv1.DorisDisaggregatedCluster:
		if o.Spec.RuntimeConfigPersistence != nil {
			explicit = &o.Spec.RuntimeConfigPersistence.Enabled
		}
		hasStatus = !reflect.ValueOf(o.Status).IsZero()
	default:
		return fmt.Errorf("unsupported runtime configuration owner %T", obj)
	}
	current := obj.GetAnnotations()[PolicyAnnotation]
	if current != "" && current != Auto && current != Legacy && current != Enabled && current != Disabled {
		return fmt.Errorf("invalid %s value %q", PolicyAnnotation, current)
	}
	if (explicit == nil || (*explicit && current == Enabled)) && current != "" {
		return nil
	}
	if explicit != nil && !*explicit && (current == Enabled || current == Auto) {
		return fmt.Errorf("runtime configuration persistence cannot be disabled after activation; migrate overrides first")
	}
	var workloads appsv1.StatefulSetList
	if err := c.List(ctx, &workloads, client.InNamespace(obj.GetNamespace())); err != nil {
		return err
	}
	owned := make([]appsv1.StatefulSet, 0)
	for _, st := range workloads.Items {
		if ownedBy(&st, obj) {
			owned = append(owned, st)
		}
	}
	desired := current
	if desired == "" {
		desired = Auto
		if hasStatus || len(owned) != 0 {
			desired = Legacy
		}
	}
	if explicit != nil {
		if *explicit {
			if desired != Enabled && (len(owned) != 0 || desired == Legacy) {
				if err := validateMigration(obj, owned); err != nil {
					return err
				}
				if err := validateMigrationPods(ctx, c, obj, owned); err != nil {
					return err
				}
			}
			desired = Enabled
		} else {
			desired = Disabled
		}
	}
	if current == desired {
		return nil
	}
	before := obj.DeepCopyObject().(client.Object)
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[PolicyAnnotation] = desired
	obj.SetAnnotations(annotations)
	return c.Patch(ctx, obj, client.MergeFrom(before))
}

type migrationWorkload struct {
	UID        string `json:"uid"`
	Generation int64  `json:"generation"`
	Replicas   int32  `json:"replicas"`
}

type migration struct {
	ClusterUID string                       `json:"clusterUID"`
	Workloads  map[string]migrationWorkload `json:"workloads"`
	PodUIDs    map[string]string            `json:"podUIDs"`
}

func validateMigration(obj client.Object, workloads []appsv1.StatefulSet) error {
	var approval migration
	if err := json.Unmarshal([]byte(obj.GetAnnotations()[MigrationAnnotation]), &approval); err != nil || approval.ClusterUID != string(obj.GetUID()) {
		return fmt.Errorf("runtime configuration migration required: run tools/runtime-config-migrate.py before enabling persistence")
	}
	if len(workloads) == 0 {
		return fmt.Errorf("cannot validate runtime configuration migration without existing workloads")
	}
	for _, st := range workloads {
		if !supportedContainer(st.Spec.Template.Spec.Containers) {
			continue
		}
		old, ok := approval.Workloads[st.Name]
		replicas := int32(1)
		if st.Spec.Replicas != nil {
			replicas = *st.Spec.Replicas
		}
		if !ok || old.UID != string(st.UID) || old.Generation != st.Generation || old.Replicas != replicas {
			return fmt.Errorf("runtime configuration migration for %s is missing or stale; rerun the migration tool", st.Name)
		}
	}
	return nil
}

func validateMigrationPods(ctx context.Context, c client.Client, obj client.Object, workloads []appsv1.StatefulSet) error {
	var approval migration
	if err := json.Unmarshal([]byte(obj.GetAnnotations()[MigrationAnnotation]), &approval); err != nil {
		return err
	}
	var pods corev1.PodList
	if err := c.List(ctx, &pods, client.InNamespace(obj.GetNamespace())); err != nil {
		return err
	}
	for _, st := range workloads {
		if !supportedContainer(st.Spec.Template.Spec.Containers) {
			continue
		}
		count := int32(0)
		for _, pod := range pods.Items {
			if !ownedBy(&pod, &st) {
				continue
			}
			if !pod.DeletionTimestamp.IsZero() || approval.PodUIDs[pod.Name] != string(pod.UID) {
				return fmt.Errorf("Pod %s changed since migration; rerun the migration tool", pod.Name)
			}
			count++
		}
		if count != approval.Workloads[st.Name].Replicas {
			return fmt.Errorf("Pod membership for %s changed since migration", st.Name)
		}
	}
	return nil
}

// Older operator resources use owner references without Controller=true.
func ownedBy(child, owner client.Object) bool {
	if owner.GetUID() == "" {
		return false
	}
	for _, ref := range child.GetOwnerReferences() {
		if ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}
