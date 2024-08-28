package metaservice

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (dms *DisaggregatedMSController) newService(ddc *dv1.DorisDisaggregatedCluster, confMap map[string]interface{}) *corev1.Service {
	labels := dms.newMSSchedulerLabels(ddc.Name)
	selector := dms.newMSPodsSelector(ddc.Name)
	spec := ddc.Spec.MetaService
	exportService := spec.Service

	svc := dms.NewDefaultService(ddc)
	svc.Name = ddc.GetMSServiceName()
	svc.Namespace = ddc.Namespace
	svc.Labels = labels

	ports := dms.newMSServicePorts(confMap, exportService)

	svc.Spec = corev1.ServiceSpec{
		Selector: selector,
		Ports:    ports,
	}

	// The external load balancer provided by the cloud provider may cause the client IP received by the service to change.
	if exportService != nil && exportService.Type == corev1.ServiceTypeLoadBalancer {
		svc.Spec.SessionAffinity = corev1.ServiceAffinityNone
	}

	if exportService != nil && exportService.Type != "" {
		svc.Spec.Type = exportService.Type
	}
	if exportService != nil {
		svc.Annotations = exportService.Annotations
	}

	return svc
}

func (dms *DisaggregatedMSController) newMSServicePorts(config map[string]interface{}, svcConf *dv1.ExportService) []corev1.ServicePort {
	brpcPort := resource.GetPort(config, resource.BRPC_LISTEN_PORT)
	ports := []corev1.ServicePort{
		{
			Name:       resource.GetPortKey(resource.BRPC_LISTEN_PORT),
			Port:       brpcPort,
			TargetPort: intstr.FromInt(int(brpcPort)),
		},
	}

	if svcConf == nil || svcConf.Type != corev1.ServiceTypeNodePort {
		return ports
	}

	for i, _ := range ports {
		for j, _ := range svcConf.PortMaps {
			if ports[i].Port == svcConf.PortMaps[j].TargetPort {
				ports[i].NodePort = svcConf.PortMaps[j].NodePort
			}
		}
	}

	return ports
}
