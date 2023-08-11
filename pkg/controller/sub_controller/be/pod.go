package be

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

func (be *Controller) buildBEPodTemplateSpec(dcr *v1.DorisCluster) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewPodTemplateSpc(dcr, v1.Component_BE)
	var containers []corev1.Container
	containers = append(containers, podTemplateSpec.Spec.Containers...)
	beContainer := be.beContainer(dcr)
	containers = append(containers, beContainer)
	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

func (be *Controller) beContainer(dcr *v1.DorisCluster) corev1.Container {
	config, _ := be.GetConfig(context.Background(), &dcr.Spec.BeSpec.ConfigMapInfo, dcr.Namespace)
	c := resource.NewBaseMainContainer(dcr, config, v1.Component_BE)
	addr, port := v1.GetConfigFEAddrForAccess(dcr, v1.Component_BE)
	var feConfig map[string]interface{}
	//if fe addr not config, we should use external service as addr and port get from fe config.
	if addr == "" {
		if dcr.Spec.FeSpec != nil {
			feConfig, _ = be.getFeConfig(context.Background(), &dcr.Spec.FeSpec.ConfigMapInfo, dcr.Namespace)
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
