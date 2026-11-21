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
	"testing"

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_BuildPrometheusAdditionalSecret(t *testing.T) {
	dcr := &dorisv1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "doris"},
		Spec: dorisv1.DorisClusterSpec{
			FeSpec: &dorisv1.FeSpec{},
			BeSpec: &dorisv1.BeSpec{},
		},
	}

	secret, err := BuildPrometheusAdditionalSecret(dcr)
	if err != nil {
		t.Fatalf("build secret failed: %s", err)
	}
	if secret.Name != "demo-prometheus-additional" || secret.Namespace != "doris" {
		t.Fatalf("unexpected secret metadata: %s/%s", secret.Namespace, secret.Name)
	}
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].Name != "demo" {
		t.Fatalf("unexpected owner references: %#v", secret.OwnerReferences)
	}

	jobs := scrapeJobs(t, secret)
	assertTarget(t, jobs, "doris/demo/fe", "demo-fe-service", "http-port", "/metrics")
	assertTarget(t, jobs, "doris/demo/be", "demo-be-service", "webserver-port", "/metrics")
	assertTarget(t, jobs, "doris/demo/be-brpc", "demo-be-service", "brpc-port", "/brpc_metrics")
	if len(jobs) != 3 {
		t.Fatalf("expected 3 scrape jobs, got %d", len(jobs))
	}
}

func scrapeJobs(t *testing.T, secret *corev1.Secret) map[string]prometheusScrapeConfig {
	t.Helper()
	data, ok := secret.Data[PrometheusAdditionalConfigKey]
	if !ok {
		t.Fatalf("secret missing key %s", PrometheusAdditionalConfigKey)
	}
	var configs []prometheusScrapeConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		t.Fatalf("unmarshal scrape configs failed: %s", err)
	}
	jobs := make(map[string]prometheusScrapeConfig, len(configs))
	for _, config := range configs {
		jobs[config.JobName] = config
	}
	return jobs
}

func assertTarget(t *testing.T, jobs map[string]prometheusScrapeConfig, jobName, serviceName, portName, path string) {
	t.Helper()
	job, ok := jobs[jobName]
	if !ok {
		t.Fatalf("job %s not found", jobName)
	}
	if job.MetricsPath != path {
		t.Errorf("job %s metrics_path = %s, want %s", jobName, job.MetricsPath, path)
	}
	if len(job.KubernetesSDConfigs) != 1 || len(job.KubernetesSDConfigs[0].Namespaces.Names) != 1 ||
		job.KubernetesSDConfigs[0].Namespaces.Names[0] != "doris" {
		t.Errorf("job %s unexpected kubernetes_sd_configs: %#v", jobName, job.KubernetesSDConfigs)
	}

	relabels := map[string]string{}
	for _, relabel := range job.RelabelConfigs {
		if len(relabel.SourceLabels) == 1 && relabel.Action == "keep" {
			relabels[relabel.SourceLabels[0]] = relabel.Regex
		}
	}
	if relabels["__meta_kubernetes_service_name"] != serviceName {
		t.Errorf("job %s service selector = %s, want %s", jobName, relabels["__meta_kubernetes_service_name"], serviceName)
	}
	if relabels["__meta_kubernetes_endpoint_port_name"] != portName {
		t.Errorf("job %s port selector = %s, want %s", jobName, relabels["__meta_kubernetes_endpoint_port_name"], portName)
	}
}
