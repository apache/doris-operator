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
	"path"
	"strconv"
	"strings"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LoadManagementTLS resolves the certificate mount before opening a SQL connection.
func LoadManagementTLS[S dorisv1.Secret | dv1.Secret](ctx context.Context, c client.Client, namespace string, conf map[string]interface{}, mounts []S) (*mysql.TLSConfig, *corev1.Secret, error) {
	raw, ok := conf[resource.ENABLE_TLS_KEY]
	if !ok || raw == "" {
		return nil, nil, nil
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(raw)))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid enable_tls setting: %w", err)
	}
	if !enabled {
		return nil, nil, nil
	}
	ca := resource.GetString(conf, resource.TLS_CA_CERTIFICATE_PATH_KEY)
	cert := resource.GetString(conf, resource.TLS_CERTIFICATE_PATH_KEY)
	key := resource.GetString(conf, resource.TLS_PRIVATE_KEY_PATH_KEY)
	if ca == "" || cert == "" || key == "" {
		return nil, nil, fmt.Errorf("management TLS requires CA, client certificate and key paths")
	}
	for _, item := range mounts {
		mount := dorisv1.Secret(item)
		dir := mount.MountPath
		if dir == "" {
			dir = resource.ConfigEnvPath
		}
		if path.Clean(dir) != path.Dir(ca) {
			continue
		}
		secret, err := k8s.GetSecret(ctx, c, namespace, mount.SecretName)
		if err != nil {
			return nil, nil, fmt.Errorf("read management TLS Secret %s/%s: %w", namespace, mount.SecretName, err)
		}
		return &mysql.TLSConfig{CAFileName: path.Base(ca), ClientCertFileName: path.Base(cert), ClientKeyFileName: path.Base(key)}, secret, nil
	}
	return nil, nil, fmt.Errorf("no management TLS Secret mounted at %s", path.Dir(ca))
}
