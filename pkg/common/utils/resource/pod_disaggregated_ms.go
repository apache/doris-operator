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

package resource

import (
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"path/filepath"
)

const (
	START_MS_COMMAND        = "/opt/apache-doris/ms_disaggregated_entrypoint.sh"
	START_RC_COMMAND        = "/opt/apache-doris/ms_disaggregated_entrypoint.sh"
	START_MS_PARAMETER      = "meta-service"
	START_RC_PARAMETER      = "recycler"
	HEALTH_MS_PROBE_COMMAND = "/opt/apache-doris/ms_disaggregated_probe.sh"
	PRESTOP_MS_COMMAND      = "/opt/apache-doris/ms_disaggregated_prestop.sh"
	MS_Log_Key              = "log_dir"
	Default_MS_Log_Path     = "/opt/apache-doris/ms/log/"
)

func NewDMSPodTemplateSpec(dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) corev1.PodTemplateSpec {
	spec, _ := GetDMSBaseSpecFromCluster(dms, componentType)
	volumes := newVolumesFromDMSBaseSpec(*spec)
	dmsAffinity := spec.Affinity
	SecurityContext := spec.SecurityContext

	//map pod labels and annotations into pod
	volumes, _ = appendPodInfoVolumesVolumeMounts(volumes, nil)

	if len(spec.ConfigMaps) != 0 {
		configVolumes, _ := getConfigmapVolumeAndVolumeMount(spec.ConfigMaps)
		volumes = append(volumes, configVolumes...)
	}

	pts := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        generateDMSPodTemplateName(dms, componentType),
			Annotations: spec.Annotations,
			Labels:      mv1.GetPodLabels(dms, componentType),
		},

		Spec: corev1.PodSpec{
			ImagePullSecrets:   spec.ImagePullSecrets,
			NodeSelector:       spec.NodeSelector,
			Volumes:            volumes,
			ServiceAccountName: spec.ServiceAccount,
			Affinity:           spec.Affinity,
			Tolerations:        spec.Tolerations,
			HostAliases:        spec.HostAliases,
			SecurityContext:    SecurityContext,
		},
	}

	pts.Spec.Affinity = constructDMSAffinity(dmsAffinity, componentType)
	return pts
}

// newVolumesFromBaseSpec return corev1.Volume build from baseSpec.
func newVolumesFromDMSBaseSpec(spec mv1.BaseSpec) []corev1.Volume {
	var volumes []corev1.Volume
	if spec.PersistentVolume == nil {
		return volumes
	}

	//construct log volume
	v := corev1.Volume{}
	v.Name = defaultLogPrefixName
	v.VolumeSource = corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: defaultLogPrefixName,
		},
	}

	volumes = append(volumes, v)

	return volumes
}

// buildVolumeMounts construct all volumeMounts contains default volumeMounts if persistentVolumes not definition.
func buildDMSVolumeMounts(spec mv1.BaseSpec, config map[string]interface{}) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	_, volumeMounts = appendPodInfoVolumesVolumeMounts(nil, volumeMounts)

	if spec.PersistentVolume == nil {
		return volumeMounts
	}

	logPath := Default_MS_Log_Path
	if p, ok := config[MS_Log_Key]; ok {
		cp := p.(string)
		//exclude the rel path interfere
		if filepath.IsAbs(cp) {
			logPath = cp
		} else {
			logPath = filepath.Join(Default_MS_Log_Path, cp)
		}
	}
	vm := corev1.VolumeMount{}
	vm.MountPath = logPath
	vm.Name = defaultLogPrefixName
	volumeMounts = append(volumeMounts, vm)
	return volumeMounts
}

func NewDMSBaseMainContainer(dms *mv1.DorisDisaggregatedMetaService, brpcPort int32, config map[string]interface{}, componentType mv1.ComponentType) corev1.Container {
	var envs []corev1.EnvVar
	spec, _ := GetDMSBaseSpecFromCluster(dms, componentType)

	command, args := buildDMSEntrypointCommand(componentType)

	fdbEndPoint := mv1.GetFDBEndPoint(dms)
	envs = append(envs, buildDMSBaseEnvs()...)
	envs = append(envs,
		corev1.EnvVar{
			Name:  FDB_ENDPOINT,
			Value: fdbEndPoint,
		}, corev1.EnvVar{
			Name: NAMESPACE,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.namespace",
				},
			},
		},
	)
	envs = mergeEnvs(envs, spec.EnvVars)

	volumeMounts := buildDMSVolumeMounts(*spec, config)
	if len(spec.ConfigMaps) != 0 {
		_, configVolumeMounts := getConfigmapVolumeAndVolumeMount(spec.ConfigMaps)
		volumeMounts = append(volumeMounts, configVolumeMounts...)
	}

	return corev1.Container{
		Image:          spec.Image,
		Command:        command,
		Args:           args,
		Ports:          []corev1.ContainerPort{},
		Env:            envs,
		VolumeMounts:   volumeMounts,
		Resources:      spec.ResourceRequirements,
		LivenessProbe:  dmsLivenessProbe(brpcPort),
		StartupProbe:   dmsStartupProbe(brpcPort),
		ReadinessProbe: dmsReadinessProbe(brpcPort),
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{PRESTOP_MS_COMMAND},
				},
			},
		},
	}
}

func buildDMSBaseEnvs() []corev1.EnvVar {
	defaultEnvs := []corev1.EnvVar{
		{
			Name: POD_NAME,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: POD_IP,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
			},
		},
		{
			Name: HOST_IP,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			},
		},
		{
			Name: POD_NAMESPACE,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			},
		},
		{
			Name:  config_env_name,
			Value: config_env_path,
		},
		{
			Name:  DORIS_ROOT,
			Value: DEFAULT_ROOT_PATH,
		},
	}

	return defaultEnvs
}

func buildDMSEntrypointCommand(componentType mv1.ComponentType) (commands []string, args []string) {
	switch componentType {
	case mv1.Component_MS:
		return []string{START_MS_COMMAND}, []string{START_MS_PARAMETER}
	case mv1.Component_RC:
		return []string{START_RC_COMMAND}, []string{START_RC_PARAMETER}
	default:
		klog.Infof("buildDMSEntrypointCommand the componentType %s is not supported.", componentType)
		return []string{}, []string{}
	}
}

func generateDMSPodTemplateName(dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) string {
	return dms.Name + "-" + string(componentType)
}

func GetDMSBaseSpecFromCluster(dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) (*mv1.BaseSpec, *int32) {
	var bSpec *mv1.BaseSpec
	var replicas *int32
	switch componentType {
	case mv1.Component_MS:
		bSpec = &dms.Spec.MS.BaseSpec
		replicas = metadata.GetInt32Pointer(mv1.DefaultMetaserviceNumber)
	case mv1.Component_RC:
		bSpec = &dms.Spec.Recycler.BaseSpec
		replicas = metadata.GetInt32Pointer(mv1.DefaultRecyclerNumber)
	default:
		klog.Infof("the componentType %s is not supported!", componentType)
	}

	return bSpec, replicas
}

// getConfigmapVolumeAndVolumeMount get Volume And VolumeMount base on configmaps
func getConfigmapVolumeAndVolumeMount(cms []mv1.ConfigMap) ([]corev1.Volume, []corev1.VolumeMount) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	for _, cm := range cms {
		path := cm.MountPath
		if cm.MountPath == "" {
			path = config_env_path
		}
		volumes = append(
			volumes,
			corev1.Volume{
				Name: cm.Name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cm.Name,
						},
					},
				},
			},
		)

		volumeMounts = append(
			volumeMounts,
			corev1.VolumeMount{
				Name:      cm.Name,
				MountPath: path,
			},
		)
	}

	return volumes, volumeMounts
}

// StartupProbe returns a startup probe.
func dmsStartupProbe(port int32) *corev1.Probe {
	commands := []string{HEALTH_MS_PROBE_COMMAND, "alive"}
	return startupProbe(port, 180, "", commands, Exec)
}

// dmsLivenessProbe returns a liveness.
func dmsLivenessProbe(port int32) *corev1.Probe {
	commands := []string{HEALTH_MS_PROBE_COMMAND, "alive"}
	return livenessProbe(port, "", commands, Exec)
}

// ReadinessProbe returns a readiness probe.
func dmsReadinessProbe(port int32) *corev1.Probe {
	commands := []string{HEALTH_MS_PROBE_COMMAND, "ready"}
	return readinessProbe(port, "", commands, Exec)
}

// getDMSDefaultAffinity build MS affinity rules based on default policy configuration
// MS default Affinity rule is :
// Pods of the same component should deploy on different hosts with Preferred scheduling.
// weight is 20, weight range is 1-100
func getDMSDefaultAffinity(componentType mv1.ComponentType) *corev1.Affinity {
	podAffinityTerm := corev1.WeightedPodAffinityTerm{
		Weight: 20,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: mv1.ComponentLabelKey, Operator: metav1.LabelSelectorOpIn, Values: []string{string(componentType)}},
				},
			},
			TopologyKey: NODE_TOPOLOGYKEY,
		},
	}
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{podAffinityTerm},
		},
	}
}

// constructDMSAffinity build MS affinity rules based on default policies and custom configurations
func constructDMSAffinity(dmsAffinity *corev1.Affinity, componentType mv1.ComponentType) *corev1.Affinity {
	affinity := getDMSDefaultAffinity(componentType)

	if dmsAffinity == nil {
		return affinity
	}

	dmsPodAntiAffinity := dmsAffinity.PodAntiAffinity
	if dmsPodAntiAffinity != nil {
		affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = dmsPodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, dmsPodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
	}

	affinity.NodeAffinity = dmsAffinity.NodeAffinity
	affinity.PodAffinity = dmsAffinity.PodAffinity

	return affinity
}
