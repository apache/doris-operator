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
	"bytes"
	"fmt"

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// PrometheusAdditionalConfigKey is the Secret key referenced by Prometheus additionalScrapeConfigs.
	PrometheusAdditionalConfigKey = "prometheus-additional.yaml"
	prometheusAdditionalSuffix    = "-prometheus-additional"
)

type prometheusScrapeConfig struct {
	JobName             string              `yaml:"job_name"`
	MetricsPath         string              `yaml:"metrics_path"`
	KubernetesSDConfigs []prometheusK8sSD   `yaml:"kubernetes_sd_configs"`
	RelabelConfigs      []prometheusRelabel `yaml:"relabel_configs"`
}

type prometheusK8sSD struct {
	Role       string                 `yaml:"role"`
	Namespaces prometheusSDNamespaces `yaml:"namespaces"`
}

type prometheusSDNamespaces struct {
	Names []string `yaml:"names"`
}

type prometheusRelabel struct {
	SourceLabels []string `yaml:"source_labels,omitempty"`
	TargetLabel  string   `yaml:"target_label,omitempty"`
	Regex        string   `yaml:"regex,omitempty"`
	Action       string   `yaml:"action"`
}

type prometheusTarget struct {
	jobName     string
	namespace   string
	serviceName string
	portName    string
	path        string
}

type prometheusEndpoint struct {
	// jobSuffix distinguishes multiple endpoints of one component, e.g. "-brpc".
	jobSuffix string
	portKey   string
	path      string
}

type prometheusComponent struct {
	component dorisv1.ComponentType
	enabled   func(*dorisv1.DorisCluster) bool
	endpoints []prometheusEndpoint
}

// prometheusComponents defines the components and endpoints scraped by Prometheus.
var prometheusComponents = []prometheusComponent{
	{
		component: dorisv1.Component_FE,
		enabled:   func(dcr *dorisv1.DorisCluster) bool { return dcr.Spec.FeSpec != nil },
		endpoints: []prometheusEndpoint{
			{portKey: HTTP_PORT, path: "/metrics"},
		},
	},
	{
		component: dorisv1.Component_BE,
		enabled:   func(dcr *dorisv1.DorisCluster) bool { return dcr.Spec.BeSpec != nil },
		endpoints: []prometheusEndpoint{
			{portKey: WEBSERVER_PORT, path: "/metrics"},
			{jobSuffix: "-brpc", portKey: BRPC_PORT, path: "/brpc_metrics"},
		},
	},
}

// PrometheusAdditionalSecretName returns the name of the generated scrape config Secret.
func PrometheusAdditionalSecretName(clusterName string) string {
	return clusterName + prometheusAdditionalSuffix
}

// BuildPrometheusAdditionalSecret builds the Secret consumed by Prometheus additionalScrapeConfigs for a DorisCluster.
func BuildPrometheusAdditionalSecret(dcr *dorisv1.DorisCluster) (*corev1.Secret, error) {
	targets := make([]prometheusTarget, 0, 3)
	for _, component := range prometheusComponents {
		if component.enabled(dcr) {
			targets = append(targets, component.prometheusTargets(dcr)...)
		}
	}

	labels := map[string]string{dorisv1.OwnerReference: dcr.Name}
	owner := prometheusOwnerReference(dorisv1.GroupVersion.String(), "DorisCluster", dcr.Name, dcr.UID)
	return buildPrometheusAdditionalSecret(dcr.Name, dcr.Namespace, labels, owner, targets)
}

func (c prometheusComponent) prometheusTargets(dcr *dorisv1.DorisCluster) []prometheusTarget {
	serviceName := dorisv1.GenerateExternalServiceName(dcr, c.component)
	targets := make([]prometheusTarget, 0, len(c.endpoints))
	for _, endpoint := range c.endpoints {
		jobComponent := string(c.component) + endpoint.jobSuffix
		targets = append(targets, newPrometheusTarget(dcr.Namespace, dcr.Name, jobComponent, serviceName, GetPortKey(endpoint.portKey), endpoint.path))
	}
	return targets
}

func newPrometheusTarget(namespace, clusterName, component, serviceName, portName, path string) prometheusTarget {
	return prometheusTarget{
		jobName:     fmt.Sprintf("%s/%s/%s", namespace, clusterName, component),
		namespace:   namespace,
		serviceName: serviceName,
		portName:    portName,
		path:        path,
	}
}

func prometheusOwnerReference(apiVersion, kind, name string, uid types.UID) metav1.OwnerReference {
	controller := true
	return metav1.OwnerReference{APIVersion: apiVersion, Kind: kind, Name: name, UID: uid, Controller: &controller}
}

func buildPrometheusAdditionalSecret(clusterName, namespace string, labels map[string]string, owner metav1.OwnerReference, targets []prometheusTarget) (*corev1.Secret, error) {
	configs := make([]prometheusScrapeConfig, 0, len(targets))
	for _, target := range targets {
		configs = append(configs, target.scrapeConfig())
	}
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(configs); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	data := buffer.Bytes()

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            PrometheusAdditionalSecretName(clusterName),
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Data: map[string][]byte{PrometheusAdditionalConfigKey: data},
	}, nil
}

func (t prometheusTarget) scrapeConfig() prometheusScrapeConfig {
	return prometheusScrapeConfig{
		JobName:     t.jobName,
		MetricsPath: t.path,
		KubernetesSDConfigs: []prometheusK8sSD{{
			Role:       "endpoints",
			Namespaces: prometheusSDNamespaces{Names: []string{t.namespace}},
		}},
		RelabelConfigs: []prometheusRelabel{
			{
				SourceLabels: []string{"__meta_kubernetes_service_name"},
				Regex:        t.serviceName,
				Action:       "keep",
			},
			{
				SourceLabels: []string{"__meta_kubernetes_endpoint_port_name"},
				Regex:        t.portName,
				Action:       "keep",
			},
			{
				SourceLabels: []string{"__meta_kubernetes_namespace"},
				TargetLabel:  "namespace",
				Action:       "replace",
			},
			{
				SourceLabels: []string{"__meta_kubernetes_pod_name"},
				TargetLabel:  "pod",
				Action:       "replace",
			},
		},
	}
}
