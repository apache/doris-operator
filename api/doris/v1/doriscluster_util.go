// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package v1

import (
	"github.com/apache/doris-operator/pkg/common/utils/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"strings"
)

// the annotation key
const (
	//ComponentsResourceHash the component hash
	ComponentResourceHash string = "app.doris.components/hash"

	FERestartAt string = "apache.doris.fe/restartedAt"
	BERestartAt string = "apache.doris.be/restartedAt"
)

// the labels key
const (
	// ComponentLabelKey is Kubernetes recommended label key, it represents the component within the architecture
	ComponentLabelKey string = "app.kubernetes.io/component"
	// NameLabelKey is Kubernetes recommended label key, it represents the name of the application
	NameLabelKey string = "app.kubernetes.io/name"

	DorisClusterLabelKey string = "app.doris.cluster"

	//OwnerReference list ownerReferences this object
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

var DefaultFeElectionNumber int32 = 3

func GenerateExternalServiceName(dcr *DorisCluster, componentType ComponentType) string {
	switch componentType {
	case Component_FE:
		return dcr.Name + "-" + string(Component_FE) + "-service"
	case Component_CN:
		return dcr.Name + "-" + string(Component_CN) + "-service"
	case Component_BE:
		return dcr.Name + "-" + string(Component_BE) + "-service"
	case Component_Broker:
		return dcr.Name + "-" + string(Component_Broker) + "-service"
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
	return dcr.Name + "-" + string(componentType) + SEARCH_SERVICE_SUFFIX
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
	labels[ComponentLabelKey] = string(Component_Broker)
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
	case Component_Broker:
		return getBrokerPodLabels(dcr)
	default:
		klog.Infof("GetPodLabels the componentType %s is not supported.", componentType)
		return metadata.Labels{}
	}
}

func getFEPodLabels(dcr *DorisCluster) metadata.Labels {
	labels := feStatefulSetSelector(dcr)
	labels.AddLabel(getDefaultLabels(dcr))
	labels.AddLabel(dcr.Spec.FeSpec.PodLabels)
	return labels
}

func getBEPodLabels(dcr *DorisCluster) metadata.Labels {
	labels := beStatefulSetSelector(dcr)
	labels.AddLabel(getDefaultLabels(dcr))
	labels.AddLabel(dcr.Spec.BeSpec.PodLabels)
	return labels
}

func getCNPodLabels(dcr *DorisCluster) metadata.Labels {
	labels := cnStatefulSetSelector(dcr)
	labels.AddLabel(getDefaultLabels(dcr))
	labels.AddLabel(dcr.Spec.CnSpec.PodLabels)
	return labels
}

func getBrokerPodLabels(dcr *DorisCluster) metadata.Labels {
	labels := brokerStatefulSetSelector(dcr)
	labels.AddLabel(getDefaultLabels(dcr))
	labels.AddLabel(dcr.Spec.BrokerSpec.PodLabels)
	return labels
}

func getDefaultLabels(dcr *DorisCluster) metadata.Labels {
	labels := metadata.Labels{}
	labels[DorisClusterLabelKey] = dcr.Name
	return labels
}

func GetConfigFEAddrForAccess(dcr *DorisCluster, componentType ComponentType) (string, int) {
	switch componentType {
	case Component_FE:
		return getFEAccessAddrForFEADD(dcr)
	case Component_BE:
		return getFEAddrForBackends(dcr)
	case Component_CN:
		return getFeAddrForComputeNodes(dcr)
	case Component_Broker:
		return getFeAddrForBroker(dcr)
	default:
		klog.Infof("GetFEAddrForAccess the componentType %s not supported.", componentType)
		return "", -1
	}
}

func getFEAccessAddrForFEADD(dcr *DorisCluster) (string, int) {
	if dcr.Spec.FeSpec != nil && dcr.Spec.FeSpec.FeAddress != nil {
		if len(dcr.Spec.FeSpec.FeAddress.Endpoints.Address) != 0 {
			return getEndpointsToString(dcr.Spec.FeSpec.FeAddress.Endpoints)
		}
	}

	return "", -1
}

func getEndpointsToString(ep Endpoints) (string, int) {
	if len(ep.Address) == 0 {
		return "", -1
	}

	return strings.Join(ep.Address, ","), ep.Port
}

func getFEAddrForBackends(dcr *DorisCluster) (string, int) {
	if dcr.Spec.BeSpec != nil && dcr.Spec.BeSpec.FeAddress != nil {
		if len(dcr.Spec.BeSpec.FeAddress.Endpoints.Address) != 0 {
			return getEndpointsToString(dcr.Spec.BeSpec.FeAddress.Endpoints)
		}

	}

	return getFEAccessAddrForFEADD(dcr)
}

func getFeAddrForComputeNodes(dcr *DorisCluster) (string, int) {
	if dcr.Spec.CnSpec != nil && dcr.Spec.CnSpec.FeAddress != nil {
		if len(dcr.Spec.CnSpec.FeAddress.Endpoints.Address) != 0 {
			return getEndpointsToString(dcr.Spec.CnSpec.FeAddress.Endpoints)
		}
	}

	return getFEAccessAddrForFEADD(dcr)
}

func getFeAddrForBroker(dcr *DorisCluster) (string, int) {
	if dcr.Spec.BrokerSpec != nil && dcr.Spec.BrokerSpec.FeAddress != nil {
		if len(dcr.Spec.BrokerSpec.FeAddress.Endpoints.Address) != 0 {
			return getEndpointsToString(dcr.Spec.BrokerSpec.FeAddress.Endpoints)
		}
	}

	return getFEAccessAddrForFEADD(dcr)
}

// GetClusterSecret get the cluster's adminuser and password through the cluster management account and password configuration in crd
func GetClusterSecret(dcr *DorisCluster, secret *corev1.Secret) (adminUserName, password string) {
	if secret != nil && secret.Data != nil {
		return string(secret.Data["username"]), string(secret.Data["password"])
	}
	// AdminUser was deprecated since 1.4.1
	if dcr.Spec.AdminUser != nil {
		return dcr.Spec.AdminUser.Name, dcr.Spec.AdminUser.Password
	}
	return "root", ""
}

func IsReconcilingStatusPhase(c *ComponentStatus) bool {
	return c.ComponentCondition.Phase == Upgrading ||
		c.ComponentCondition.Phase == Scaling ||
		c.ComponentCondition.Phase == Restarting ||
		c.ComponentCondition.Phase == Reconciling
}

func (dcr *DorisCluster) GetElectionNumber() int32 {
	if dcr.Spec.FeSpec.ElectionNumber != nil {
		return *dcr.Spec.FeSpec.ElectionNumber
	}
	return DefaultFeElectionNumber
}

func GetRestartAnnotationKey(componentType ComponentType) string {
	var restartAnnotationsKey string
	switch componentType {
	case Component_FE:
		restartAnnotationsKey = FERestartAt
	case Component_BE:
		restartAnnotationsKey = BERestartAt
	default:
		klog.Infof("GetRestartAnnotationKey the componentType %s is not supported.", componentType)
	}
	return restartAnnotationsKey

}

func (dcr *DorisCluster) GetComponentStatus(componentType ComponentType) *ComponentStatus {
	switch componentType {
	case Component_FE:
		return dcr.Status.FEStatus
	case Component_BE:
		return dcr.Status.BEStatus
	case Component_CN:
		return &dcr.Status.CnStatus.ComponentStatus
	case Component_Broker:
		return dcr.Status.BrokerStatus
	default:
		klog.Infof("GetComponentStatus the componentType %s is not supported.", componentType)
	}
	return nil
}
