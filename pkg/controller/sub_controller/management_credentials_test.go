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
	"testing"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagementTLS(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().Build()
	conf := map[string]interface{}{resource.TLS_CA_CERTIFICATE_PATH_KEY: "/etc/doris/ca.pem", resource.TLS_CERTIFICATE_PATH_KEY: "/etc/doris/client.pem", resource.TLS_PRIVATE_KEY_PATH_KEY: "/etc/doris/client.key"}
	mounts := []dv1.Secret{{SecretName: "certs"}}
	for _, val := range []string{"", "false", "0"} {
		conf[resource.ENABLE_TLS_KEY] = val
		cfg, secret, err := LoadManagementTLS(ctx, c, "ns", conf, mounts)
		if err != nil || cfg != nil || secret != nil {
			t.Fatalf("disabled TLS %q: %v", val, err)
		}
	}
	conf[resource.ENABLE_TLS_KEY] = "invalid"
	if _, _, err := LoadManagementTLS(ctx, c, "ns", conf, mounts); err == nil {
		t.Fatal("invalid boolean accepted")
	}
	conf[resource.ENABLE_TLS_KEY] = "true"
	for _, mount := range [][]dv1.Secret{nil, mounts} {
		if _, _, err := LoadManagementTLS(ctx, c, "ns", conf, mount); err == nil {
			t.Fatal("missing TLS Secret accepted")
		}
	}
	if err := c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "certs"}}); err != nil {
		t.Fatal(err)
	}
	cfg, secret, err := LoadManagementTLS(ctx, c, "ns", conf, mounts)
	if err != nil || cfg.CAFileName != "ca.pem" || secret.Name != "certs" {
		t.Fatalf("recover after Secret creation: %v", err)
	}
	for _, material := range []*corev1.Secret{nil, secret} {
		if _, err := mysql.NewDorisSqlDB(mysql.DBConfig{}, cfg, material); err == nil {
			t.Fatal("incomplete key material accepted")
		}
	}
}

func TestExplicitManagementSecret(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().Build()
	controller := DisaggregatedSubDefaultController{K8sclient: c}
	ddc := &dv1.DorisDisaggregatedCluster{}
	ddc.Namespace = "ns"
	check := func(wantUser, wantPass string, wantErr bool) {
		t.Helper()
		user, pass, err := controller.GetManagementAdminUserAndPWD(ctx, ddc)
		if (err != nil) != wantErr || user != wantUser || pass != wantPass {
			t.Fatalf("unexpected credentials or error: %v", err)
		}
	}
	check("root", "", false)
	ddc.Spec.AdminUser = &dv1.AdminUser{Name: "legacy", Password: "old"}
	check("legacy", "old", false)
	ddc.Spec.AuthSecret = "account"
	check("", "", true)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "account"}}
	if err := c.Create(ctx, secret); err != nil {
		t.Fatal(err)
	}
	check("", "", true)
	secret.Data = map[string][]byte{"password": []byte("")}
	if err := c.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	check("root", "", false)
	secret.Data = map[string][]byte{"username": []byte("managed"), "password": []byte("new")}
	if err := c.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	check("managed", "new", false)
}

func TestAdminCredentialMigration(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, register := range []func(*runtime.Scheme) error{dv1.AddToScheme, appsv1.AddToScheme, corev1.AddToScheme} {
		if err := register(scheme); err != nil {
			t.Fatal(err)
		}
	}
	for _, existing := range []bool{false, true} {
		ddc := &dv1.DorisDisaggregatedCluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns", UID: "cluster-uid"}}
		ddc.Spec.AdminUser = &dv1.AdminUser{Name: "dbuser", Password: "first"}
		ddc.Spec.AuthSecret = "separate-account"
		objects := []client.Object{ddc}
		if existing {
			objects = append(objects, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "legacy", Namespace: "ns", OwnerReferences: []metav1.OwnerReference{{UID: ddc.UID}}}})
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		if err := c.Get(ctx, client.ObjectKeyFromObject(ddc), ddc); err != nil {
			t.Fatal(err)
		}
		ensure := func() {
			t.Helper()
			if err := EnsureAdminCredentials(ctx, c, ddc); err != nil {
				t.Fatal(err)
			}
		}
		ensure()
		if existing {
			if ddc.Annotations[adminCredentialMode] != modeLegacy || AdminCredentialEnvs(ddc)[1].Value != "first" {
				t.Fatal("upgrade changed existing environment")
			}
			before := ddc.DeepCopy()
			ddc.Annotations[adminCredentialMode] = modeSecret
			if err := c.Patch(ctx, ddc, client.MergeFrom(before)); err != nil {
				t.Fatal(err)
			}
			ensure()
		}
		env := AdminCredentialEnvs(ddc)[1]
		if env.Value != "" || env.ValueFrom.SecretKeyRef.Name != adminSecretName(ddc) || env.ValueFrom.SecretKeyRef.Key != "password" {
			t.Fatal("incorrect Secret reference")
		}
		rev := ddc.Annotations[adminCredentialRevision]
		ensure()
		if rev == "" || rev != ddc.Annotations[adminCredentialRevision] {
			t.Fatal("credential revision is unstable")
		}
		ddc.Spec.AdminUser.Password = "second"
		ensure()
		var secret corev1.Secret
		if err := c.Get(ctx, client.ObjectKey{Namespace: "ns", Name: adminSecretName(ddc)}, &secret); err != nil {
			t.Fatal(err)
		}
		if string(secret.Data["password"]) != "second" || rev == ddc.Annotations[adminCredentialRevision] || !metav1.IsControlledBy(&secret, ddc) {
			t.Fatal("credential rotation failed")
		}
		secret.OwnerReferences = nil
		if err := c.Update(ctx, &secret); err != nil {
			t.Fatal(err)
		}
		if err := EnsureAdminCredentials(ctx, c, ddc); err == nil {
			t.Fatal("accepted a Secret owned elsewhere")
		}
	}
}
