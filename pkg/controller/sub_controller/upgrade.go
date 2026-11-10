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

package sub_controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	WaitBackendUpgrade  = "WaitBackendUpgrade"
	UpgradeRequeueAfter = 5 * time.Second
)

// BackendUpgradePending is a normal wait, not a reconciliation failure. The
// parent must continue syncing BE controllers and revisit the FE even if the
// old, healthy pods leave CR status unchanged.
type BackendUpgradePending struct{ Message string }

func (p *BackendUpgradePending) Error() string { return p.Message }

func IsBackendUpgradePending(err error) bool {
	var pending *BackendUpgradePending
	return errors.As(err, &pending)
}

type backendUpgradeTarget struct {
	name, container, image string
	replicas               *int32
}

// CheckFrontendUpgrade only gates image changes to an existing FE StatefulSet.
// Creation and reconciles without an FE image change retain their existing
// behavior. No remembered generation or previous patch is needed: both combined
// and separate BE/FE patches are checked against the current desired backends.
func CheckFrontendUpgrade(ctx context.Context, reader client.Reader, obj client.Object) error {
	var feName, feImage, feContainer string
	var targets []backendUpgradeTarget
	switch cluster := obj.(type) {
	case *dorisv1.DorisCluster:
		if cluster.Spec.FeSpec == nil {
			return nil
		}
		feName = dorisv1.GenerateComponentStatefulSetName(cluster, dorisv1.Component_FE)
		feImage, feContainer = cluster.Spec.FeSpec.Image, string(dorisv1.Component_FE)
		if cluster.Spec.BeSpec != nil {
			targets = append(targets, backendUpgradeTarget{
				name:      dorisv1.GenerateComponentStatefulSetName(cluster, dorisv1.Component_BE),
				container: string(dorisv1.Component_BE), image: cluster.Spec.BeSpec.Image,
				replicas: cluster.Spec.BeSpec.Replicas,
			})
		}
	case *dv1.DorisDisaggregatedCluster:
		feName, feImage, feContainer = cluster.GetFEStatefulsetName(), cluster.Spec.FeSpec.Image, resource.DISAGGREGATED_FE_MAIN_CONTAINER_NAME
		for i := range cluster.Spec.ComputeGroups {
			cg := &cluster.Spec.ComputeGroups[i]
			targets = append(targets, backendUpgradeTarget{
				name: cluster.GetCGStatefulsetName(cg), container: resource.DISAGGREGATED_BE_MAIN_CONTAINER_NAME,
				image: cg.Image, replicas: cg.Replicas,
			})
		}
	default:
		return fmt.Errorf("unsupported frontend upgrade resource %T", obj)
	}

	var fe appsv1.StatefulSet
	if err := reader.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: feName}, &fe); err != nil {
		if apierrors.IsNotFound(err) {
			return nil // FE must be created before BE can register.
		}
		return err
	}
	if mainContainerImage(fe.Spec.Template.Spec.Containers, feContainer) == feImage {
		return nil
	}
	for _, target := range targets {
		if err := checkBackendUpgrade(ctx, reader, obj.GetNamespace(), target); err != nil {
			return err
		}
	}
	return nil
}

func checkBackendUpgrade(ctx context.Context, reader client.Reader, namespace string, target backendUpgradeTarget) error {
	wait := func(reason string) error {
		return &BackendUpgradePending{Message: fmt.Sprintf("FE image update is waiting for backend %s/%s: %s", namespace, target.name, reason)}
	}
	var sts appsv1.StatefulSet
	if err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: target.name}, &sts); err != nil {
		if apierrors.IsNotFound(err) {
			return wait("StatefulSet does not exist")
		}
		return err
	}
	if !sts.DeletionTimestamp.IsZero() || mainContainerImage(sts.Spec.Template.Spec.Containers, target.container) != target.image {
		return wait("StatefulSet has not applied the desired image")
	}
	replicas := int32(1)
	if target.replicas != nil {
		replicas = *target.replicas
	}
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != replicas || sts.Status.ObservedGeneration < sts.Generation {
		return wait("StatefulSet has not observed the desired replicas and generation")
	}
	// Graceful OnDelete rollouts also wait for BE membership after Pod readiness.
	// Respect their existing state machine rather than replacing its completion
	// checks with native RollingUpdate counters or currentRevision equality.
	if raw := sts.Annotations["doris.disaggregated.cluster/graceful-action"]; raw != "" {
		var action dv1.GracefulAction
		if err := json.Unmarshal([]byte(raw), &action); err != nil {
			return wait("graceful action cannot be decoded")
		}
		if action.Phase != dv1.GracefulPhaseDone {
			return wait("graceful action is " + string(action.Phase))
		}
	}
	if sts.Spec.Selector == nil {
		return wait("StatefulSet selector is missing")
	}
	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return err
	}
	var pods corev1.PodList
	if err := reader.List(ctx, &pods, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return err
	}
	var count int32
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !metav1.IsControlledBy(pod, &sts) {
			continue
		}
		count++
		if !pod.DeletionTimestamp.IsZero() || pod.Status.Phase != corev1.PodRunning ||
			mainContainerImage(pod.Spec.Containers, target.container) != target.image ||
			sts.Status.UpdateRevision == "" || pod.Labels[appsv1.StatefulSetRevisionLabel] != sts.Status.UpdateRevision {
			return wait("Pod " + pod.Name + " has not reached the desired image and revision")
		}
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				ready = true
			}
		}
		if !ready {
			return wait("Pod " + pod.Name + " is not Ready")
		}
	}
	if count != replicas {
		return wait(fmt.Sprintf("expected %d Pods, found %d", replicas, count))
	}
	return nil
}

func mainContainerImage(containers []corev1.Container, name string) string {
	for _, container := range containers {
		if container.Name == name {
			return container.Image
		}
	}
	return ""
}
