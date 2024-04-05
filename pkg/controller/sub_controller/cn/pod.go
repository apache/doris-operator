package cn

import (
	"context"
	"strconv"

	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
)

func (cn *Controller) buildCnPodTemplateSpec(dcr *v1.DorisCluster) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewPodTemplateSpec(dcr, v1.Component_CN)
	var containers []corev1.Container
	containers = append(containers, podTemplateSpec.Spec.Containers...)
	cnContainer := cn.cnContainer(dcr)
	containers = append(containers, cnContainer)

	containers = resource.ApplySecurityContext(containers, dcr.Spec.CnSpec.ContainerSecurityContext)
	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

func (cn *Controller) cnContainer(dcr *v1.DorisCluster) corev1.Container {
	cnConfig, _ := cn.GetConfig(context.Background(), &dcr.Spec.CnSpec.ConfigMapInfo, dcr.Namespace)
	container := resource.NewBaseMainContainer(dcr, cnConfig, v1.Component_CN)
	address, port := v1.GetConfigFEAddrForAccess(dcr, v1.Component_CN)
	// if address is empty
	var feConfig map[string]interface{}
	if address == "" {
		if dcr.Spec.FeSpec != nil {
			//if fe exist, get fe config.
			feConfig, _ = cn.getFeConfig(context.Background(), &dcr.Spec.FeSpec.ConfigMapInfo, dcr.Namespace)
		}

		address = v1.GenerateExternalServiceName(dcr, v1.Component_FE)
	}
	feQueryPort := strconv.FormatInt(int64(resource.GetPort(feConfig, resource.QUERY_PORT)), 10)
	if port != -1 {
		feQueryPort = strconv.FormatInt(int64(port), 10)
	}

	ports := resource.GetContainerPorts(cnConfig, v1.Component_CN)
	container.Name = "cn"
	container.Ports = append(container.Ports, ports...)
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  resource.ENV_FE_ADDR,
		Value: address,
	}, corev1.EnvVar{
		Name:  resource.ENV_FE_PORT,
		Value: feQueryPort,
	}, corev1.EnvVar{
		Name:  resource.COMPONENT_TYPE,
		Value: "COMPUTE",
	})
	return container
}
