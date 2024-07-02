package resource

import (
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"strings"
)

func BuildDMSInternalService(ddm *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType, config map[string]interface{}) corev1.Service {
	labels := mv1.GenerateInternalServiceLabels(ddm, componentType)
	selector := mv1.GenerateServiceSelector(ddm, componentType)
	//the k8s service type.
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mv1.GenerateInternalCommunicateServiceName(ddm, componentType),
			Namespace: ddm.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: ddm.APIVersion,
					Kind:       ddm.Kind,
					Name:       ddm.Name,
					UID:        ddm.UID,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				getDMSInternalServicePort(config, componentType),
			},

			Selector: selector,
			//value = true, Pod don't need to become ready that be search by domain.
			PublishNotReadyAddresses: true,
		},
	}

}

func getDMSInternalServicePort(config map[string]interface{}, componentType mv1.ComponentType) corev1.ServicePort {
	switch componentType {
	case mv1.Component_MS:
		return corev1.ServicePort{
			Name:       GetDMSPortKey(MS_BRPC_LISTEN_PORT),
			Port:       GetPort(config, MS_BRPC_LISTEN_PORT),
			TargetPort: intstr.FromInt(int(GetPort(config, HEARTBEAT_SERVICE_PORT))),
		}
	default:
		klog.Infof("getDMSInternalServicePort not supported the type %s", componentType)
		return corev1.ServicePort{}
	}
}

func GetDMSContainerPorts(config map[string]interface{}, componentType mv1.ComponentType) []corev1.ContainerPort {
	switch componentType {
	case mv1.Component_MS:
		return getMSContainerPorts(config)
	default:
		klog.Infof("GetDMSContainerPorts the componentType %s not supported.", componentType)
		return []corev1.ContainerPort{}
	}
}

func getMSContainerPorts(config map[string]interface{}) []corev1.ContainerPort {
	return []corev1.ContainerPort{{
		Name:          GetPortKey(BRPC_LISTEN_PORT),
		ContainerPort: GetPort(config, MS_BRPC_LISTEN_PORT),
		Protocol:      corev1.ProtocolTCP,
	}}
}

func GetDMSPortKey(configKey string) string {
	switch configKey {
	case MS_BRPC_LISTEN_PORT:
		return strings.ReplaceAll(MS_BRPC_LISTEN_PORT, "_", "-")
	case RC_BRPC_LISTEN_PORT:
		return strings.ReplaceAll(MS_BRPC_LISTEN_PORT, "_", "-")
	default:
		return ""
	}
}

func DMSServiceDeepEqual(newSvc, oldSvc *corev1.Service) bool {
	var newHashValue, oldHashValue string
	if _, ok := newSvc.Annotations[mv1.ComponentResourceHash]; ok {
		newHashValue = newSvc.Annotations[mv1.ComponentResourceHash]
	} else {
		newHashService := dmsServiceHashObject(newSvc)
		newHashValue = hash.HashObject(newHashService)
	}

	if _, ok := oldSvc.Annotations[mv1.ComponentResourceHash]; ok {
		oldHashValue = oldSvc.Annotations[mv1.ComponentResourceHash]
	} else {
		oldHashService := dmsServiceHashObject(oldSvc)
		oldHashValue = hash.HashObject(oldHashService)
	}

	// set hash value in annotation for avoiding deep equal.
	newSvc.Annotations = mergeMaps(newSvc.Annotations, map[string]string{mv1.ComponentResourceHash: newHashValue})
	return newHashValue == oldHashValue &&
		newSvc.Namespace == oldSvc.Namespace
}

// hash service for diff new generate service and old service in kubernetes.
func dmsServiceHashObject(svc *corev1.Service) hashService {
	annos := make(map[string]string, len(svc.Annotations))
	//for support service annotations, avoid hash value in annotations interfere equal comparison.
	for key, value := range svc.Annotations {
		if key == mv1.ComponentResourceHash {
			continue
		}

		annos[key] = value
	}

	return hashService{
		name:        svc.Name,
		namespace:   svc.Namespace,
		ports:       svc.Spec.Ports,
		selector:    svc.Spec.Selector,
		serviceType: svc.Spec.Type,
		labels:      svc.Labels,
		annotations: annos,
	}
}
