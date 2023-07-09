package cn

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

func (cn *Controller) buildCnPodTemplateSpec(dcr *v1.DorisCluster) corev1.PodTemplateSpec {
	podTemplateSpc := resource.NewPodTemplateSpc(dcr, v1.Component_CN)
	var containers []corev1.Container
	containers = append(containers, podTemplateSpc.Spec.Containers...)
	cnContainer := cn.cnContainer(dcr)
	containers = append(containers, cnContainer)
	podTemplateSpc.Spec.Containers = containers
	return podTemplateSpc
}
func (cn *Controller) cnContainer(dcr *v1.DorisCluster) corev1.Container {
	container := resource.NewBaseMainContainer(dcr.Spec.CnSpec.BaseSpec, v1.Component_CN)
	cnConfig, _ := cn.GetConfig(context.Background(), &dcr.Spec.CnSpec.ConfigMapInfo, dcr.Namespace)
	address := v1.GetConfigFEAddrForAccess(dcr, v1.Component_CN)
	// if address is empty

	var feconfig map[string]interface{}

	// fe query port set has nothing to do with the address
	if dcr.Spec.CnSpec.ConfigMapInfo.ConfigMapName != "" && dcr.Spec.CnSpec.ConfigMapInfo.ResolveKey != "" {
		feconfig, _ = cn.getFeConfig(context.Background(), &dcr.Spec.CnSpec.ConfigMapInfo, dcr.Namespace)
	}
	cnConfig[resource.QUERY_PORT] = strconv.FormatInt(int64(resource.GetPort(feconfig, resource.QUERY_PORT)), 10)

	ports := resource.GetContainerPorts(cnConfig, v1.Component_CN)
	container.Name = "cn"
	container.Ports = append(container.Ports, ports...)
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  resource.ENV_FE_ADDR,
		Value: address,
	})
	return container
}
