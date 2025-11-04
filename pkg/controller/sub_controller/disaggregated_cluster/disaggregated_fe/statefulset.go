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

package disaggregated_fe

import (
	"fmt"
	"strconv"

	v1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sub "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MS_ENDPOINT      string = "MS_ENDPOINT"
	STATEFULSET_NAME string = "STATEFULSET_NAME"
	CLUSTER_ID       string = "CLUSTER_ID"
)

const (
	//DefaultStorageSize int64 = 107374182400
	basic_auth_path  = "/etc/basic_auth"
	auth_volume_name = "basic-auth"
)

func (dfc *DisaggregatedFEController) newFEPodsSelector(ddcName string) map[string]string {
	return map[string]string{
		v1.DorisDisaggregatedClusterName:    ddcName,
		v1.DorisDisaggregatedPodType:        "fe",
		v1.DorisDisaggregatedOwnerReference: ddcName,
	}
}

func (dfc *DisaggregatedFEController) newFESchedulerLabels(ddcName string) map[string]string {
	return map[string]string{
		v1.DorisDisaggregatedClusterName: ddcName,
		v1.DorisDisaggregatedPodType:     "fe",
	}
}

func (dfc *DisaggregatedFEController) NewStatefulset(ddc *v1.DorisDisaggregatedCluster, confMap map[string]interface{}) *appv1.StatefulSet {
	spec := ddc.Spec.FeSpec
	_, _, vcts := dfc.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedFE, &spec.CommonSpec)
	pts := dfc.NewPodTemplateSpec(ddc, confMap)
	//add last supplementary spec. if add new config in ddc spec and the config need add in pod, use the follow function to add.
	dfc.DisaggregatedSubDefaultController.AddClusterSpecForPodTemplate(v1.DisaggregatedFE, confMap, &ddc.Spec, &pts)
	st := dfc.NewDefaultStatefulset(ddc)
	//metadata
	func() {
		st.Name = ddc.GetFEStatefulsetName()
		st.Labels = dfc.newFESchedulerLabels(ddc.Name)
	}()

	func() {
		selector := dfc.newFEPodsSelector(ddc.Name)
		st.Spec.Replicas = ddc.Spec.FeSpec.Replicas
		st.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
		st.Spec.VolumeClaimTemplates = vcts
		st.Spec.ServiceName = ddc.GetFEInternalServiceName()
		st.Spec.Template = pts
	}()

	return st
}

func (dfc *DisaggregatedFEController) getFEPodLabels(ddc *v1.DorisDisaggregatedCluster) resource.Labels {
	selector := dfc.newFEPodsSelector(ddc.Name)
	labels := (resource.Labels)(selector)
	labels.AddLabel(ddc.Spec.FeSpec.Labels)
	return labels
}

func (dfc *DisaggregatedFEController) NewPodTemplateSpec(ddc *v1.DorisDisaggregatedCluster, confMap map[string]interface{}) corev1.PodTemplateSpec {
	pts := resource.NewPodTemplateSpecWithCommonSpec(false, &ddc.Spec.FeSpec.CommonSpec, v1.DisaggregatedFE)
	//pod template metadata.
	labels := dfc.getFEPodLabels(ddc)
	pts.Labels = labels
	c := dfc.NewFEContainer(ddc, confMap)
	pts.Spec.Containers = append(pts.Spec.Containers, c)
	vs, _, _ := dfc.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedFE, &ddc.Spec.FeSpec.CommonSpec)
	configVolumes, _ := dfc.BuildDefaultConfigMapVolumesVolumeMounts(ddc.Spec.FeSpec.ConfigMaps)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)
	pts.Spec.Volumes = append(pts.Spec.Volumes, configVolumes...)

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

	if len(ddc.Spec.FeSpec.Secrets) != 0 {
		secretVolumes, _ := resource.GetMultiSecretVolumeAndVolumeMountWithCommonSpec(&ddc.Spec.FeSpec.CommonSpec)
		pts.Spec.Volumes = append(pts.Spec.Volumes, secretVolumes...)
	}

	pts.Spec.Affinity = dfc.ConstructDefaultAffinity(v1.DorisDisaggregatedClusterName, labels[v1.DorisDisaggregatedClusterName], ddc.Spec.FeSpec.Affinity)

	return pts
}

func (dfc *DisaggregatedFEController) NewFEContainer(ddc *v1.DorisDisaggregatedCluster, cvs map[string]interface{}) corev1.Container {
	c := resource.NewContainerWithCommonSpec(&ddc.Spec.FeSpec.CommonSpec)
	c.Lifecycle = resource.LifeCycleWithPreStopScript(c.Lifecycle, sub.GetDisaggregatedPreStopScript(v1.DisaggregatedFE))
	cmd, args := sub.GetDisaggregatedCommand(v1.DisaggregatedFE)
	c.Command = cmd
	c.Args = args
	c.Name = resource.DISAGGREGATED_FE_MAIN_CONTAINER_NAME

	c.Ports = resource.GetDisaggregatedContainerPorts(cvs, v1.DisaggregatedFE)
	c.Env = ddc.Spec.FeSpec.CommonSpec.EnvVars
	c.Env = append(c.Env, resource.GetPodDefaultEnv()...)
	c.Env = append(c.Env, dfc.newSpecificEnvs(ddc)...)

	resource.BuildDisaggregatedProbe(&c, &ddc.Spec.FeSpec.CommonSpec, v1.DisaggregatedFE)
	_, vms, _ := dfc.BuildVolumesVolumeMountsAndPVCs(cvs, v1.DisaggregatedFE, &ddc.Spec.FeSpec.CommonSpec)
	_, cmvms := dfc.BuildDefaultConfigMapVolumesVolumeMounts(ddc.Spec.FeSpec.ConfigMaps)
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

	if len(ddc.Spec.FeSpec.Secrets) != 0 {
		_, secretVolumeMounts := resource.GetMultiSecretVolumeAndVolumeMountWithCommonSpec(&ddc.Spec.FeSpec.CommonSpec)
		c.VolumeMounts = append(c.VolumeMounts, secretVolumeMounts...)
	}

	return c
}

func (dfc *DisaggregatedFEController) newSpecificEnvs(ddc *v1.DorisDisaggregatedCluster) []corev1.EnvVar {
	var feEnvs []corev1.EnvVar
	stsName := ddc.GetFEStatefulsetName()

	// get meta service endpoint and token
	// msPort explain ms`s conf(doris_cloud.conf) instead of fe`s conf(fe.conf)
	msConfMap := dfc.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.MS_RESOLVEKEY, ddc.Spec.MetaService.ConfigMaps)
	msPort := resource.GetPort(msConfMap, resource.BRPC_LISTEN_PORT)
	msEndpoint := ddc.GetMSServiceName() + "." + ddc.Namespace + ":" + strconv.Itoa(int(msPort))
	feEnvs = append(feEnvs,
		corev1.EnvVar{Name: MS_ENDPOINT, Value: msEndpoint},
		corev1.EnvVar{Name: CLUSTER_ID, Value: fmt.Sprintf("%d", ddc.GetInstanceHashId())},
		corev1.EnvVar{Name: STATEFULSET_NAME, Value: stsName},
		corev1.EnvVar{Name: resource.ENV_FE_ADDR, Value: ddc.GetFEVIPAddresss()},
		corev1.EnvVar{Name: resource.ENV_FE_ELECT_NUMBER, Value: strconv.FormatInt(int64(ddc.GetElectionNumber()), 10)},
	)

	// add user and password envs
	if ddc.Spec.AdminUser != nil {
		feEnvs = append(feEnvs,
			corev1.EnvVar{Name: resource.ADMIN_USER, Value: ddc.Spec.AdminUser.Name},
			corev1.EnvVar{Name: resource.ADMIN_PASSWD, Value: ddc.Spec.AdminUser.Password},
		)
	}

	return feEnvs
}
