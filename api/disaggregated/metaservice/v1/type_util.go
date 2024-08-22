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
	"github.com/selectdb/doris-operator/pkg/common/utils/metadata"
)

var (
	FDBNameSuffix  = "-foundationdb"
	NameLabelKey   = "disaggregated.metaservice.doris.com/name"
	MsPort         = "5000"
	DefaultMsToken = "greedisgood9999"
)

// the labels key
const (
	//ComponentsResourceHash the component hash
	ComponentResourceHash string = "app.disaggregated.components/hash"

	// ComponentLabelKey is Kubernetes recommended label key, it represents the component within the architecture
	ComponentLabelKey string = "app.kubernetes.io/component"

	DisaggregatedDorisMetaserviceLabelKey string = "doris.disaggregated.metaservice"

	//OwnerReference list ownerReferences this object
	OwnerReference string = "app.disaggregated.ownerreference/name"

	ServiceRoleForCluster string = "app.disaggregated.service/role"
)

type ServiceRole string

const (
	Service_Role_Access ServiceRole = "access"
)

type ComponentType string

const (
	Component_FDB ComponentType = "fdb"
	Component_MS  ComponentType = "metaservice"
	Component_RC  ComponentType = "recycler"
)

const (
	DefaultMetaserviceNumber int32 = 2
	DefaultRecyclerNumber    int32 = 1
)

// build foundationdbCluster's label for classify pods.
func (dms *DorisDisaggregatedMetaService) GenerateFDBLabels() map[string]string {
	if dms.Labels == nil {
		return map[string]string{
			NameLabelKey: dms.Name,
		}
	}

	labels := make(map[string]string)
	labels[NameLabelKey] = dms.Name
	for k, v := range dms.Labels {
		labels[k] = v
	}

	return labels
}

func (dms *DorisDisaggregatedMetaService) GenerateFDBClusterName() string {
	return dms.Name + FDBNameSuffix
}

func GenerateServiceLabels(dms *DorisDisaggregatedMetaService, componentType ComponentType) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = dms.Name
	labels[ComponentLabelKey] = string(componentType)
	labels[ServiceRoleForCluster] = string(Service_Role_Access)
	return labels
}

func GenerateServiceSelector(dms *DorisDisaggregatedMetaService, componentType ComponentType) metadata.Labels {
	return GenerateStatefulSetSelector(dms, componentType)
}

func GenerateStatefulSetSelector(dms *DorisDisaggregatedMetaService, componentType ComponentType) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = statefulSetName(dms, componentType)
	labels[DisaggregatedDorisMetaserviceLabelKey] = dms.Name
	labels[ComponentLabelKey] = string(componentType)
	return labels
}

func statefulSetName(dms *DorisDisaggregatedMetaService, componentType ComponentType) string {
	return dms.Name + "-" + string(componentType)
}

func GenerateStatefulSetLabels(dms *DorisDisaggregatedMetaService, componentType ComponentType) metadata.Labels {
	labels := metadata.Labels{}
	labels[OwnerReference] = dms.Name
	labels[ComponentLabelKey] = string(componentType)
	return labels
}

func GenerateCommunicateServiceName(dms *DorisDisaggregatedMetaService, componentType ComponentType) string {
	return dms.Name + "-" + string(componentType) + "-service"
}

func GenerateComponentStatefulSetName(dms *DorisDisaggregatedMetaService, componentType ComponentType) string {
	return statefulSetName(dms, componentType)
}

func GetPodLabels(dms *DorisDisaggregatedMetaService, componentType ComponentType) metadata.Labels {
	labels := GenerateStatefulSetSelector(dms, componentType)
	labels.AddLabel(getDefaultLabels(dms))
	labels.AddLabel(dms.Spec.MS.Labels)
	return labels
}

func getDefaultLabels(dms *DorisDisaggregatedMetaService) metadata.Labels {
	labels := metadata.Labels{}
	labels[DisaggregatedDorisMetaserviceLabelKey] = dms.Name
	return labels
}

func GetFDBEndPoint(dms *DorisDisaggregatedMetaService) string {
	return dms.Status.FDBStatus.FDBAddress
}

func IsReconcilingStatusPhase(c MetaServicePhase) bool {
	return c == Upgrading || c == Failed
}

func (ddm *DorisDisaggregatedMetaService) GetMSServiceName() string {
	return GenerateCommunicateServiceName(ddm, Component_MS)
}
