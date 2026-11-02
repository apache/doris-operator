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

package metaservice

import (
	"context"
	"fmt"

	v1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/metadata"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	defaultLogPrefixName = "log"
	//DefaultStorageSize   int64 = 107374182400
)

// FDBClusterFileKey is the key used by fdb-kubernetes-operator for the cluster file.
const FDBClusterFileKey = "cluster-file"

func (dms *DisaggregatedMSController) newMSPodsSelector(ddcName string) map[string]string {
	return map[string]string{
		v1.DorisDisaggregatedClusterName:    ddcName,
		v1.DorisDisaggregatedPodType:        "ms",
		v1.DorisDisaggregatedOwnerReference: ddcName,
	}
}

func (dms *DisaggregatedMSController) newMSSchedulerLabels(ddcName string) map[string]string {
	return map[string]string{
		v1.DorisDisaggregatedClusterName: ddcName,
		v1.DorisDisaggregatedPodType:     "ms",
	}
}

func (dms *DisaggregatedMSController) newStatefulset(ddc *v1.DorisDisaggregatedCluster, confMap map[string]interface{}, fdbEndpoint string) *appv1.StatefulSet {
	st := dms.NewDefaultStatefulset(ddc)
	func() {
		st.Name = ddc.GetMSStatefulsetName()
		st.Labels = dms.newMSSchedulerLabels(ddc.Name)
	}()

	msSpec := ddc.Spec.MetaService
	matchLabels := dms.newMSPodsSelector(ddc.Name)
	_, _, vcts := dms.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedMS, &msSpec.CommonSpec)
	replicas := metadata.GetInt32Pointer(v1.DefaultMetaserviceNumber)
	if msSpec.Replicas != nil {
		replicas = msSpec.Replicas
	}

	func() {
		st.Spec.Replicas = replicas
		st.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
		st.Spec.Template = dms.NewPodTemplateSpec(ddc, matchLabels, confMap, fdbEndpoint)
		st.Spec.ServiceName = ddc.GetMSServiceName()
		st.Spec.VolumeClaimTemplates = vcts
	}()

	return st
}

func (dms *DisaggregatedMSController) NewPodTemplateSpec(ddc *v1.DorisDisaggregatedCluster, selector map[string]string, confMap map[string]interface{}, fdbEndpoint string) corev1.PodTemplateSpec {
	pts := resource.NewPodTemplateSpecWithCommonSpec(false, &ddc.Spec.MetaService.CommonSpec, v1.DisaggregatedMS)
	//pod template metadata.
	func() {
		l := (resource.Labels)(selector)
		l.AddLabel(pts.Labels)
		pts.Labels = l
	}()

	c := dms.NewMSContainer(ddc, confMap, fdbEndpoint)
	pts.Spec.Containers = append(pts.Spec.Containers, c)
	vs, _, _ := dms.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedMS, &ddc.Spec.MetaService.CommonSpec)
	configVolumes, _ := dms.BuildDefaultConfigMapVolumesVolumeMounts(ddc.Spec.MetaService.ConfigMaps)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)
	pts.Spec.Volumes = append(pts.Spec.Volumes, configVolumes...)
	pts.Spec.Affinity = dms.ConstructDefaultAffinity(v1.DorisDisaggregatedClusterName, selector[v1.DorisDisaggregatedClusterName], ddc.Spec.MetaService.Affinity)

	if len(ddc.Spec.MetaService.Secrets) != 0 {
		secretVolumes, _ := resource.GetMultiSecretVolumeAndVolumeMountWithCommonSpec(&ddc.Spec.MetaService.CommonSpec)
		pts.Spec.Volumes = append(pts.Spec.Volumes, secretVolumes...)
	}

	return pts
}

func (dms *DisaggregatedMSController) NewMSContainer(ddc *v1.DorisDisaggregatedCluster, cvs map[string]interface{}, fdbEndpoint string) corev1.Container {
	c := resource.NewContainerWithCommonSpec(&ddc.Spec.MetaService.CommonSpec)

	c.Lifecycle = resource.LifeCycleWithPreStopScript(c.Lifecycle, sc.GetDisaggregatedPreStopScript(v1.DisaggregatedMS))
	cmd, args := sc.GetDisaggregatedCommand(v1.DisaggregatedMS)
	c.Command = cmd
	c.Args = args
	//c.Name = "metaservice"
	c.Name = resource.DISAGGREGATED_MS_MAIN_CONTAINER_NAME

	c.Ports = resource.GetDisaggregatedContainerPorts(cvs, v1.DisaggregatedMS)
	c.Env = ddc.Spec.MetaService.CommonSpec.EnvVars
	c.Env = append(c.Env, resource.GetPodDefaultEnv()...)
	c.Env = append(c.Env, dms.newSpecificEnvs(fdbEndpoint)...)
	resource.BuildDisaggregatedProbe(&c, &ddc.Spec.MetaService.CommonSpec, v1.DisaggregatedMS)
	_, vms, _ := dms.BuildVolumesVolumeMountsAndPVCs(cvs, v1.DisaggregatedMS, &ddc.Spec.MetaService.CommonSpec)
	_, cmvms := dms.BuildDefaultConfigMapVolumesVolumeMounts(ddc.Spec.MetaService.ConfigMaps)
	if len(vms) != 0 {
		c.VolumeMounts = append(c.VolumeMounts, vms...)
	}
	if len(cmvms) != 0 {
		c.VolumeMounts = append(c.VolumeMounts, cmvms...)
	}

	if len(ddc.Spec.MetaService.Secrets) != 0 {
		_, secretVolumeMounts := resource.GetMultiSecretVolumeAndVolumeMountWithCommonSpec(&ddc.Spec.MetaService.CommonSpec)
		c.VolumeMounts = append(c.VolumeMounts, secretVolumeMounts...)
	}

	return c
}

func (dms *DisaggregatedMSController) resolveFDBEndpoint(ctx context.Context, ddc *v1.DorisDisaggregatedCluster) (string, error) {
	msSpec := ddc.Spec.MetaService
	if msSpec.FDB.Address != "" {
		return msSpec.FDB.Address, nil
	}

	ref := msSpec.FDB.ConfigMapNamespaceName
	if ref.Namespace == "" || ref.Name == "" {
		return "", fmt.Errorf("FDB address or ConfigMap reference is not configured")
	}

	// Referenced FDB ConfigMaps may be outside this Operator's cached
	// namespaces. Read the API directly without expanding workload ownership.
	// The API server still enforces this Operator's RBAC permissions.
	reader := dms.APIReader
	if reader == nil {
		reader = dms.K8sclient
	}
	var cm corev1.ConfigMap
	if err := reader.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, &cm); err != nil {
		return "", fmt.Errorf("get FDB ConfigMap %s/%s: %w", ref.Namespace, ref.Name, err)
	}
	fdbEndpoint, ok := cm.Data[FDBClusterFileKey]
	if !ok || fdbEndpoint == "" {
		return "", fmt.Errorf("FDB ConfigMap %s/%s does not contain a non-empty %q", ref.Namespace, ref.Name, FDBClusterFileKey)
	}
	return fdbEndpoint, nil
}

func (dms *DisaggregatedMSController) newSpecificEnvs(fdbEndpoint string) []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name:  resource.FDB_ENDPOINT,
		Value: fdbEndpoint,
	}}
}
