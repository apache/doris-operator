// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package controller

import tdev1 "github.com/apache/doris-operator/api/tde"

func tdeReferencesSecret(config *tdev1.TDEConfig, status *tdev1.TDEStatus, name string) bool {
	if config == nil {
		return statusReferencesSecret(status, name)
	}
	if config.Provider.Local != nil && config.Provider.Local.SecretKeyRef.Name == name {
		return true
	}
	if config.Provider.Kms != nil && config.Provider.Kms.Auth.CredentialSecretRef != nil && config.Provider.Kms.Auth.CredentialSecretRef.Name == name {
		return true
	}
	return statusReferencesSecret(status, name)
}

func statusReferencesSecret(status *tdev1.TDEStatus, name string) bool {
	if status == nil {
		return false
	}
	providers := []*tdev1.ProviderStatus{status.Current}
	if status.Operation != nil {
		providers = append(providers, status.Operation.Source, status.Operation.Target)
	}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if provider.RootKeyRef != nil && provider.RootKeyRef.SecretName == name {
			return true
		}
		if provider.Kms != nil && provider.Kms.Auth.CredentialSecretName == name {
			return true
		}
	}
	return false
}
