package disaggregated_fe

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (dfc *DisaggregatedFEController) newService(ddc *dv1.DorisDisaggregatedCluster, cvs map[string]interface{}) *corev1.Service {
	svcConf := ddc.Spec.FeSpec.CommonSpec.Service
	ports := newFEServicePorts(cvs, svcConf)
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ddc.GetFEServiceName(),
			Namespace: ddc.Namespace,
			Labels:    dfc.newFESchedulerLabels(ddc.Namespace),
		},
		Spec: corev1.ServiceSpec{
			Selector: dfc.newFEPodsSelector(ddc.Name),
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    ports,
		},
	}

	if svcConf != nil && svcConf.Type != "" {
		svc.Spec.Type = svcConf.Type
	}

	return &svc
}

// new ports by start config that mounted into container by configMap.
func newFEServicePorts(config map[string]interface{}, svcConf *dv1.ExportService) []corev1.ServicePort {
	httpPort := resource.GetPort(config, resource.HTTP_PORT)
	rpcPort := resource.GetPort(config, resource.RPC_PORT)
	queryPort := resource.GetPort(config, resource.QUERY_PORT)
	editPort := resource.GetPort(config, resource.EDIT_LOG_PORT)
	ports := []corev1.ServicePort{
		{
			Port: httpPort, TargetPort: intstr.FromInt(int(httpPort)), Name: resource.GetPortKey(resource.HTTP_PORT),
		}, {
			Port: rpcPort, TargetPort: intstr.FromInt(int(rpcPort)), Name: resource.GetPortKey(resource.RPC_PORT),
		}, {
			Port: queryPort, TargetPort: intstr.FromInt(int(queryPort)), Name: resource.GetPortKey(resource.QUERY_PORT),
		}, {
			Port: editPort, TargetPort: intstr.FromInt(int(editPort)), Name: resource.GetPortKey(resource.EDIT_LOG_PORT),
		}}

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
