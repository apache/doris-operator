package resource

import (
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"strconv"
)

const (
	START_MS_COMMAND       = "/opt/apache-doris/ms_disaggregated_entrypoint.sh"
	START_RC_COMMAND       = "/opt/apache-doris/ms_disaggregated_entrypoint.sh"
	START_MS_PARAMETER     = "meta-service"
	START_RC_PARAMETER     = "recycler"
	HEALTH_MS_LIVE_COMMAND = "/opt/apache-doris/ms_disaggregated_is_alive.sh"
	HEALTH_RC_LIVE_COMMAND = "/opt/apache-doris/ms_disaggregated_is_alive.sh"
	PRESTOP_MS_COMMAND     = "/opt/apache-doris/ms_disaggregated_prestop.sh"
	PRESTOP_RC_COMMAND     = "/opt/apache-doris/ms_disaggregated_prestop.sh"
)

func NewDMSPodTemplateSpec(dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) corev1.PodTemplateSpec {
	spec := GetDMSBaseSpecFromCluster(dms, componentType)
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
func buildDMSVolumeMounts(spec mv1.BaseSpec) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	_, volumeMounts = appendPodInfoVolumesVolumeMounts(nil, volumeMounts)

	for _, pvs := range spec.PersistentVolumes {
		var volumeMount corev1.VolumeMount
		volumeMount.MountPath = pvs.MountPath
		volumeMount.Name = pvs.Name
		volumeMounts = append(volumeMounts, volumeMount)
	}

	return volumeMounts
}

func NewDMSBaseMainContainer(dms *mv1.DorisDisaggregatedMetaService, config map[string]interface{}, componentType mv1.ComponentType) corev1.Container {
	var envs []corev1.EnvVar
	var port int32
	var prestopScript string
	spec := GetDMSBaseSpecFromCluster(dms, componentType)

	command, args := buildDMSEntrypointCommand(componentType)

	fdbEndPoint := mv1.GetFDBEndPoint(dms)
	envs = append(envs, buildDMSBaseEnvs()...)
	envs = append(envs,
		corev1.EnvVar{
			Name:  FDB_ENDPOINT,
			Value: fdbEndPoint,
		}, corev1.EnvVar{
			Name: "OPERATOR_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.namespace",
				},
			},
		},
	)
	envs = mergeEnvs(envs, spec.EnvVars)

	volumeMounts := buildDMSVolumeMounts(*spec)
	if len(spec.ConfigMaps) != 0 {
		_, configVolumeMounts := getConfigmapVolumeAndVolumeMount(spec.ConfigMaps)
		volumeMounts = append(volumeMounts, configVolumeMounts...)
	}

	imagePullPolicy := spec.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = defaultDMSImagePullPolicy
	}

	switch componentType {
	case mv1.Component_MS:
		port = GetPort(config, MS_BRPC_LISTEN_PORT)
		prestopScript = PRESTOP_MS_COMMAND
	case mv1.Component_RC:
		port = GetPort(config, RC_BRPC_LISTEN_PORT)
		prestopScript = PRESTOP_RC_COMMAND
	default:
		klog.Infof("the componentType %s is not supported in probe.")
	}

	return corev1.Container{
		Image:           spec.Image,
		Command:         command,
		Args:            args,
		Ports:           []corev1.ContainerPort{},
		Env:             envs,
		VolumeMounts:    volumeMounts,
		ImagePullPolicy: imagePullPolicy,
		Resources:       spec.ResourceRequirements,
		LivenessProbe:   dmsLivenessProbe(port),
		StartupProbe:    dmsStartupProbe(port),
		ReadinessProbe:  dmsReadinessProbe(port),
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{prestopScript},
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

func GetDMSBaseSpecFromCluster(dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) *mv1.BaseSpec {
	var bSpec *mv1.BaseSpec
	switch componentType {
	case mv1.Component_MS:
		bSpec = &dms.Spec.MS.BaseSpec
	default:
		klog.Infof("the componentType %s is not supported!", componentType)
	}

	return bSpec
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
	return &corev1.Probe{
		FailureThreshold: 60,
		PeriodSeconds:    5,
		ProbeHandler:     getDMSProbe(port),
	}
}

// dmsLivenessProbe returns a liveness.
func dmsLivenessProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		PeriodSeconds:    5,
		FailureThreshold: 3,
		// for pulling image and start doris
		InitialDelaySeconds: 80,
		TimeoutSeconds:      180,
		ProbeHandler:        getDMSProbe(port),
	}
}

// ReadinessProbe returns a readiness probe.
func dmsReadinessProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		TimeoutSeconds:   3,
		SuccessThreshold: 1,
		PeriodSeconds:    5,
		FailureThreshold: 3,
		ProbeHandler:     getDMSProbe(port),
	}
}

// getProbe describe a health check.
func getDMSProbe(port int32) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{HEALTH_MS_LIVE_COMMAND, strconv.Itoa(int(port))},
		},
	}

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
