package recycler

import (
	"context"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
)

var (
	DefaultRecyclerNumber int32 = 2
)

func (rc *RecyclerController) buildMSPodTemplateSpec(dms *mv1.DorisDisaggregatedMetaService) corev1.PodTemplateSpec {
	podTemplateSpec := resource.NewDMSPodTemplateSpec(dms, mv1.Component_RC)
	var containers []corev1.Container
	config, _ := rc.GetMSConfig(context.Background(), dms.Spec.Recycler.ConfigMaps, dms.Namespace, mv1.Component_RC)
	msContainer := rc.rcContainer(dms, config)
	containers = append(containers, msContainer)
	containers = resource.ApplySecurityContext(containers, dms.Spec.Recycler.ContainerSecurityContext)
	podTemplateSpec.Spec.Containers = containers
	return podTemplateSpec
}

func (rc *RecyclerController) rcContainer(dms *mv1.DorisDisaggregatedMetaService, config map[string]interface{}) corev1.Container {
	brpcPort := resource.GetPort(config, resource.BRPC_LISTEN_PORT)
	c := resource.NewDMSBaseMainContainer(dms, brpcPort, config, mv1.Component_RC)
	if dms.Spec.MS.Replicas == nil {
		dms.Spec.MS.Replicas = resource.GetInt32Pointer(DefaultRecyclerNumber)
	}

	ports := resource.GetDMSContainerPorts(brpcPort, mv1.Component_RC)
	c.Name = "disaggregated-recyler"
	c.Ports = append(c.Ports, ports...)

	return c
}
