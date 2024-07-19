package disaggregated_fe

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	sub "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strconv"
)

const (
	MS_ENDPOINT      string = "MS_ENDPOINT"
	STATEFULSET_NAME string = "STATEFULSET_NAME"
	INSTANCE_ID      string = "INSTANCE_ID"
	INSTANCE_NAME    string = "INSTANCE_NAME"
	MS_TOKEN         string = "MS_TOKEN"
	CLUSTER_ID       string = "CLUSTER_ID"
	CLUSTER_NAME     string = "CLUSTER_NAME"
)

const (
	DefaultMetaPath = "/opt/apache-doris/fe/doris-meta"
	MetaPathKey     = "meta_dir"
	DefaultLogPath  = "/opt/apache-doris/fe/log"
	LogPathKey      = "LOG_DIR"
	LogStoreName    = "fe-log"
	MetaStoreName   = "fe-meta"
	FeClusterId     = "RESERVED_CLUSTER_ID_FOR_SQL_SERVER"
	FeClusterName   = "RESERVED_CLUSTER_NAME_FOR_SQL_SERVER"
)

var (
	Default_Election_Number int32 = 1
)

func (dfc *DisaggregatedFEController) newFEPodsSelector(ddcName string) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName:    ddcName,
		dv1.DorisDisaggregatedPodType:        "fe",
		dv1.DorisDisaggregatedOwnerReference: ddcName,
	}
}

func (dfc *DisaggregatedFEController) newFESchedulerLabels(ddcName string) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName: ddcName,
		dv1.DorisDisaggregatedPodType:     "fe",
	}
}

func (dfc *DisaggregatedFEController) NewStatefulset(ddc *dv1.DorisDisaggregatedCluster, confMap map[string]interface{}) *appv1.StatefulSet {
	spec := ddc.Spec.FeSpec
	selector := dfc.newFEPodsSelector(ddc.Name)
	_, _, vcts := dfc.buildVolumesVolumeMountsAndPVCs(confMap, &spec)
	pts := dfc.NewPodTemplateSpec(ddc, selector, confMap)
	st := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       ddc.Namespace,
			Name:            ddc.GetFEStatefulsetName(),
			OwnerReferences: []metav1.OwnerReference{resource.GetOwnerReference(ddc)},
			Labels:          dfc.newFESchedulerLabels(ddc.Name),
		},
		Spec: appv1.StatefulSetSpec{
			Replicas:             ddc.Spec.FeSpec.Replicas,
			Selector:             &metav1.LabelSelector{MatchLabels: selector},
			VolumeClaimTemplates: vcts,
			ServiceName:          ddc.GetFEServiceName(),
			Template:             pts,
			PodManagementPolicy:  appv1.ParallelPodManagement,
		},
	}
	return st
}

func (dfc *DisaggregatedFEController) NewPodTemplateSpec(ddc *dv1.DorisDisaggregatedCluster, selector map[string]string, confMap map[string]interface{}) corev1.PodTemplateSpec {
	pts := resource.NewPodTemplateSpecWithCommonSpec(&ddc.Spec.FeSpec.CommonSpec, dv1.DisaggregatedFE)
	//pod template metadata.
	func() {
		l := (resource.Labels)(selector)
		l.AddLabel(pts.Labels)
		pts.Labels = l
	}()

	c := dfc.NewFEContainer(ddc, confMap)
	pts.Spec.Containers = append(pts.Spec.Containers, c)
	vs, _, _ := dfc.buildVolumesVolumeMountsAndPVCs(confMap, &ddc.Spec.FeSpec)
	configVolumes, _ := dfc.buildConfigMapVolumesVolumeMounts(&ddc.Spec.FeSpec)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)
	pts.Spec.Volumes = append(pts.Spec.Volumes, configVolumes...)

	pts.Spec.Affinity = dfc.constructAffinity(dv1.DorisDisaggregatedClusterName, selector[dv1.DorisDisaggregatedClusterName], ddc.Spec.FeSpec.Affinity)

	return pts
}

func (dfc *DisaggregatedFEController) constructAffinity(matchKey, value string, ddcAffinity *corev1.Affinity) *corev1.Affinity {
	affinity := newFEDefaultAffinity(matchKey, value)

	if ddcAffinity == nil {
		return affinity
	}

	ddcPodAntiAffinity := ddcAffinity.PodAntiAffinity
	if ddcPodAntiAffinity != nil {
		affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = ddcPodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, ddcPodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
	}

	affinity.NodeAffinity = ddcAffinity.NodeAffinity
	affinity.PodAffinity = ddcAffinity.PodAffinity

	return affinity
}

func newFEDefaultAffinity(matchKey, value string) *corev1.Affinity {
	if matchKey == "" || value == "" {
		return nil
	}

	podAffinityTerm := corev1.WeightedPodAffinityTerm{
		Weight: 20,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: matchKey, Operator: metav1.LabelSelectorOpIn, Values: []string{value}},
				},
			},
			TopologyKey: resource.NODE_TOPOLOGYKEY,
		},
	}
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{podAffinityTerm},
		},
	}
}

func (dfc *DisaggregatedFEController) buildConfigMapVolumesVolumeMounts(fe *dv1.FeSpec) ([]corev1.Volume, []corev1.VolumeMount) {
	var vs []corev1.Volume
	var vms []corev1.VolumeMount
	for _, cm := range fe.ConfigMaps {
		v := corev1.Volume{
			Name: cm.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm.Name,
					},
				},
			},
		}

		vs = append(vs, v)
		vm := corev1.VolumeMount{
			Name:      cm.Name,
			MountPath: cm.MountPath,
		}
		if vm.MountPath == "" {
			vm.MountPath = resource.ConfigEnvPath
		}
		vms = append(vms, vm)
	}
	return vs, vms
}

func (dfc *DisaggregatedFEController) NewFEContainer(ddc *dv1.DorisDisaggregatedCluster, cvs map[string]interface{}) corev1.Container {

	if ddc.Spec.FeSpec.ElectionNumber == nil {
		ddc.Spec.FeSpec.ElectionNumber = resource.GetInt32Pointer(Default_Election_Number)
	}

	c := resource.NewContainerWithCommonSpec(&ddc.Spec.FeSpec.CommonSpec)
	resource.LifeCycleWithPreStopScript(c.Lifecycle, sub.GetDisaggregatedPreStopScript(dv1.DisaggregatedFE))
	cmd, args := sub.GetDisaggregatedCommand(dv1.DisaggregatedFE)
	c.Command = cmd
	c.Args = args
	c.Name = "fe"

	c.Ports = resource.GetDisaggregatedContainerPorts(cvs, dv1.DisaggregatedFE)
	c.Env = ddc.Spec.FeSpec.CommonSpec.EnvVars
	c.Env = append(c.Env, resource.GetPodDefaultEnv()...)
	c.Env = append(c.Env, dfc.newSpecificEnvs(ddc)...)
	resource.BuildDisaggregatedProbe(&c, ddc.Spec.FeSpec.StartTimeout, dv1.DisaggregatedFE)
	_, vms, _ := dfc.buildVolumesVolumeMountsAndPVCs(cvs, &ddc.Spec.FeSpec)
	_, cmvms := dfc.buildConfigMapVolumesVolumeMounts(&ddc.Spec.FeSpec)
	c.VolumeMounts = vms
	if c.VolumeMounts == nil {
		c.VolumeMounts = cmvms
	} else {
		c.VolumeMounts = append(c.VolumeMounts, cmvms...)
	}
	return c
}

func (dfc *DisaggregatedFEController) buildVolumesVolumeMountsAndPVCs(confMap map[string]interface{}, fe *dv1.FeSpec) ([]corev1.Volume, []corev1.VolumeMount, []corev1.PersistentVolumeClaim) {
	if fe.PersistentVolume == nil {
		vs, vms := dfc.getDefaultVolumesVolumeMounts(confMap)
		return vs, vms, nil
	}

	var vs []corev1.Volume
	var vms []corev1.VolumeMount
	var pvcs []corev1.PersistentVolumeClaim

	vs = append(vs, corev1.Volume{Name: LogStoreName, VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: LogStoreName,
		}}})
	vms = append(vms, corev1.VolumeMount{Name: LogStoreName, MountPath: dfc.getLogPath(confMap)})
	pvcs = append(pvcs, corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        LogStoreName,
			Annotations: fe.CommonSpec.PersistentVolume.Annotations,
		},
		Spec: fe.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec,
	})

	vs = append(vs, corev1.Volume{Name: MetaStoreName, VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: MetaStoreName,
		}}})
	vms = append(vms, corev1.VolumeMount{Name: MetaStoreName, MountPath: dfc.getMetaPath(confMap)})
	pvcs = append(pvcs, corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        MetaStoreName,
			Annotations: fe.CommonSpec.PersistentVolume.Annotations,
		},
		Spec: fe.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec,
	})

	return vs, vms, pvcs
}

// when not config persisentTemplateSpec, pod should mount emptyDir volume for meta data and log. mountPath resolve from config file.
func (dfc *DisaggregatedFEController) getDefaultVolumesVolumeMounts(confMap map[string]interface{}) ([]corev1.Volume, []corev1.VolumeMount) {
	vs := []corev1.Volume{
		{
			Name: LogStoreName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: MetaStoreName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	vms := []corev1.VolumeMount{
		{
			Name:      LogStoreName,
			MountPath: dfc.getLogPath(confMap),
		},
		{
			Name:      MetaStoreName,
			MountPath: dfc.getMetaPath(confMap),
		},
	}
	return vs, vms
}

func (dfc *DisaggregatedFEController) getLogPath(confMap map[string]interface{}) string {
	v := confMap[LogPathKey]
	if v == nil {
		return DefaultLogPath
	}
	//log path support use $DORIS_HOME as subPath.
	dev := map[string]string{
		"DORIS_HOME": "/opt/apache-doris/fe",
	}
	mapping := func(key string) string {
		return dev[key]
	}
	path := os.Expand(v.(string), mapping)
	return path
}

func (dfc *DisaggregatedFEController) getMetaPath(confMap map[string]interface{}) string {
	v := confMap[MetaPathKey]
	if v == nil {
		return DefaultMetaPath
	}
	return v.(string)
}

func (dfc *DisaggregatedFEController) newSpecificEnvs(ddc *dv1.DorisDisaggregatedCluster) []corev1.EnvVar {
	var feEnvs []corev1.EnvVar
	stsName := ddc.GetFEStatefulsetName()

	//config in start reconcile, operator get DorisDisaggregatedMetaService to assign ms info.
	ms_endpoint := ddc.Status.MsEndpoint
	ms_token := ddc.Status.MsToken
	feEnvs = append(feEnvs,
		corev1.EnvVar{Name: MS_ENDPOINT, Value: ms_endpoint},
		corev1.EnvVar{Name: CLUSTER_ID, Value: FeClusterId},
		corev1.EnvVar{Name: CLUSTER_NAME, Value: FeClusterName},
		corev1.EnvVar{Name: INSTANCE_NAME, Value: ddc.Name},
		corev1.EnvVar{Name: INSTANCE_ID, Value: ddc.GetInstanceId()},
		corev1.EnvVar{Name: STATEFULSET_NAME, Value: stsName},
		corev1.EnvVar{Name: MS_TOKEN, Value: ms_token},
		corev1.EnvVar{Name: resource.ENV_FE_ELECT_NUMBER, Value: strconv.FormatInt(int64(*ddc.Spec.FeSpec.ElectionNumber), 10)},
	)
	return feEnvs
}
