package ms

import (
	"context"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
)

var (
	DefaultMetaserviceNumber int32 = 2
)

func (dmc *Controller) buildMSPodTemplateSpec(dms *mv1.DorisDisaggregatedMetaService) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewDMSPodTemplateSpec(dms, mv1.Component_MS)
	var containers []corev1.Container
	config, _ := dmc.GetMSConfig(context.Background(), dms.Spec.MS.ConfigMaps, dms.Namespace, mv1.Component_MS)
	msContainer := dmc.msContainer(dms, config)
	containers = append(containers, msContainer)
	containers = resource.ApplySecurityContext(containers, dms.Spec.MS.ContainerSecurityContext)
	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

func (dmc *Controller) msContainer(dms *mv1.DorisDisaggregatedMetaService, config map[string]interface{}) corev1.Container {
	c := resource.NewDMSBaseMainContainer(dms, config, mv1.Component_MS)
	if dms.Spec.MS.Replicas == nil {
		dms.Spec.MS.Replicas = resource.GetInt32Pointer(DefaultMetaserviceNumber)
	}

	ports := resource.GetDMSContainerPorts(config, mv1.Component_MS)
	c.Name = "disaggregated-metaservice"
	c.Ports = append(c.Ports, ports...)

	return c
}
