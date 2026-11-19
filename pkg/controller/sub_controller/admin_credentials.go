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
	"fmt"
	"reflect"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	adminCredentialMode     = "doris.apache.org/admin-credential-mode"
	adminCredentialRevision = "doris.apache.org/admin-credential-revision"
	modeSecret              = "secret"
	modeLegacy              = "legacy"
)

func adminSecretName(ddc *dv1.DorisDisaggregatedCluster) string {
	return "doris-admin-" + string(ddc.UID)
}

// EnsureAdminCredentials establishes the cluster-wide mode before any child is rendered.
func EnsureAdminCredentials(ctx context.Context, c client.Client, ddc *dv1.DorisDisaggregatedCluster) error {
	if !ddc.DeletionTimestamp.IsZero() {
		return nil
	}
	mode := ddc.Annotations[adminCredentialMode]
	if mode == "" {
		existing, err := hasExistingDeployment(ctx, c, ddc)
		if err != nil {
			return err
		}
		mode = modeSecret
		if existing {
			mode = modeLegacy
		}
	}
	if mode != modeSecret && mode != modeLegacy {
		return fmt.Errorf("unknown %s value %q", adminCredentialMode, mode)
	}
	revision := ""
	if mode == modeSecret && ddc.Spec.AdminUser != nil {
		secret, err := reconcileAdminSecret(ctx, c, ddc)
		if err != nil {
			return err
		}
		revision = secret.Annotations[adminCredentialRevision]
	}
	if mode == ddc.Annotations[adminCredentialMode] && revision == ddc.Annotations[adminCredentialRevision] {
		return nil
	}
	before := ddc.DeepCopy()
	if ddc.Annotations == nil {
		ddc.Annotations = make(map[string]string)
	}
	ddc.Annotations[adminCredentialMode] = mode
	if revision == "" {
		delete(ddc.Annotations, adminCredentialRevision)
	} else {
		ddc.Annotations[adminCredentialRevision] = revision
	}
	return c.Patch(ctx, ddc, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}

func hasExistingDeployment(ctx context.Context, c client.Client, ddc *dv1.DorisDisaggregatedCluster) (bool, error) {
	if !reflect.ValueOf(ddc.Status).IsZero() {
		return true, nil
	}
	var workloads appsv1.StatefulSetList
	if err := c.List(ctx, &workloads, client.InNamespace(ddc.Namespace)); err != nil {
		return false, err
	}
	for _, st := range workloads.Items {
		// Older releases created non-controller owner references.
		for _, owner := range st.OwnerReferences {
			if ddc.UID != "" && owner.UID == ddc.UID {
				return true, nil
			}
		}
	}
	return false, nil
}

func reconcileAdminSecret(ctx context.Context, c client.Client, ddc *dv1.DorisDisaggregatedCluster) (*corev1.Secret, error) {
	name := adminSecretName(ddc)
	data := map[string][]byte{corev1.BasicAuthUsernameKey: []byte(ddc.Spec.AdminUser.Name), corev1.BasicAuthPasswordKey: []byte(ddc.Spec.AdminUser.Password)}
	secret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{Namespace: ddc.Namespace, Name: name}, secret)
	if apierrors.IsNotFound(err) {
		owner := metav1.NewControllerRef(ddc, dv1.GroupVersion.WithKind("DorisDisaggregatedCluster"))
		owner.BlockOwnerDeletion = nil
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: ddc.Namespace, Name: name,
				OwnerReferences: []metav1.OwnerReference{*owner},
				Annotations:     map[string]string{adminCredentialRevision: string(uuid.NewUUID())},
			},
			Data: data,
		}
		return secret, c.Create(ctx, secret)
	}
	if err != nil {
		return nil, err
	}
	if !metav1.IsControlledBy(secret, ddc) {
		return nil, fmt.Errorf("Secret %s/%s already exists with a different owner", ddc.Namespace, name)
	}
	if secret.DeletionTimestamp != nil {
		return nil, fmt.Errorf("waiting for deletion of Secret %s/%s", ddc.Namespace, name)
	}
	// A metadata-only edit must not restart workloads that consume this Secret.
	if reflect.DeepEqual(secret.Data, data) && secret.Annotations[adminCredentialRevision] != "" {
		return secret, nil
	}
	secret.Data = data
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[adminCredentialRevision] = string(uuid.NewUUID())
	return secret, c.Update(ctx, secret)
}

func AdminCredentialEnvs(ddc *dv1.DorisDisaggregatedCluster) []corev1.EnvVar {
	user := ddc.Spec.AdminUser
	if user == nil {
		return nil
	}
	if ddc.Annotations[adminCredentialMode] != modeSecret {
		return []corev1.EnvVar{{Name: resource.ADMIN_USER, Value: user.Name}, {Name: resource.ADMIN_PASSWD, Value: user.Password}}
	}
	ref := func(key string) *corev1.EnvVarSource {
		return &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: adminSecretName(ddc)}, Key: key}}
	}
	return []corev1.EnvVar{{Name: resource.ADMIN_USER, ValueFrom: ref(corev1.BasicAuthUsernameKey)}, {Name: resource.ADMIN_PASSWD, ValueFrom: ref(corev1.BasicAuthPasswordKey)}}
}

// Secret-backed environment variables are read once when the container starts.
func SetAdminCredentialRevision(ddc *dv1.DorisDisaggregatedCluster, pts *corev1.PodTemplateSpec) {
	if rev := ddc.Annotations[adminCredentialRevision]; rev != "" && ddc.Annotations[adminCredentialMode] == modeSecret && ddc.Spec.AdminUser != nil {
		if pts.Annotations == nil {
			pts.Annotations = make(map[string]string)
		}
		pts.Annotations[adminCredentialRevision] = rev
	}
}
