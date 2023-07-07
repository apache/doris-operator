package v1

import (
	"github.com/selectdb/doris-operator/pkg/common/utils/metadata"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
)

// the annotation key
const (
	//ComponentsResourceHash the component hash
	ComponentResourceHash string = "app.doris.components/hash"

	ComponentReplicasEmpty string = "app.dois.components/replica/empty"
)

// the labels key
const (
	// ComponentLabelKey is Kubernetes recommended label key, it represents the component within the architecture
	ComponentLabelKey string = "app.kubernetes.io/component"
	// NameLabelKey is Kubernetes recommended label key, it represents the name of the application
	NameLabelKey string = "app.kubernetes.io/name"

	//OwnerReference list object depended by this object
	OwnerReference string = "app.doris.ownerreference/name"

	ServiceRoleForCluster string = "app.doris.service/role"
)

type ServiceRole string

const (
	Service_Role_Access   ServiceRole = "access"
	Service_Role_Internal ServiceRole = "internal"
)

type ComponentType string

const (
	Component_FE     ComponentType = "fe"
	Component_BE     ComponentType = "be"
	Component_CN     ComponentType = "cn"
	Component_Broker ComponentType = "broker"
)

func GenerateExternalServiceName(dcr *DorisCluster, componentType ComponentType) string {
	switch componentType {
	case Component_FE:
		return dcr.Name + "-" + string(Component_FE) + "-service"
	case Component_CN:
		return dcr.Name + "-" + string(Component_CN) + "-service"
	case Component_BE:
		return dcr.Name + "-" + string(Component_BE) + "-service"
	default:
		return ""
	}
}

func GenerateComponentStatefulSetName(dcr *DorisCluster, componentType ComponentType) string {
	switch componentType {
	case Component_FE:
		return feStatefulSetName(dcr)
	case Component_BE:
		return beStatefulSetName(dcr)
	case Component_CN:
		return cnStatefulSetName(dcr)
	case Component_Broker:
		return brokerStatefulSetName(dcr)
	default:
		return ""
	}
}

func beStatefulSetName(dcr *DorisCluster) string {
	return dcr.Name + "-" + string(Component_BE)
}

func cnStatefulSetName(dcr *DorisCluster) string {
	return dcr.Name + "-" + string(Component_CN)
}

func feStatefulSetName(dcr *DorisCluster) string {
	return dcr.Name + "-" + string(Component_FE)
}

func brokerStatefulSetName(dcr *DorisCluster) string {
	return dcr.Name + "-" + string(Component_Broker)
}

func GenerateExternalServiceLabels(dcr *DorisCluster, componentType ComponentType) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = dcr.Name
	labels[ComponentLabelKey] = string(componentType)
	labels[ServiceRoleForCluster] = string(Service_Role_Access)
	//once the labels updated, the statefulset will enter into a not reconcile state.
	//labels.AddLabel(src.Labels)
	return labels
}

const (
	SEARCH_SERVICE_SUFFIX = "-internal"
)

func GenerateInternalCommunicateServiceName(dcr *DorisCluster, componentType ComponentType) string {
	return dcr.Name + string(componentType) + SEARCH_SERVICE_SUFFIX
}

func GenerateInternalServiceLabels(dcr *DorisCluster, componentType ComponentType) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = dcr.Name
	labels[ComponentLabelKey] = string(componentType)
	labels[ServiceRoleForCluster] = string(Service_Role_Internal)
	//once the labels updated, the statefulset will enter into a not reconcile state.
	//labels.AddLabel(src.Labels)
	return labels
}

func GenerateServiceSelector(dcr *DorisCluster, componentType ComponentType) metadata.Labels {
	return GenerateStatefulSetSelector(dcr, componentType)
}

func GenerateStatefulSetSelector(dcr *DorisCluster, componentType ComponentType) metadata.Labels {
	switch componentType {
	case Component_FE:
		return feStatefulSetSelector(dcr)
	case Component_BE:
		return beStatefulSetSelector(dcr)
	case Component_CN:
		return cnStatefulSetSelector(dcr)
	case Component_Broker:
		return brokerStatefulSetSelector(dcr)
	default:
		return metadata.Labels{}
	}
}

func feStatefulSetSelector(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = feStatefulSetName(dcr)
	labels[ComponentLabelKey] = string(Component_FE)
	return labels
}

func cnStatefulSetSelector(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = cnStatefulSetName(dcr)
	labels[ComponentLabelKey] = string(Component_CN)
	return labels
}

func beStatefulSetSelector(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = beStatefulSetName(dcr)
	labels[ComponentLabelKey] = string(Component_BE)
	return labels
}

func brokerStatefulSetSelector(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = brokerStatefulSetName(dcr)
	labels[ComponentLabelKey] = string(Component_BE)
	return labels
}

func GenerateStatefulSetLabels(dcr *DorisCluster, componentType ComponentType) metadata.Labels {
	switch componentType {
	case Component_FE:
		return feStatefulSetLabels(dcr)
	case Component_BE:
		return beStatefulSetLabels(dcr)
	case Component_CN:
		return cnStatefulSetLabels(dcr)
	case Component_Broker:
		return brokerStatefulSetLabels(dcr)
	default:
		return metadata.Labels{}
	}
}

func feStatefulSetLabels(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = dcr.Name
	labels[ComponentLabelKey] = string(Component_FE)
	//once the labels updated, the statefulset will enter into a not reconcile state.
	//labels.AddLabel(src.Labels)
	return labels
}

func beStatefulSetLabels(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = dcr.Name
	labels[ComponentLabelKey] = string(Component_BE)
	//once the labels updated, the statefulset will enter into a not reconcile state.
	//labels.AddLabel(src.Labels)
	return labels
}

func cnStatefulSetLabels(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = dcr.Name
	labels[ComponentLabelKey] = string(Component_CN)
	//once the labels updated, the statefulset will enter into a not reconcile state.
	//labels.AddLabel(src.Labels)
	return labels
}

func brokerStatefulSetLabels(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = dcr.Name
	labels[ComponentLabelKey] = string(Component_Broker)
	//once the labels updated, the statefulset will enter into a not reconcile state.
	//labels.AddLabel(src.Labels)
	return labels
}

func GetPodLabels(dcr *DorisCluster, componentType ComponentType) metadata.Labels {
	switch componentType {
	case Component_FE:
		return getFEPodLabels(dcr)
	case Component_BE:
		return getBEPodLabels(dcr)
	case Component_CN:
		return getCNPodLabels(dcr)
	default:
		klog.Infof("GetPodLabels the componentType %s is not supported.", componentType)
		return metadata.Labels{}
	}
}

func getFEPodLabels(dcr *DorisCluster) metadata.Labels {
	labels := feStatefulSetSelector(dcr)
	labels.AddLabel(dcr.Spec.FeSpec.PodLabels)
	return labels
}

func getBEPodLabels(dcr *DorisCluster) metadata.Labels {
	labels := beStatefulSetLabels(dcr)
	labels.AddLabel(dcr.Spec.BeSpec.PodLabels)
	return labels
}

func getCNPodLabels(dcr *DorisCluster) metadata.Labels {
	labels := cnStatefulSetLabels(dcr)
	labels.AddLabel(dcr.Spec.CnSpec.PodLabels)
	return labels
}

func GetConfigFEAddrForAccess(dcr *DorisCluster, componentType ComponentType) string {
	switch componentType {
	case Component_FE:
		return getFEAccessAddrForFEADD(dcr)
	case Component_BE:
		return getFEAddrForBackends(dcr)
	case Component_CN:
		return getFeAddrForComputeNodes(dcr)
	default:
		klog.Infof("GetFEAddrForAccess the componentType %s not supported.", componentType)
		return ""
	}
}

func getFEAccessAddrForFEADD(dcr *DorisCluster) string {
	if dcr.Spec.FeSpec != nil && dcr.Spec.FeSpec.FeAddress != nil {
		if len(dcr.Spec.FeSpec.FeAddress.Endpoints) != 0 {
			return getEndpointsToString(dcr.Spec.FeSpec.FeAddress.Endpoints)
		}

		return dcr.Spec.FeSpec.FeAddress.ServiceName
	}

	return ""
}

func getEndpointsToString(eps []Endpoint) string {
	if len(eps) == 0 {
		return ""
	}

	var addrs []string
	for _, ep := range eps {
		addrs = append(addrs, ep.Address+":"+strconv.Itoa(ep.Port))
	}

	return strings.Join(addrs, ";")
}

func getFEAddrForBackends(dcr *DorisCluster) string {
	if dcr.Spec.BeSpec != nil && dcr.Spec.BeSpec.FeAddress != nil {
		if len(dcr.Spec.BeSpec.FeAddress.Endpoints) != 0 {
			return getEndpointsToString(dcr.Spec.BeSpec.FeAddress.Endpoints)
		}

		return dcr.Spec.CnSpec.FeAddress.ServiceName
	}

	return ""
}

func getFeAddrForComputeNodes(dcr *DorisCluster) string {
	if dcr.Spec.CnSpec != nil && dcr.Spec.CnSpec.FeAddress != nil {
		if len(dcr.Spec.CnSpec.FeAddress.Endpoints) != 0 {
			return getEndpointsToString(dcr.Spec.CnSpec.FeAddress.Endpoints)
		}

		return dcr.Spec.CnSpec.FeAddress.ServiceName
	}

	return ""
}
