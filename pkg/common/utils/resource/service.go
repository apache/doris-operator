package resource

import (
	dorisv1 "github.com/selectdb/doris-operator/api/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"strings"
)

// HashService service hash components
type hashService struct {
	name      string
	namespace string
	ports     []corev1.ServicePort
	selector  map[string]string
	//deal with external access load balancer.
	//serviceType corev1.ServiceType
	labels map[string]string
}

func BuildInternalService(dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType, config map[string]interface{}) corev1.Service {
	labels := dorisv1.GenerateInternalServiceLabels(dcr, componentType)
	selector := dorisv1.GenerateServiceSelector(dcr, componentType)
	//the k8s service type.
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dorisv1.GenerateInternalCommunicateServiceName(dcr, componentType),
			Namespace: dcr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				getInternalServicePort(config, componentType),
			},

			Selector: selector,
			//value = true, Pod don't need to become ready that be search by domain.
			PublishNotReadyAddresses: true,
		},
	}

}

func getInternalServicePort(config map[string]interface{}, componentType dorisv1.ComponentType) corev1.ServicePort {
	switch componentType {
	case dorisv1.Component_FE:
		return corev1.ServicePort{
			Name:       strings.ReplaceAll(QUERY_PORT, "_", "-"),
			Port:       GetPort(config, QUERY_PORT),
			TargetPort: intstr.FromInt(int(GetPort(config, QUERY_PORT))),
		}
	case dorisv1.Component_BE:
		return corev1.ServicePort{
			Name:       strings.ReplaceAll(HEARTBEAT_SERVICE_PORT, "_", "-"),
			Port:       GetPort(config, HEARTBEAT_SERVICE_PORT),
			TargetPort: intstr.FromInt(int(GetPort(config, HEARTBEAT_SERVICE_PORT))),
		}

	default:
		klog.Infof("getInternalServicePort not supported the type %s", componentType)
		return corev1.ServicePort{}
	}
}

// BuildExternalService build the external service. not have selector
func BuildExternalService(dcr *dorisv1.DorisCluster, componentType dorisv1.ComponentType, config map[string]interface{}) corev1.Service {
	labels := dorisv1.GenerateExternalServiceLabels(dcr, componentType)
	selector := dorisv1.GenerateServiceSelector(dcr, componentType)
	//the k8s service type.
	var ports []corev1.ServicePort
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dorisv1.GenerateExternalServiceName(dcr, componentType),
			Namespace: dcr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
		},
	}

	switch componentType {
	case dorisv1.Component_FE:
		setServiceType(dcr.Spec.FeSpec.Service, &svc)
		ports = getFeServicePorts(config)
	case dorisv1.Component_BE:
		setServiceType(dcr.Spec.BeSpec.Service, &svc)
		ports = getBeServicePorts(config)
	case dorisv1.Component_CN:
		setServiceType(dcr.Spec.FeSpec.Service, &svc)
		ports = getCnServicePorts(config)
	default:
		klog.Infof("BuildExternalService componentType %s not supported.")
	}

	or := metav1.OwnerReference{
		APIVersion: dcr.APIVersion,
		Kind:       dcr.Kind,
		Name:       dcr.Name,
		UID:        dcr.UID,
	}
	svc.OwnerReferences = []metav1.OwnerReference{or}
	hso := serviceHashObject(&svc)
	anno := map[string]string{}
	anno[dorisv1.ComponentResourceHash] = hash.HashObject(hso)
	svc.Annotations = anno
	svc.Spec.Ports = ports
	return svc
}

func getFeServicePorts(config map[string]interface{}) (ports []corev1.ServicePort) {
	httpPort := GetPort(config, HTTP_PORT)
	rpcPort := GetPort(config, RPC_PORT)
	queryPort := GetPort(config, QUERY_PORT)
	editPort := GetPort(config, EDIT_LOG_PORT)
	ports = append(ports, corev1.ServicePort{
		Port: httpPort, TargetPort: intstr.FromInt(int(httpPort)), Name: "http",
	}, corev1.ServicePort{
		Port: rpcPort, TargetPort: intstr.FromInt(int(rpcPort)), Name: "rpc",
	}, corev1.ServicePort{
		Port: queryPort, TargetPort: intstr.FromInt(int(queryPort)), Name: "query",
	}, corev1.ServicePort{
		Port: editPort, TargetPort: intstr.FromInt(int(editPort)), Name: "edit-log"})

	return
}

func getBeServicePorts(config map[string]interface{}) (ports []corev1.ServicePort) {
	bePort := GetPort(config, BE_PORT)
	webseverPort := GetPort(config, WEBSERVER_PORT)
	heartPort := GetPort(config, HEARTBEAT_SERVICE_PORT)
	brpcPort := GetPort(config, BRPC_PORT)

	ports = append(ports, corev1.ServicePort{
		Port: bePort, TargetPort: intstr.FromInt(int(bePort)), Name: "be",
	}, corev1.ServicePort{
		Port: webseverPort, TargetPort: intstr.FromInt(int(webseverPort)), Name: "webserver",
	}, corev1.ServicePort{
		Port: heartPort, TargetPort: intstr.FromInt(int(heartPort)), Name: "heartbeat",
	}, corev1.ServicePort{
		Port: brpcPort, TargetPort: intstr.FromInt(int(brpcPort)), Name: "brpc",
	})

	return
}

func GetContainerPorts(config map[string]interface{}, componentType dorisv1.ComponentType) []corev1.ContainerPort {
	switch componentType {
	case dorisv1.Component_FE:
		return getFeContainerPorts(config)
	case dorisv1.Component_BE:
		return getBeContainerPorts(config)
	default:
		klog.Infof("GetContainerPorts the componentType %s not supported.", componentType)
		return []corev1.ContainerPort{}
	}
}

func getFeContainerPorts(config map[string]interface{}) []corev1.ContainerPort {
	return []corev1.ContainerPort{{
		Name:          strings.ReplaceAll(HTTP_PORT, "_", "-"),
		ContainerPort: GetPort(config, HTTP_PORT),
		Protocol:      corev1.ProtocolTCP,
	}, {
		Name:          "rpc-port",
		ContainerPort: GetPort(config, RPC_PORT),
		Protocol:      corev1.ProtocolTCP,
	}, {
		Name:          "query-port",
		ContainerPort: GetPort(config, QUERY_PORT),
		Protocol:      corev1.ProtocolTCP,
	}}
}

func getBeContainerPorts(config map[string]interface{}) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          strings.ReplaceAll(BE_PORT, "_", "-"),
			ContainerPort: GetPort(config, BE_PORT),
		}, {
			Name:          strings.ReplaceAll(WEBSERVER_PORT, "_", "-"),
			ContainerPort: GetPort(config, WEBSERVER_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          strings.ReplaceAll(HEARTBEAT_SERVICE_PORT, "_", "-"),
			ContainerPort: GetPort(config, HEARTBEAT_SERVICE_PORT),
			Protocol:      corev1.ProtocolTCP,
		}, {
			Name:          strings.ReplaceAll(BRPC_PORT, "_", "-"),
			ContainerPort: GetPort(config, BRPC_PORT),
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

func getCnServicePorts(config map[string]interface{}) (ports []corev1.ServicePort) {
	thriftPort := GetPort(config, THRIFT_PORT)
	webseverPort := GetPort(config, WEBSERVER_PORT)
	heartPort := GetPort(config, HEARTBEAT_SERVICE_PORT)
	brpcPort := GetPort(config, BRPC_PORT)
	ports = append(ports, corev1.ServicePort{
		Port: thriftPort, TargetPort: intstr.FromInt(int(thriftPort)), Name: "thrift",
	}, corev1.ServicePort{
		Port: webseverPort, TargetPort: intstr.FromInt(int(webseverPort)), Name: "webserver",
	}, corev1.ServicePort{
		Port: heartPort, TargetPort: intstr.FromInt(int(heartPort)), Name: "heartbeat",
	}, corev1.ServicePort{
		Port: brpcPort, TargetPort: intstr.FromInt(int(brpcPort)), Name: "brpc",
	})

	return
}

func setServiceType(svc *dorisv1.ExportService, service *corev1.Service) {
	service.Spec.Type = corev1.ServiceTypeClusterIP
	if svc != nil && svc.Type != "" {
		service.Spec.Type = svc.Type
	}

	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && svc.LoadBalancerIP != "" {
		service.Spec.LoadBalancerIP = svc.LoadBalancerIP
	}
}

func ServiceDeepEqual(nsvc, oldsvc *corev1.Service) bool {
	var nhsvcValue, ohsvcValue string
	if _, ok := nsvc.Annotations[dorisv1.ComponentResourceHash]; ok {
		nhsvcValue = nsvc.Annotations[dorisv1.ComponentResourceHash]
	} else {
		nhsvc := serviceHashObject(nsvc)
		nhsvcValue = hash.HashObject(nhsvc)
	}

	if _, ok := oldsvc.Annotations[dorisv1.ComponentResourceHash]; ok {
		ohsvcValue = oldsvc.Annotations[dorisv1.ComponentResourceHash]
	} else {
		ohsvc := serviceHashObject(oldsvc)
		ohsvcValue = hash.HashObject(ohsvc)
	}

	return nhsvcValue == ohsvcValue &&
		nsvc.Namespace == oldsvc.Namespace /*&& oldGeneration == oldsvc.Generation*/
}

func serviceHashObject(svc *corev1.Service) hashService {
	return hashService{
		name:      svc.Name,
		namespace: svc.Namespace,
		ports:     svc.Spec.Ports,
		selector:  svc.Spec.Selector,
		labels:    svc.Labels,
	}
}

func HaveEqualOwnerReference(svc1 *corev1.Service, svc2 *corev1.Service) bool {
	set := make(map[string]bool)
	for _, o := range svc1.OwnerReferences {
		key := o.Kind + o.Name
		set[key] = true

	}

	for _, o := range svc2.OwnerReferences {
		key := o.Kind + o.Name
		if _, ok := set[key]; ok {
			return true
		}
	}
	return false
}
