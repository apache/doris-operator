package resource

import (
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"strconv"
)

const (
	config_env_path = "/etc/doris"
	config_env_name = "CONFIGMAP_MOUNT_PATH"
	be_storage_name = "be-storage"
	be_storage_path = "/opt/apache-doris/be/storage"
	fe_meta_path    = "/opt/apache-doris/fe/doris-meta"
	fe_meta_name    = "fe-meta"

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
)

func NewPodTemplateSpec(dcr *v1.DorisCluster, componentType v1.ComponentType) corev1.PodTemplateSpec {
	spec := getBaseSpecFromCluster(dcr, componentType)
	var volumes []corev1.Volume
	var si *v1.SystemInitialization
	var dcrAffinity *corev1.Affinity
	var defaultInitContainers []corev1.Container
	switch componentType {
	case v1.Component_FE:
		volumes = newVolumesFromBaseSpec(dcr.Spec.FeSpec.BaseSpec)
		si = dcr.Spec.FeSpec.BaseSpec.SystemInitialization
		dcrAffinity = dcr.Spec.FeSpec.BaseSpec.Affinity
	case v1.Component_BE:
		volumes = newVolumesFromBaseSpec(dcr.Spec.BeSpec.BaseSpec)
		si = dcr.Spec.BeSpec.BaseSpec.SystemInitialization
		dcrAffinity = dcr.Spec.BeSpec.BaseSpec.Affinity
		//TODO: for icbc cancel
		//defaultInitContainers = append(defaultInitContainers, constructBeDefaultInitContainer())
	case v1.Component_CN:
		si = dcr.Spec.CnSpec.BaseSpec.SystemInitialization
		dcrAffinity = dcr.Spec.CnSpec.BaseSpec.Affinity
		//TODO: for icbc cancel

		//defaultInitContainers = append(defaultInitContainers, constructBeDefaultInitContainer())
	case v1.Component_Broker:
		si = dcr.Spec.BrokerSpec.BaseSpec.SystemInitialization
		dcrAffinity = dcr.Spec.BrokerSpec.BaseSpec.Affinity
	default:
		klog.Errorf("NewPodTemplateSpec dorisClusterName %s, namespace %s componentType %s not supported.", dcr.Name, dcr.Namespace, componentType)
	}

	if len(volumes) == 0 {
		volumes, _ = getDefaultVolumesVolumeMounts(componentType)
	}
	volumes, _ = appendPodInfoVolumesVolumeMounts(volumes, nil)

	if spec.ConfigMapInfo.ConfigMapName != "" && spec.ConfigMapInfo.ResolveKey != "" {
		configVolume, _ := getConfigVolumeAndVolumeMount(&spec.ConfigMapInfo, componentType)
		volumes = append(volumes, configVolume)
	}

	pts := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generatePodTemplateName(dcr, componentType),
			Labels: v1.GetPodLabels(dcr, componentType),
		},

		Spec: corev1.PodSpec{
			ImagePullSecrets:   spec.ImagePullSecrets,
			NodeSelector:       spec.NodeSelector,
			Volumes:            volumes,
			ServiceAccountName: spec.ServiceAccount,
			Affinity:           spec.Affinity,
			Tolerations:        spec.Tolerations,
			HostAliases:        spec.HostAliases,
			InitContainers:     defaultInitContainers,
		},
	}

	if si != nil {
		initContainer := newBaseInitContainer("init", si)
		pts.Spec.InitContainers = append(pts.Spec.InitContainers, initContainer)
	}

	pts.Spec.Affinity = constructAffinity(dcrAffinity, componentType)

	return pts
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
		initImage = "selectdb/alpine:latest"
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
	if spec.ConfigMapInfo.ConfigMapName != "" && spec.ConfigMapInfo.ResolveKey != "" {
		envs = append(envs, corev1.EnvVar{
			Name:  config_env_name,
			Value: config_env_path,
		})

		_, volumeMount := getConfigVolumeAndVolumeMount(&spec.ConfigMapInfo, componentType)
		volumeMounts = append(volumeMounts, volumeMount)
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

	var healthPort int32
	var prestopScript string
	var health_api_path string
	switch componentType {
	case v1.Component_FE:
		healthPort = GetPort(config, HTTP_PORT)
		prestopScript = FE_PRESTOP
		health_api_path = HEALTH_API_PATH
	case v1.Component_BE, v1.Component_CN:
		healthPort = GetPort(config, WEBSERVER_PORT)
		prestopScript = BE_PRESTOP
		health_api_path = HEALTH_API_PATH
	case v1.Component_Broker:
		healthPort = GetPort(config, BROKER_IPC_PORT)
		prestopScript = BROKER_PRESTOP
		health_api_path = ""
	default:
		klog.Infof("the componentType %s is not supported in probe.")
	}

	if healthPort != 0 {
		c.LivenessProbe = livenessProbe(healthPort, health_api_path)
		c.StartupProbe = startupProbe(healthPort, health_api_path)
		c.ReadinessProbe = readinessProbe(healthPort, health_api_path)
		c.Lifecycle = lifeCycle(prestopScript)
	}

	return c
}

func buildBaseEnvs(dcr *v1.DorisCluster) []corev1.EnvVar {
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
	}

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

func generatePodTemplateName(dcr *v1.DorisCluster, componentType v1.ComponentType) string {
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

func getConfigVolumeAndVolumeMount(cmInfo *v1.ConfigMapInfo, componentType v1.ComponentType) (corev1.Volume, corev1.VolumeMount) {
	var volume corev1.Volume
	var volumeMount corev1.VolumeMount
	if cmInfo.ConfigMapName != "" && cmInfo.ResolveKey != "" {
		volume = corev1.Volume{
			Name: cmInfo.ConfigMapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cmInfo.ConfigMapName,
					},
				},
			},
		}
		volumeMount = corev1.VolumeMount{
			Name: cmInfo.ConfigMapName,
		}

		switch componentType {
		case v1.Component_FE, v1.Component_BE, v1.Component_CN, v1.Component_Broker:
			volumeMount.MountPath = config_env_path
		default:
			klog.Infof("getConfigVolumeAndVolumeMount componentType %s not supported.", componentType)
		}
	}

	return volume, volumeMount
}

// StartupProbe returns a startup probe.
func startupProbe(port int32, path string) *corev1.Probe {
	return &corev1.Probe{
		FailureThreshold: 60,
		PeriodSeconds:    5,
		ProbeHandler:     getProbe(port, path),
	}
}

// livenessProbe returns a liveness.
func livenessProbe(port int32, path string) *corev1.Probe {
	return &corev1.Probe{
		PeriodSeconds:       5,
		FailureThreshold:    3,
		TimeoutSeconds:      180,
		InitialDelaySeconds: 120,
		ProbeHandler:        getProbe(port, path),
	}
}

// ReadinessProbe returns a readiness probe.
func readinessProbe(port int32, path string) *corev1.Probe {
	return &corev1.Probe{
		PeriodSeconds:    5,
		FailureThreshold: 3,
		TimeoutSeconds:   180,
		ProbeHandler:     getProbe(port, path),
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

func getProbe(port int32, path string) corev1.ProbeHandler {
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
	} else {
		p = corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{HEALTH_BROKER_LIVE_COMMAND, strconv.Itoa(int(port))},
			},
		}
	}

	return p
}

func getDefaultAffinity(componentType v1.ComponentType) *corev1.Affinity {
	// default Affinity rule is :
	// Pods of the same component should deploy on different hosts with Preferred scheduling.
	// weight is 100, weight range is 1-100
	podAffinityTerm := corev1.WeightedPodAffinityTerm{
		Weight: 100,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: v1.ComponentLabelKey, Operator: metav1.LabelSelectorOpIn, Values: []string{string(componentType)}},
				},
			},
			TopologyKey: "kubernetes.io/hostname",
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

func constructBeDefaultInitContainer() corev1.Container {
	return newBaseInitContainer(
		"default-init",
		&v1.SystemInitialization{
			Command: []string{"/bin/sh"},
			Args:    []string{"-c", "sysctl -w vm.max_map_count=2000000 && swapoff -a"},
		},
	)
}
