package fe

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

func (fc *Controller) buildFEPodTemplateSpec(dcr *v1.DorisCluster) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewPodTemplateSpc(dcr, v1.Component_FE)
	var containers []corev1.Container
	containers = append(containers, podTemplateSpec.Spec.Containers...)
	config, _ := fc.GetFeConfig(context.Background(), &dcr.Spec.FeSpec.ConfigMapInfo, dcr.Namespace)
	feContainer := fc.feContainer(dcr, config)
	containers = append(containers, feContainer)
	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

func (fc *Controller) feContainer(dcr *v1.DorisCluster, config map[string]interface{}) corev1.Container {
	c := resource.NewBaseMainContainer(dcr.Spec.FeSpec.BaseSpec, v1.Component_FE)
	feAddr := v1.GetConfigFEAddrForAccess(dcr, v1.Component_FE)
	queryPort := resource.GetPort(config, resource.QUERY_PORT)
	//if fe addr not config, use external service as addr, if port not config in configmap use default value.
	if feAddr == "" {
		feAddr = v1.GenerateExternalServiceName(dcr, v1.Component_FE) + ":" + strconv.Itoa(int(queryPort))
	}

	ports := resource.GetContainerPorts(config, v1.Component_FE)
	c.Name = "fe"
	c.Ports = append(c.Ports, ports...)
	c.Env = append(c.Env, corev1.EnvVar{
		Name:  resource.ENV_FE_ADDR,
		Value: feAddr,
	})

	return c
}
