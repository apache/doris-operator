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
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	v1 "github.com/apache/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
)

const (
	config_env_path    = "/etc/doris"
	ConfigEnvPath      = config_env_path
	secret_config_path = config_env_path
	config_env_name    = "CONFIGMAP_MOUNT_PATH"
	basic_auth_path    = "/etc/basic_auth"
	auth_volume_name   = "basic-auth"
	be_storage_name    = "be-storage"
	be_storage_path    = "/opt/apache-doris/be/storage"
	fe_meta_path       = "/opt/apache-doris/fe/doris-meta"
	fe_meta_name       = "fe-meta"

	HEALTH_API_PATH            = "/api/health"
	HEALTH_BROKER_LIVE_COMMAND = "/opt/apache-doris/broker_is_alive.sh"
	FE_PRESTOP                 = "/opt/apache-doris/fe_prestop.sh"
	BE_PRESTOP                 = "/opt/apache-doris/be_prestop.sh"
	BROKER_PRESTOP             = "/opt/apache-doris/broker_prestop.sh"

	//keys for pod env variables
	POD_NAME             = "POD_NAME"
	POD_IP               = "POD_IP"
	HOST_IP              = "HOST_IP"
	POD_NAMESPACE        = "POD_NAMESPACE"
	ADMIN_USER           = "USER"
	ADMIN_PASSWD         = "PASSWD"
	DORIS_ROOT           = "DORIS_ROOT"
	DEFAULT_ADMIN_USER   = "root"
	DEFAULT_ROOT_PATH    = "/opt/apache-doris"
	POD_INFO_PATH        = "/etc/podinfo"
	POD_INFO_VOLUME_NAME = "podinfo"

	NODE_TOPOLOGYKEY = "kubernetes.io/hostname"

	DEFAULT_INIT_IMAGE = "selectdb/alpine:latest"

	HEALTH_DISAGGREGATED_FE_PROBE_COMMAND = "/opt/apache-doris/fe_disaggregated_probe.sh"
	HEALTH_DISAGGREGATED_BE_PROBE_COMMAND = "/opt/apache-doris/be_disaggregated_probe.sh"
	HEALTH_DISAGGREGATED_MS_PROBE_COMMAND = "/opt/apache-doris/ms_disaggregated_probe.sh"

	DISAGGREGATED_LIVE_PARAM_ALIVE = "alive"
	DISAGGREGATED_LIVE_PARAM_READY = "ready"
)

type ProbeType string

var (
	HttpGet   ProbeType = "httpGet"
	TcpSocket ProbeType = "tcpSocket"
	Exec      ProbeType = "exec"
)

func NewPodTemplateSpec(dcr *v1.DorisCluster, componentType v1.ComponentType) corev1.PodTemplateSpec {
	spec := getBaseSpecFromCluster(dcr, componentType)
	var volumes []corev1.Volume
	var si *v1.SystemInitialization
	var dcrAffinity *corev1.Affinity
	var defaultInitContainers []corev1.Container
	var SecurityContext *corev1.PodSecurityContext
	switch componentType {
	case v1.Component_FE:
		volumes = newVolumesFromBaseSpec(dcr.Spec.FeSpec.BaseSpec)
		si = dcr.Spec.FeSpec.BaseSpec.SystemInitialization
		dcrAffinity = dcr.Spec.FeSpec.BaseSpec.Affinity
		SecurityContext = dcr.Spec.FeSpec.BaseSpec.SecurityContext
	case v1.Component_BE:
		volumes = newVolumesFromBaseSpec(dcr.Spec.BeSpec.BaseSpec)
		si = dcr.Spec.BeSpec.BaseSpec.SystemInitialization
		dcrAffinity = dcr.Spec.BeSpec.BaseSpec.Affinity
		SecurityContext = dcr.Spec.BeSpec.BaseSpec.SecurityContext
	case v1.Component_CN:
		si = dcr.Spec.CnSpec.BaseSpec.SystemInitialization
		dcrAffinity = dcr.Spec.CnSpec.BaseSpec.Affinity
		SecurityContext = dcr.Spec.CnSpec.BaseSpec.SecurityContext
	case v1.Component_Broker:
		si = dcr.Spec.BrokerSpec.BaseSpec.SystemInitialization
		dcrAffinity = dcr.Spec.BrokerSpec.BaseSpec.Affinity
		SecurityContext = dcr.Spec.BrokerSpec.BaseSpec.SecurityContext
	default:
		klog.Errorf("NewPodTemplateSpec dorisClusterName %s, namespace %s componentType %s not supported.", dcr.Name, dcr.Namespace, componentType)
	}

	if len(volumes) == 0 {
		volumes, _ = getDefaultVolumesVolumeMounts(componentType)
	}
	//map pod labels and annotations into pod
	volumes, _ = appendPodInfoVolumesVolumeMounts(volumes, nil)
	if dcr.Spec.AuthSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: auth_volume_name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: dcr.Spec.AuthSecret,
				},
			},
		})
	}

	if len(GetMountConfigMapInfo(spec.ConfigMapInfo)) != 0 {
		configVolumes, _ := getMultiConfigVolumeAndVolumeMount(&spec.ConfigMapInfo, componentType)
		volumes = append(volumes, configVolumes...)
	}

	if len(spec.Secrets) != 0 {
		secretVolumes, _ := getMultiSecretVolumeAndVolumeMount(spec, componentType)
		volumes = append(volumes, secretVolumes...)
	}

	pts := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        GeneratePodTemplateName(dcr, componentType),
			Annotations: spec.Annotations,
			Labels:      v1.GetPodLabels(dcr, componentType),
		},

		Spec: corev1.PodSpec{
			ImagePullSecrets:   spec.ImagePullSecrets,
			NodeSelector:       spec.NodeSelector,
			Volumes:            volumes,
			ServiceAccountName: spec.ServiceAccount,
			Affinity:           spec.Affinity.DeepCopy(),
			Tolerations:        spec.Tolerations,
			HostAliases:        spec.HostAliases,
			InitContainers:     defaultInitContainers,
			SecurityContext:    SecurityContext,
		},
	}

	constructInitContainers(componentType, &pts.Spec, si)
	pts.Spec.Affinity = constructAffinity(dcrAffinity, componentType)

	return pts
}

// for disaggregated cluster.
func NewPodTemplateSpecWithCommonSpec(cs *dv1.CommonSpec, componentType dv1.DisaggregatedComponentType) corev1.PodTemplateSpec {
	var vs []corev1.Volume
	si := cs.SystemInitialization
	var defaultInitContainers []corev1.Container
	vs, _ = appendPodInfoVolumesVolumeMounts(vs, nil)
	pts := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        strings.ToLower(string(componentType)),
			Annotations: cs.Annotations,
			Labels:      cs.Labels,
		},

		Spec: corev1.PodSpec{
			ImagePullSecrets:   cs.ImagePullSecrets,
			NodeSelector:       cs.NodeSelector,
			ServiceAccountName: cs.ServiceAccount,
			Affinity:           cs.Affinity.DeepCopy(),
			Tolerations:        cs.Tolerations,
			HostAliases:        cs.HostAliases,
			InitContainers:     defaultInitContainers,
			SecurityContext:    cs.SecurityContext,
			Volumes:            vs,
		},
	}
	constructDisaggregatedInitContainers(componentType, &pts.Spec, si)
	return pts
}

// build disaggregated node(fe,be) container.
func NewContainerWithCommonSpec(cs *dv1.CommonSpec) corev1.Container {
	var vms []corev1.VolumeMount
	_, vms = appendPodInfoVolumesVolumeMounts(nil, vms)
	c := corev1.Container{
		Image:           cs.Image,
		SecurityContext: cs.ContainerSecurityContext,
		Resources:       cs.ResourceRequirements,
		VolumeMounts:    vms,
	}
	return c
}

// ApplySecurityContext applies the container security context to all containers in the pod (if not already set).
func ApplySecurityContext(containers []corev1.Container, securityContext *corev1.SecurityContext) []corev1.Container {
	if securityContext == nil {
		return containers
	}

	for i := range containers {
		if containers[i].SecurityContext == nil {
			containers[i].SecurityContext = securityContext
		} else {
			klog.Info("SecurityContext already exists in container" + containers[i].Name + "! Not overwriting it.")
		}
	}

	return containers
}

func constructInitContainers(componentType v1.ComponentType, podSpec *corev1.PodSpec, si *v1.SystemInitialization) {
	defaultImage := ""
	var defaultInitContains []corev1.Container
	if si != nil {
		initContainer := newBaseInitContainer("init", si)
		defaultImage = si.InitImage
		defaultInitContains = append(defaultInitContains, initContainer)
	}

	// the init containers have sequence，should confirm use initial is always in the first priority.
	if componentType == v1.Component_BE || componentType == v1.Component_CN {
		podSpec.InitContainers = append(podSpec.InitContainers, constructBeDefaultInitContainer(defaultImage))
	}
	podSpec.InitContainers = append(podSpec.InitContainers, defaultInitContains...)
}

func constructDisaggregatedInitContainers(componentType dv1.DisaggregatedComponentType, podSpec *corev1.PodSpec, si *dv1.SystemInitialization) {
	initImage := DEFAULT_INIT_IMAGE
	var defaultInitContains []corev1.Container
	if si != nil {
		enablePrivileged := true
		if si.InitImage != "" {
			initImage = si.InitImage
		}
		initContainer := corev1.Container{
			Image:           initImage,
			Name:            "init",
			Command:         si.Command,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args:            si.Args,
			SecurityContext: &corev1.SecurityContext{
				Privileged: &enablePrivileged,
			},
		}
		si.InitImage = initImage
		defaultInitContains = append(defaultInitContains, initContainer)
	}

	// the init containers have sequence，should confirm use initial is always in the first priority.
	if componentType == dv1.DisaggregatedBE {
		podSpec.InitContainers = append(podSpec.InitContainers, constructBeDefaultInitContainer(initImage))
	}
	podSpec.InitContainers = append(podSpec.InitContainers, defaultInitContains...)
}

// newVolumesFromBaseSpec return corev1.Volume build from baseSpec.
func newVolumesFromBaseSpec(spec v1.BaseSpec) []corev1.Volume {
	var volumes []corev1.Volume
	for _, pv := range spec.PersistentVolumes {
		var volume corev1.Volume
		volume.Name = pv.Name
		volume.VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pv.Name,
			},
		}
		volumes = append(volumes, volume)
	}

	return volumes
}

// buildVolumeMounts construct all volumeMounts contains default volumeMounts if persistentVolumes not definition.
func buildVolumeMounts(spec v1.BaseSpec, componentType v1.ComponentType) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	_, volumeMounts = appendPodInfoVolumesVolumeMounts(nil, volumeMounts)
	if len(spec.PersistentVolumes) == 0 {
		_, volumeMount := getDefaultVolumesVolumeMounts(componentType)
		volumeMounts = append(volumeMounts, volumeMount...)
		return volumeMounts
	}

	for _, pvs := range spec.PersistentVolumes {
		var volumeMount corev1.VolumeMount
		volumeMount.MountPath = pvs.MountPath
		volumeMount.Name = pvs.Name
		volumeMounts = append(volumeMounts, volumeMount)
	}

	return volumeMounts
}

// dst array have high priority will cover the src env when the env's name is right.
func mergeEnvs(src []corev1.EnvVar, dst []corev1.EnvVar) []corev1.EnvVar {
	if len(dst) == 0 {
		return src
	}

	if len(src) == 0 {
		return dst
	}

	m := make(map[string]bool, len(dst))
	for _, env := range dst {
		m[env.Name] = true
	}

	for _, env := range src {
		if _, ok := m[env.Name]; ok {
			continue
		}
		dst = append(dst, env)
	}

	return dst
}

func newBaseInitContainer(name string, si *v1.SystemInitialization) corev1.Container {
	enablePrivileged := true
	initImage := si.InitImage
	if initImage == "" {
		initImage = DEFAULT_INIT_IMAGE
	}
	c := corev1.Container{
		Image:           initImage,
		Name:            name,
		Command:         si.Command,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args:            si.Args,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &enablePrivileged,
		},
	}
	return c
}

func NewBaseMainContainer(dcr *v1.DorisCluster, config map[string]interface{}, componentType v1.ComponentType) corev1.Container {
	command, args := getCommand(componentType)
	var spec v1.BaseSpec
	switch componentType {
	case v1.Component_FE:
		spec = dcr.Spec.FeSpec.BaseSpec
	case v1.Component_BE:
		spec = dcr.Spec.BeSpec.BaseSpec
	case v1.Component_CN:
		spec = dcr.Spec.CnSpec.BaseSpec
	case v1.Component_Broker:
		spec = dcr.Spec.BrokerSpec.BaseSpec
	default:
	}

	volumeMounts := buildVolumeMounts(spec, componentType)
	var envs []corev1.EnvVar
	envs = append(envs, buildBaseEnvs(dcr)...)
	envs = mergeEnvs(envs, spec.EnvVars)

	if len(GetMountConfigMapInfo(spec.ConfigMapInfo)) != 0 {
		_, configVolumeMounts := getMultiConfigVolumeAndVolumeMount(&spec.ConfigMapInfo, componentType)
		volumeMounts = append(volumeMounts, configVolumeMounts...)
	}

	if len(spec.Secrets) != 0 {
		_, secretVolumeMounts := getMultiSecretVolumeAndVolumeMount(&spec, componentType)
		volumeMounts = append(volumeMounts, secretVolumeMounts...)
	}

	// add basic auth secret volumeMount
	if dcr.Spec.AuthSecret != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      auth_volume_name,
			MountPath: basic_auth_path,
		})
	}

	c := corev1.Container{
		Image:           spec.Image,
		Name:            string(componentType),
		Command:         command,
		Args:            args,
		Ports:           []corev1.ContainerPort{},
		Env:             envs,
		VolumeMounts:    volumeMounts,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Resources:       spec.ResourceRequirements,
	}

	//livenessPort use heartbeat port for probe service alive.
	var livenessPort int32
	//readnessPort use http port for confirm the service can provider service to client.
	var readnessPort int32
	var prestopScript string
	var health_api_path string
	var liveProbeType ProbeType
	var readinessProbeType ProbeType
	var commands []string
	switch componentType {
	case v1.Component_FE:
		readnessPort = GetPort(config, HTTP_PORT)
		livenessPort = GetPort(config, QUERY_PORT)
		liveProbeType = TcpSocket
		readinessProbeType = HttpGet
		prestopScript = FE_PRESTOP
		health_api_path = HEALTH_API_PATH
	case v1.Component_BE, v1.Component_CN:
		readnessPort = GetPort(config, WEBSERVER_PORT)
		livenessPort = GetPort(config, HEARTBEAT_SERVICE_PORT)
		liveProbeType = TcpSocket
		readinessProbeType = HttpGet
		prestopScript = BE_PRESTOP
		health_api_path = HEALTH_API_PATH
	case v1.Component_Broker:
		livenessPort = GetPort(config, BROKER_IPC_PORT)
		readnessPort = GetPort(config, BROKER_IPC_PORT)
		liveProbeType = Exec
		readinessProbeType = Exec
		prestopScript = BROKER_PRESTOP
		commands = append(commands, HEALTH_BROKER_LIVE_COMMAND, strconv.Itoa(int(livenessPort)))
	default:
		klog.Infof("the componentType %s is not supported in probe.", componentType)
	}

	// if tcpSocket the health_api_path will ignore.
	c.LivenessProbe = livenessProbe(livenessPort, spec.LiveTimeout, health_api_path, commands, liveProbeType)
	// use liveness as startup, when in debugging mode will not be killed
	c.StartupProbe = startupProbe(livenessPort, spec.StartTimeout, health_api_path, commands, liveProbeType)
	c.ReadinessProbe = readinessProbe(readnessPort, health_api_path, commands, readinessProbeType)
	c.Lifecycle = lifeCycle(prestopScript)

	return c
}

func buildBaseEnvs(dcr *v1.DorisCluster) []corev1.EnvVar {
	defaultEnvs := buildEnvFromPod()

	if dcr.Spec.AdminUser != nil {
		defaultEnvs = append(defaultEnvs, corev1.EnvVar{
			Name:  ADMIN_USER,
			Value: dcr.Spec.AdminUser.Name,
		})
		if dcr.Spec.AdminUser.Password != "" {
			defaultEnvs = append(defaultEnvs, corev1.EnvVar{
				Name:  ADMIN_PASSWD,
				Value: dcr.Spec.AdminUser.Password,
			})
		}
	} else {
		defaultEnvs = append(defaultEnvs, []corev1.EnvVar{{
			Name:  ADMIN_USER,
			Value: DEFAULT_ADMIN_USER,
		}, {
			Name:  DORIS_ROOT,
			Value: DEFAULT_ROOT_PATH,
		}}...)
	}

	return defaultEnvs
}

func buildEnvFromPod() []corev1.EnvVar {
	return []corev1.EnvVar{
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
	}
}

func GetPodDefaultEnv() []corev1.EnvVar {
	return buildEnvFromPod()
}

func getCommand(componentType v1.ComponentType) (commands []string, args []string) {
	switch componentType {
	case v1.Component_FE:
		return []string{"/opt/apache-doris/fe_entrypoint.sh"}, []string{"$(ENV_FE_ADDR)"}
	case v1.Component_BE, v1.Component_CN:
		return []string{"/opt/apache-doris/be_entrypoint.sh"}, []string{"$(ENV_FE_ADDR)"}
	case v1.Component_Broker:
		return []string{"/opt/apache-doris/broker_entrypoint.sh"}, []string{"$(ENV_FE_ADDR)"}
	default:
		klog.Infof("getCommand the componentType %s is not supported.", componentType)
		return []string{}, []string{}
	}
}

func GeneratePodTemplateName(dcr *v1.DorisCluster, componentType v1.ComponentType) string {
	switch componentType {
	case v1.Component_FE:
		return dcr.Name + "-" + string(v1.Component_FE)
	case v1.Component_BE:
		return dcr.Name + "-" + string(v1.Component_BE)
	case v1.Component_CN:
		return dcr.Name + "-" + string(v1.Component_CN)
	case v1.Component_Broker:
		return dcr.Name + "-" + string(v1.Component_Broker)
	default:
		return ""
	}
}

func getBaseSpecFromCluster(dcr *v1.DorisCluster, componentType v1.ComponentType) *v1.BaseSpec {
	var bSpec *v1.BaseSpec
	switch componentType {
	case v1.Component_FE:
		bSpec = &dcr.Spec.FeSpec.BaseSpec
	case v1.Component_BE:
		bSpec = &dcr.Spec.BeSpec.BaseSpec
	case v1.Component_CN:
		bSpec = &dcr.Spec.CnSpec.BaseSpec
	case v1.Component_Broker:
		bSpec = &dcr.Spec.BrokerSpec.BaseSpec
	default:
		klog.Infof("the componentType %s is not supported!", componentType)
	}

	return bSpec
}

func getDefaultVolumesVolumeMounts(componentType v1.ComponentType) ([]corev1.Volume, []corev1.VolumeMount) {
	switch componentType {
	case v1.Component_FE:
		return getFeDefaultVolumesVolumeMounts()
	case v1.Component_BE, v1.Component_CN:
		return getBeDefaultVolumesVolumeMounts()
	default:
		klog.Infof("GetDefaultVolumesVolumeMountsAndPersistentVolumeClaims componentType %s not supported.", componentType)
		return nil, nil
	}
}

func getFeDefaultVolumesVolumeMounts() ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: fe_meta_name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	volumMounts := []corev1.VolumeMount{
		{
			Name:      fe_meta_name,
			MountPath: fe_meta_path,
		},
	}

	return volumes, volumMounts
}

func appendPodInfoVolumesVolumeMounts(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount) {
	if volumes == nil {
		var _ []corev1.Volume
	}
	if volumeMounts == nil {
		var _ []corev1.VolumeMount
	}

	volumes = append(volumes, corev1.Volume{
		Name: POD_INFO_VOLUME_NAME,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{{
					Path: "labels",
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.labels",
					},
				}, {
					Path: "annotations",
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.annotations",
					},
				}},
			},
		},
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      POD_INFO_VOLUME_NAME,
		MountPath: POD_INFO_PATH,
	})

	return volumes, volumeMounts
}

func getBeDefaultVolumesVolumeMounts() ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: be_storage_name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      be_storage_name,
			MountPath: be_storage_path,
		},
	}

	return volumes, volumeMounts
}

func getMultiConfigVolumeAndVolumeMount(cmInfo *v1.ConfigMapInfo, componentType v1.ComponentType) ([]corev1.Volume, []corev1.VolumeMount) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	if cmInfo == nil {
		return volumes, volumeMounts
	}

	cms := GetMountConfigMapInfo(*cmInfo)

	if len(cms) != 0 {

		defaultMountPath := ""
		switch componentType {
		case v1.Component_FE, v1.Component_BE, v1.Component_CN, v1.Component_Broker:
			defaultMountPath = config_env_path
		default:
			klog.Infof("getConfigVolumeAndVolumeMount componentType %s not supported.", componentType)
		}

		for _, cm := range cms {
			path := cm.MountPath
			if cm.MountPath == "" {
				path = defaultMountPath
			}
			volumes = append(
				volumes,
				corev1.Volume{
					Name: cm.ConfigMapName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cm.ConfigMapName,
							},
						},
					},
				},
			)

			volumeMounts = append(
				volumeMounts,
				corev1.VolumeMount{
					Name:      cm.ConfigMapName,
					MountPath: path,
				},
			)
		}
	}
	return volumes, volumeMounts
}

func getMultiSecretVolumeAndVolumeMount(bSpec *v1.BaseSpec, componentType v1.ComponentType) ([]corev1.Volume, []corev1.VolumeMount) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	defaultMountPath := ""
	switch componentType {
	case v1.Component_FE, v1.Component_BE, v1.Component_CN, v1.Component_Broker:
		defaultMountPath = secret_config_path
	default:
		klog.Infof("getMultiSecretVolumeAndVolumeMount componentType %s not supported.", componentType)
	}

	for _, secret := range bSpec.Secrets {
		path := secret.MountPath
		if secret.MountPath == "" {
			path = defaultMountPath
		}
		volumes = append(
			volumes,
			corev1.Volume{
				Name: secret.SecretName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.SecretName,
					},
				},
			},
		)

		volumeMounts = append(
			volumeMounts,
			corev1.VolumeMount{
				Name:      secret.SecretName,
				MountPath: path,
			},
		)
	}
	return volumes, volumeMounts
}

func GetMultiSecretVolumeAndVolumeMountWithCommonSpec(cSpec *dv1.CommonSpec) ([]corev1.Volume, []corev1.VolumeMount) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	defaultMountPath := secret_config_path

	for _, secret := range cSpec.Secrets {
		path := secret.MountPath
		if secret.MountPath == "" {
			path = defaultMountPath
		}
		volumes = append(
			volumes,
			corev1.Volume{
				Name: secret.SecretName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.SecretName,
					},
				},
			},
		)

		volumeMounts = append(
			volumeMounts,
			corev1.VolumeMount{
				Name:      secret.SecretName,
				MountPath: path,
			},
		)
	}
	return volumes, volumeMounts
}

func LivenessProbe(port, timeout int32, path string, commands []string, pt ProbeType) *corev1.Probe {
	return livenessProbe(port, timeout, path, commands, pt)
}

func ReadinessProbe(port int32, path string, commands []string, pt ProbeType) *corev1.Probe {
	return readinessProbe(port, path, commands, pt)
}

// StartupProbe returns a startup probe.
func startupProbe(port, timeout int32, path string, commands []string, pt ProbeType) *corev1.Probe {
	var failurethreshold int32
	if timeout < 300 {
		timeout = 300
	}

	failurethreshold = timeout / 5
	return &corev1.Probe{
		FailureThreshold: failurethreshold,
		PeriodSeconds:    5,
		ProbeHandler:     getProbe(port, path, commands, pt),
	}
}

// livenessProbe returns a liveness.
func livenessProbe(port, timeout int32, path string, commands []string, pt ProbeType) *corev1.Probe {
	if timeout < 1 {
		timeout = 180
	}
	return &corev1.Probe{
		PeriodSeconds:    5,
		FailureThreshold: 3,
		// for pulling image and start doris
		InitialDelaySeconds: 80,
		TimeoutSeconds:      timeout,
		ProbeHandler:        getProbe(port, path, commands, pt),
	}
}

// ReadinessProbe returns a readiness probe.
func readinessProbe(port int32, path string, commands []string, pt ProbeType) *corev1.Probe {
	return &corev1.Probe{
		PeriodSeconds:    5,
		FailureThreshold: 3,
		ProbeHandler:     getProbe(port, path, commands, pt),
	}
}

// LifeCycle returns a lifecycle.
func lifeCycle(preStopScriptPath string) *corev1.Lifecycle {
	return &corev1.Lifecycle{
		PreStop: &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{
				Command: []string{preStopScriptPath},
			},
		},
	}
}

func LifeCycleWithPreStopScript(lc *corev1.Lifecycle, preStopScript string) *corev1.Lifecycle {
	if lc == nil {
		lc = &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{preStopScript},
				},
			},
		}

		return lc
	}

	lc.PreStop = &corev1.LifecycleHandler{
		Exec: &corev1.ExecAction{
			Command: []string{preStopScript},
		},
	}
	return lc
}

// getProbe describe a health check.
func getProbe(port int32, path string, commands []string, pt ProbeType) corev1.ProbeHandler {
	switch pt {
	case TcpSocket:
		return getTcpSocket(port)
	case HttpGet:
		return getHttpProbe(port, path)
	case Exec:
		return getExecProbe(commands)
	default:
	}
	return corev1.ProbeHandler{}
}

func getTcpSocket(port int32) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		TCPSocket: &corev1.TCPSocketAction{
			Port: intstr.FromInt32(port),
		},
	}
}

func getHttpProbe(port int32, path string) corev1.ProbeHandler {
	var p corev1.ProbeHandler
	if path != "" {
		p = corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: port,
				},
			},
		}
	}

	return p
}

func getExecProbe(commands []string) corev1.ProbeHandler {
	if len(commands) == 0 {
		return corev1.ProbeHandler{}
	}

	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: commands,
		},
	}
}

func BuildDisaggregatedProbe(container *corev1.Container, cs *dv1.CommonSpec, componentType dv1.DisaggregatedComponentType) {
	var failurethreshold int32
	startTimeout := int32(300)
	liveTimeout := cs.LiveTimeout
	if cs.StartTimeout >= 300 {
		startTimeout = cs.StartTimeout
	}
	failurethreshold = startTimeout / 5

	if liveTimeout < 1 {
		liveTimeout = 180
	}

	var commend string
	switch componentType {
	case dv1.DisaggregatedFE:
		commend = HEALTH_DISAGGREGATED_FE_PROBE_COMMAND
	case dv1.DisaggregatedBE:
		commend = HEALTH_DISAGGREGATED_BE_PROBE_COMMAND
	case dv1.DisaggregatedMS:
		commend = HEALTH_DISAGGREGATED_MS_PROBE_COMMAND
	default:
	}

	// check running status
	alive := corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{commend, DISAGGREGATED_LIVE_PARAM_ALIVE},
		},
	}

	// check ready status
	ready := corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{commend, DISAGGREGATED_LIVE_PARAM_READY},
		},
	}

	container.LivenessProbe = &corev1.Probe{
		PeriodSeconds:       5,
		FailureThreshold:    3,
		InitialDelaySeconds: 80,
		TimeoutSeconds:      liveTimeout,
		ProbeHandler:        alive,
	}

	container.StartupProbe = &corev1.Probe{
		FailureThreshold: failurethreshold,
		PeriodSeconds:    5,
		ProbeHandler:     alive,
	}

	container.ReadinessProbe = &corev1.Probe{
		PeriodSeconds:    5,
		FailureThreshold: 3,
		ProbeHandler:     ready,
	}
}

func getDefaultAffinity(componentType v1.ComponentType) *corev1.Affinity {
	// default Affinity rule is :
	// Pods of the same component should deploy on different hosts with Preferred scheduling.
	// weight is 20, weight range is 1-100
	podAffinityTerm := corev1.WeightedPodAffinityTerm{
		Weight: 20,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: v1.ComponentLabelKey, Operator: metav1.LabelSelectorOpIn, Values: []string{string(componentType)}},
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

func constructAffinity(dcrAffinity *corev1.Affinity, componentType v1.ComponentType) *corev1.Affinity {
	affinity := getDefaultAffinity(componentType)

	if dcrAffinity == nil {
		return affinity
	}

	dcrPodAntiAffinity := dcrAffinity.PodAntiAffinity
	if dcrPodAntiAffinity != nil {
		affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = dcrPodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, dcrPodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
	}

	affinity.NodeAffinity = dcrAffinity.NodeAffinity
	affinity.PodAffinity = dcrAffinity.PodAffinity

	return affinity
}

func constructBeDefaultInitContainer(defaultImage string) corev1.Container {
	return newBaseInitContainer(
		"default-init",
		&v1.SystemInitialization{
			Command:   []string{"/bin/sh"},
			InitImage: defaultImage,
			Args:      []string{"-c", "sysctl -w vm.max_map_count=2000000 && swapoff -a"},
		},
	)
}
