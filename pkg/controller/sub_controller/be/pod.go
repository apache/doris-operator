package be

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"

	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
)

func (be *Controller) buildBEPodTemplateSpec(dcr *v1.DorisCluster) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewPodTemplateSpec(dcr, v1.Component_BE)
	be.addFeAntiAffinity(&podTemplateSpec)

	var containers []corev1.Container
	containers = append(containers, podTemplateSpec.Spec.Containers...)
	beContainer := be.beContainer(dcr)
	containers = append(containers, beContainer)
	containers = resource.ApplySecurityContext(containers, dcr.Spec.BeSpec.ContainerSecurityContext)
	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

// be pods add fe anti affinity for prefer deploy fe and be on different nodes.
func (be *Controller) addFeAntiAffinity(tplSpec *corev1.PodTemplateSpec) {
	preferedScheduleTerm := corev1.WeightedPodAffinityTerm{
		Weight: 80,
		PodAffinityTerm: corev1.PodAffinityTerm{
			TopologyKey: resource.NODE_TOPOLOGYKEY,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					resource.NODE_TOPOLOGYKEY: string(v1.Component_FE),
				},
			},
		},
	}

	if tplSpec.Spec.Affinity == nil {
		tplSpec.Spec.Affinity = &corev1.Affinity{}
	}
	if tplSpec.Spec.Affinity.PodAntiAffinity == nil {
		tplSpec.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	}

	tplSpec.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(tplSpec.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
		preferedScheduleTerm)
}

func (be *Controller) beContainer(dcr *v1.DorisCluster) corev1.Container {
	config, _ := be.GetConfig(context.Background(), &dcr.Spec.BeSpec.ConfigMapInfo, dcr.Namespace, v1.Component_BE)
	c := resource.NewBaseMainContainer(dcr, config, v1.Component_BE)
	addr, port := v1.GetConfigFEAddrForAccess(dcr, v1.Component_BE)
	var feConfig map[string]interface{}
	//if fe addr not config, we should use external service as addr and port get from fe config.
	if addr == "" {
		if dcr.Spec.FeSpec != nil {
			feConfig, _ = be.GetConfig(context.Background(), &dcr.Spec.FeSpec.ConfigMapInfo, dcr.Namespace, v1.Component_FE)
		}

		addr = v1.GenerateExternalServiceName(dcr, v1.Component_FE)
	}

	feQueryPort := strconv.FormatInt(int64(resource.GetPort(feConfig, resource.QUERY_PORT)), 10)
	if port != -1 {
		feQueryPort = strconv.FormatInt(int64(port), 10)
	}

	ports := resource.GetContainerPorts(config, v1.Component_BE)
	c.Name = "be"
	c.Ports = append(c.Ports, ports...)
	c.Env = append(c.Env, corev1.EnvVar{
		Name:  resource.ENV_FE_ADDR,
		Value: addr,
	}, corev1.EnvVar{
		Name:  resource.ENV_FE_PORT,
		Value: feQueryPort,
	})

	return c
}
