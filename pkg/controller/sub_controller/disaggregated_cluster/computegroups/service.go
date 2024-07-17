package computegroups

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (dccs *DisaggregatedComputeGroupsController) newService(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup, cvs map[string]interface{}) *corev1.Service {
	cgClusterId := ddc.GetCGClusterId(cg)
	svcConf := cg.CommonSpec.Service
	sps := newComputeServicePorts(cvs, svcConf)
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            ddc.GetCGServiceName(cg),
			Namespace:       ddc.Namespace,
			Labels:          dccs.newCG2LayerSchedulerLabels(ddc.Namespace, cgClusterId),
			OwnerReferences: []metav1.OwnerReference{resource.GetOwnerReference(ddc)},
		},
		Spec: corev1.ServiceSpec{
			Selector: dccs.newCGPodsSelector(ddc.Name, cgClusterId),
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    sps,
		},
	}

	if svcConf != nil && svcConf.Type != "" {
		svc.Spec.Type = svcConf.Type
	}

	return &svc
}

// new ports by start config that mounted into container by configMap.
func newComputeServicePorts(cvs map[string]interface{}, svcConf *dv1.ExportService) []corev1.ServicePort {
	bePort := resource.GetPort(cvs, resource.BE_PORT)
	webserverPort := resource.GetPort(cvs, resource.WEBSERVER_PORT)
	heartbeatPort := resource.GetPort(cvs, resource.HEARTBEAT_SERVICE_PORT)
	brpcPort := resource.GetPort(cvs, resource.BRPC_PORT)
	sps := []corev1.ServicePort{{
		Name:       resource.GetPortKey(resource.BE_PORT),
		TargetPort: intstr.FromInt(int(bePort)),
		Port:       bePort,
	}, {
		Name:       resource.GetPortKey(resource.WEBSERVER_PORT),
		TargetPort: intstr.FromInt(int(webserverPort)),
		Port:       webserverPort,
	}, {
		Name:       resource.GetPortKey(resource.HEARTBEAT_SERVICE_PORT),
		TargetPort: intstr.FromInt(int(heartbeatPort)),
		Port:       heartbeatPort,
	}, {
		Name:       resource.GetPortKey(resource.BRPC_PORT),
		TargetPort: intstr.FromInt(int(brpcPort)),
		Port:       brpcPort,
	}}

	if svcConf == nil || svcConf.Type != corev1.ServiceTypeNodePort {
		return sps
	}

	for i, _ := range sps {
		for j, _ := range svcConf.PortMaps {
			if sps[i].Port == svcConf.PortMaps[j].TargetPort {
				sps[i].NodePort = svcConf.PortMaps[j].NodePort
			}
		}
	}

	return sps
}
