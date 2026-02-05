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

package computegroups

import (
	"fmt"
	"strconv"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sub "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	basic_auth_path  = "/etc/basic_auth"
	auth_volume_name = "basic-auth"
)

// generate statefulset or service labels
func (dcgs *DisaggregatedComputeGroupsController) newCG2LayerSchedulerLabels(ddcName /*DisaggregatedClusterName*/, uniqueId string) map[string]string {
	labels := dcgs.GetCG2LayerCommonSchedulerLabels(ddcName)
	labels[dv1.DorisDisaggregatedComputeGroupUniqueId] = uniqueId
	return labels
}

func (dcgs *DisaggregatedComputeGroupsController) GetCG2LayerCommonSchedulerLabels(ddcName string) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName:    ddcName,
		dv1.DorisDisaggregatedOwnerReference: ddcName,
	}
}
func (dcgs *DisaggregatedComputeGroupsController) newCGPodsSelector(ddcName /*DisaggregatedClusterName*/, uniqueId string) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName:          ddcName,
		dv1.DorisDisaggregatedComputeGroupUniqueId: uniqueId,
		dv1.DorisDisaggregatedPodType:              "compute",
	}
}

func (dcgs *DisaggregatedComputeGroupsController) NewStatefulset(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup, cvs map[string]interface{}) *appv1.StatefulSet {
	st := dcgs.NewDefaultStatefulset(ddc)
	uniqueId := cg.UniqueId
	matchLabels := dcgs.newCGPodsSelector(ddc.Name, uniqueId)

	//build metadata
	func() {
		st.Name = ddc.GetCGStatefulsetName(cg)
		st.Labels = dcgs.newCG2LayerSchedulerLabels(ddc.Name, uniqueId)
	}()

	// build st.spec
	func() {
		st.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
		_, _, vcts := dcgs.BuildVolumesVolumeMountsAndPVCs(cvs, dv1.DisaggregatedBE, &cg.CommonSpec)
		st.Spec.Replicas = cg.Replicas
		st.Spec.VolumeClaimTemplates = vcts
		st.Spec.ServiceName = ddc.GetCGServiceName(cg)
		pts := dcgs.NewPodTemplateSpec(ddc, cvs, cg)
		st.Spec.Template = pts
	}()

	return st
}

func (dcgs *DisaggregatedComputeGroupsController) getCGPodLabels(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) resource.Labels {
	selector := dcgs.newCGPodsSelector(ddc.Name, cg.UniqueId)
	labels := (resource.Labels)(selector)
	labels.AddLabel(cg.Labels)
	return labels

}

func (dcgs *DisaggregatedComputeGroupsController) NewPodTemplateSpec(ddc *dv1.DorisDisaggregatedCluster, cvs map[string]interface{}, cg *dv1.ComputeGroup) corev1.PodTemplateSpec {
	pts := resource.NewPodTemplateSpecWithCommonSpec(cg.SkipDefaultSystemInit, &cg.CommonSpec, dv1.DisaggregatedBE)
	//pod template metadata.
	labels := dcgs.getCGPodLabels(ddc, cg)
	pts.Labels = labels
	c := dcgs.NewCGContainer(ddc, cvs, cg)
	pts.Spec.Containers = append(pts.Spec.Containers, c)

	vs, _, _ := dcgs.BuildVolumesVolumeMountsAndPVCs(cvs, dv1.DisaggregatedBE, &cg.CommonSpec)
	configVolumes, _ := dcgs.BuildDefaultConfigMapVolumesVolumeMounts(cg.ConfigMaps)
	pts.Spec.Volumes = append(pts.Spec.Volumes, configVolumes...)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)

	if ddc.Spec.AuthSecret != "" {
		pts.Spec.Volumes = append(pts.Spec.Volumes, corev1.Volume{
			Name: auth_volume_name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: ddc.Spec.AuthSecret,
				},
			},
		})
	}

	if len(cg.Secrets) != 0 {
		secretVolumes, _ := resource.GetMultiSecretVolumeAndVolumeMountWithCommonSpec(&cg.CommonSpec)
		pts.Spec.Volumes = append(pts.Spec.Volumes, secretVolumes...)
	}

	//add last supplementary spec. if add new config in ddc spec and the config need add in pod, use the follow function to add.
	dcgs.DisaggregatedSubDefaultController.AddClusterSpecForPodTemplate(dv1.DisaggregatedBE, cvs, &ddc.Spec, &pts)
	cgUniqueId := labels[dv1.DorisDisaggregatedComputeGroupUniqueId]
	pts.Spec.Affinity = dcgs.ConstructDefaultAffinity(dv1.DorisDisaggregatedComputeGroupUniqueId, cgUniqueId, pts.Spec.Affinity)

	return pts
}

func (dcgs *DisaggregatedComputeGroupsController) NewCGContainer(ddc *dv1.DorisDisaggregatedCluster, cvs map[string]interface{}, cg *dv1.ComputeGroup) corev1.Container {

	if cg.EnableWorkloadGroup {
		if cg.ContainerSecurityContext == nil {
			cg.ContainerSecurityContext = &corev1.SecurityContext{}
		}
		cg.ContainerSecurityContext.Privileged = pointer.Bool(true)
	}

	c := resource.NewContainerWithCommonSpec(&cg.CommonSpec)
	c.Lifecycle = resource.LifeCycleWithPreStopScript(c.Lifecycle, sub.GetDisaggregatedPreStopScript(dv1.DisaggregatedBE))
	cmd, args := sub.GetDisaggregatedCommand(dv1.DisaggregatedBE)
	c.Command = cmd
	c.Args = args
	c.Name = resource.DISAGGREGATED_BE_MAIN_CONTAINER_NAME

	c.Ports = resource.GetDisaggregatedContainerPorts(cvs, dv1.DisaggregatedBE)
	c.Env = cg.CommonSpec.EnvVars
	c.Env = append(c.Env, resource.GetPodDefaultEnv()...)
	c.Env = append(c.Env, dcgs.newSpecificEnvs(ddc, cg)...)

	if cg.SkipDefaultSystemInit {
		// Only works when the doris version is higher than 2.1.8 or 3.0.4
		// When the environment variable SKIP_CHECK_ULIMIT=true is passed in, the start_be.sh will not check system parameters like ulimit and vm.max_map_count etc.
		c.Env = append(c.Env, corev1.EnvVar{Name: "SKIP_CHECK_ULIMIT", Value: "true"})
	}

	if cg.AutoResolveLimitCPU && cg.Limits.Cpu() != nil {
		c.Env = append(c.Env, corev1.EnvVar{Name: "BE_CPU_LIMIT", Value: cg.Limits.Cpu().String()})
	}

	resource.BuildDisaggregatedProbe(&c, &cg.CommonSpec, dv1.DisaggregatedBE)
	_, vms, _ := dcgs.BuildVolumesVolumeMountsAndPVCs(cvs, dv1.DisaggregatedBE, &cg.CommonSpec)
	_, cmvms := dcgs.BuildDefaultConfigMapVolumesVolumeMounts(cg.ConfigMaps)
	c.VolumeMounts = vms
	if c.VolumeMounts == nil {
		c.VolumeMounts = cmvms
	} else {
		c.VolumeMounts = append(c.VolumeMounts, cmvms...)
	}

	// add basic auth secret volumeMount
	if ddc.Spec.AuthSecret != "" {
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      auth_volume_name,
			MountPath: basic_auth_path,
		})
	}

	if len(cg.Secrets) != 0 {
		_, secretVolumeMounts := resource.GetMultiSecretVolumeAndVolumeMountWithCommonSpec(&cg.CommonSpec)
		c.VolumeMounts = append(c.VolumeMounts, secretVolumeMounts...)
	}

	return c
}

// add specific envs for be, the env will used by be_disaggregated_entrypoint script.
func (dcgs *DisaggregatedComputeGroupsController) newSpecificEnvs(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) []corev1.EnvVar {
	var cgEnvs []corev1.EnvVar
	stsName := ddc.GetCGStatefulsetName(cg)

	//get fe config for find query port
	confMap := dcgs.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.FE_RESOLVEKEY, ddc.Spec.FeSpec.ConfigMaps)
	fqp := resource.GetPort(confMap, resource.QUERY_PORT)
	fqpStr := strconv.FormatInt(int64(fqp), 10)
	//use fe service name as access address.
	feAddr := ddc.GetFEVIPAddresss()
	cgEnvs = append(cgEnvs,
		corev1.EnvVar{Name: resource.STATEFULSET_NAME, Value: stsName},
		corev1.EnvVar{Name: resource.COMPUTE_GROUP_NAME, Value: ddc.GetCGName(cg)},
		corev1.EnvVar{Name: resource.ENV_FE_ADDR, Value: feAddr},
		corev1.EnvVar{Name: resource.ENV_FE_PORT, Value: fqpStr})

	// add user and password envs
	if ddc.Spec.AdminUser != nil {
		cgEnvs = append(cgEnvs,
			corev1.EnvVar{Name: resource.ADMIN_USER, Value: ddc.Spec.AdminUser.Name},
			corev1.EnvVar{Name: resource.ADMIN_PASSWD, Value: ddc.Spec.AdminUser.Password},
		)
	}

	if cg.EnableWorkloadGroup {
		cgEnvs = append(cgEnvs,
			corev1.EnvVar{Name: resource.ENABLE_WORKLOAD_GROUP, Value: fmt.Sprintf("%t", cg.EnableWorkloadGroup)},
		)
	}

	return cgEnvs
}

func (dcgs *DisaggregatedComputeGroupsController) useNewDefaultValuesInStatefulset(st *appv1.StatefulSet) {
	resource.UseNewDefaultInitContainerImage(&st.Spec.Template)
}
