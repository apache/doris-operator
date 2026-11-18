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

package k8s

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateResourceOwnership accepts legacy non-controller owners while refusing
// to adopt an object belonging to another cluster or controller.
func ValidateResourceOwnership(desired, existing client.Object) error {
	refs := desired.GetOwnerReferences()
	if len(refs) == 0 {
		return nil
	}
	owner := refs[0]
	if controller := metav1.GetControllerOf(desired); controller != nil {
		owner = *controller
	}
	// Objects constructed before admission may not have an owner UID yet.
	if owner.UID == "" {
		return nil
	}
	return validateOwnerReference(owner, existing)
}

// ValidateResourceOwner protects operations that run before the desired child
// is constructed, such as FE SQL scale-down and CG graceful transitions.
func ValidateResourceOwner(owner, existing client.Object) error {
	if owner.GetUID() == "" {
		return nil
	}
	return validateOwnerReference(metav1.OwnerReference{Name: owner.GetName(), UID: owner.GetUID()}, existing)
}
func validateOwnerReference(expected metav1.OwnerReference, existing client.Object) error {
	if other := metav1.GetControllerOf(existing); other != nil && other.UID != expected.UID {
		return fmt.Errorf("resource %s/%s is controlled by %s %s (%s), refusing to overwrite it", existing.GetNamespace(), existing.GetName(), other.Kind, other.Name, other.UID)
	}
	for _, ref := range existing.GetOwnerReferences() {
		if ref.UID == expected.UID && ref.Name == expected.Name {
			return nil
		}
	}
	return fmt.Errorf("resource %s/%s is not owned by %s (%s), refusing to adopt it", existing.GetNamespace(), existing.GetName(), expected.Name, expected.UID)
}
