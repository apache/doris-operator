package resource

import (
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
)

const (
	config_env_path = "/etc/doris"
	config_env_name = "CONFIGMAP_MOUNT_PATH"
	be_storage_name = "be-storage"
	be_storage_path = "/opt/doris/be/storage"
	fe_meta_path    = "/opt/doris/fe/doris-meta"
	fe_meta_name    = "fe-meta"
	HEALTH_API_PATH = "/api/health"
)

func NewPodTemplateSpc(dcr *v1.DorisCluster, componentType v1.ComponentType) corev1.PodTemplateSpec {
	spec := getBaseSpecFromCluster(dcr, componentType)
	var volumes []corev1.Volume
	switch componentType {
	case v1.Component_FE:
		volumes = newVolumesFromBaseSpec(dcr.Spec.FeSpec.BaseSpec)
	case v1.Component_BE:
		volumes = newVolumesFromBaseSpec(dcr.Spec.BeSpec.BaseSpec)
	default:
		klog.Errorf("NewPodTemplateSpc dorisClusterName %s, namespace %s componentType %s not supported.", dcr.Name, dcr.Namespace, componentType)
	}

	if len(volumes) == 0 {
		volumes = newDefaultVolume(componentType)
	}

	return corev1.PodTemplateSpec{
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
		},
	}
}

func newDefaultVolume(componentType v1.ComponentType) []corev1.Volume {
	switch componentType {
	case v1.Component_FE:
		return []corev1.Volume{{
			Name: fe_meta_name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}}
	case v1.Component_BE:
		return []corev1.Volume{{
			Name: be_storage_name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}}
	default:
		klog.Infof("newDefaultVolume have not support componentType %s", componentType)
		return []corev1.Volume{}
	}
}

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

func NewBaseMainContainer(spec v1.BaseSpec, config map[string]interface{}, componentType v1.ComponentType) corev1.Container {
	command, args := getCommand(componentType)
	volumeMounts := buildVolumeMounts(spec, componentType)
	var envs []corev1.EnvVar
	envs = append(envs, buildBaseEnvs()...)
	envs = append(envs, spec.EnvVars...)
	if spec.ConfigMapInfo.ConfigMapName != "" && spec.ConfigMapInfo.ResolveKey != "" {
		envs = append(envs, corev1.EnvVar{
			Name:  config_env_name,
			Value: config_env_path,
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
	}

	var healthPort int32
	switch componentType {
	case v1.Component_FE:
		healthPort = GetPort(config, HTTP_PORT)
	case v1.Component_BE:
		healthPort = GetPort(config, WEBSERVER_PORT)
	default:
		klog.Infof("the componentType %s is not supported in probe.")
	}

	if healthPort != 0 {
		c.LivenessProbe = livenessProbe(healthPort, HEALTH_API_PATH)
		c.StartupProbe = startupProbe(healthPort, HEALTH_API_PATH)
		c.ReadinessProbe = readinessProbe(healthPort, HEALTH_API_PATH)
	}

	return c
}

func buildBaseEnvs() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
			},
		},
		{
			Name: "HOST_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			},
		},
		{
			Name:  "USER",
			Value: "root",
		}, {
			Name:  "DORIS_ROOT",
			Value: "/opt/doris",
		},
	}
}

func buildVolumeMounts(spec v1.BaseSpec, componentType v1.ComponentType) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	if len(spec.PersistentVolumes) == 0 {
		_, volumeMount := GetDefaultVolumesVolumeMountsAndPersistentVolumeClaims(componentType)
		volumeMounts = append(volumeMounts, volumeMount...)
		return volumeMounts
	}

	for _, pvs := range spec.PersistentVolumes {
		var volumeMount corev1.VolumeMount
		volumeMount.MountPath = pvs.MountPath
		volumeMount.Name = pvs.Name
	}
	return volumeMounts
}

func getCommand(componentType v1.ComponentType) (commands []string, args []string) {
	switch componentType {
	case v1.Component_FE:
		return []string{"/opt/doris/fe_entrypoint.sh"}, []string{"$(ENV_FE_ADDR)"}
	case v1.Component_BE:
		return []string{"/opt/doris/be_entrypoint.sh"}, []string{"$(ENV_FE_ADDR)"}
		//TODO: cn and broker.
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

func GetDefaultVolumesVolumeMountsAndPersistentVolumeClaims(componentType v1.ComponentType) ([]corev1.Volume, []corev1.VolumeMount) {
	switch componentType {
	case v1.Component_FE:
		return getFeDefaultVolumesVolumeMounts()
	case v1.Component_BE:
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
		case v1.Component_FE:
		case v1.Component_BE:
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
		PeriodSeconds:    5,
		FailureThreshold: 3,
		ProbeHandler:     getProbe(port, path),
	}
}

// ReadinessProbe returns a readiness probe.
func readinessProbe(port int32, path string) *corev1.Probe {
	return &corev1.Probe{
		PeriodSeconds:    5,
		FailureThreshold: 3,
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
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: path,
			Port: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: port,
			},
		},
	}
}
