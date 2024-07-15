package computegroups

import (
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	sub "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strconv"
	"strings"
)

// env key
const (
	MS_ENDPOINT         string = "MS_ENDPOINT"
	CLOUD_UNIQUE_ID_PRE string = "CLOUD_UNIQUE_ID_PRE"
	CLUSTER_ID          string = "CLUSTER_ID"
	STATEFULSET_NAME    string = "STATEFULSET_NAME"
	INSTANCE_ID         string = "INSTANCE_ID"
	INSTANCE_NAME       string = "INSTANCE_NAME"
	MS_TOKEN            string = "MS_TOKEN"
)

const (
	DefaultStorageRootPath = "/opt/apache-doris/be/storage"
	StoragePathKey         = "storage_root_path"
	DefaultLogPath         = "/opt/apache-doris/be/log"
	LogPathKey             = "LOG_DIR"
	LogStoreName           = "be-log"
	StorageStorePreName    = "be-storage"
)

func (dccs *DisaggregatedComputeGroupsController) newCG2LayerSchedulerLabels(ddcName /*DisaggregatedClusterName*/, cgClusterId string) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName:           ddcName,
		dv1.DorisDisaggregatedComputeGroupClusterId: cgClusterId,
		dv1.DorisDisaggregatedOwnerReference:        ddcName,
	}
}

func (dccs *DisaggregatedComputeGroupsController) newCGPodsSelector(ddcName /*DisaggregatedClusterName*/, cgClusterId string) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName:           ddcName,
		dv1.DorisDisaggregatedComputeGroupClusterId: cgClusterId,
		dv1.DorisDisaggregatedPodType:               "compute",
	}
}

func (dccs *DisaggregatedComputeGroupsController) NewStatefulset(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup, cvs map[string]interface{}) *appv1.StatefulSet {
	st := resource.NewStatefulSetWithComputeGroup(cg)
	cgClusterId := ddc.GetCGClusterId(cg)
	matchLabels := dccs.newCGPodsSelector(ddc.Name, cgClusterId)

	//build metadata
	func() {
		st.Namespace = ddc.Namespace
		st.Name = ddc.GetCGStatefulsetName(cg)
		st.OwnerReferences = []metav1.OwnerReference{resource.GetOwnerReference(ddc)}
		st.Labels = dccs.newCG2LayerSchedulerLabels(ddc.Name, cgClusterId)
	}()

	// build st.spec
	func() {
		st.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
		_, _, vcts := dccs.buildVolumesVolumeMountsAndPVCs(cvs, cg)
		st.Spec.VolumeClaimTemplates = vcts
		st.Spec.ServiceName = ddc.GetCGServiceName(cg)
		pts := dccs.NewPodTemplateSpec(ddc, matchLabels, cvs, cg)
		st.Spec.Template = pts
	}()

	return st
}

func (dccs *DisaggregatedComputeGroupsController) NewPodTemplateSpec(ddc *dv1.DorisDisaggregatedCluster, selector map[string]string, cvs map[string]interface{}, cg *dv1.ComputeGroup) corev1.PodTemplateSpec {
	pts := resource.NewPodTemplateSpecWithCommonSpec(&cg.CommonSpec)
	//pod template metadata.
	func() {
		l := (resource.Labels)(selector)
		l.AddLabel(pts.Labels)
		pts.Labels = l
	}()

	c := dccs.NewCGContainer(ddc, cvs, cg)
	pts.Spec.Containers = append(pts.Spec.Containers, c)
	vs, _, _ := dccs.buildVolumesVolumeMountsAndPVCs(cvs, cg)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)
	return pts
}

func (dccs *DisaggregatedComputeGroupsController) NewCGContainer(ddc *dv1.DorisDisaggregatedCluster, cvs map[string]interface{}, cg *dv1.ComputeGroup) corev1.Container {
	c := resource.NewContainerWithCommonSpec(&cg.CommonSpec)
	resource.LifeCycleWithPreStopScript(c.Lifecycle, sub.GetDisaggregatedPreStopScript(dv1.DisaggregatedBE))
	cmd, args := sub.GetDisaggregatedCommand(dv1.DisaggregatedBE)
	c.Command = cmd
	c.Args = args
	c.Name = "compute"

	c.Ports = resource.GetDisaggregatedContainerPorts(cvs, dv1.DisaggregatedBE)
	c.Env = cg.CommonSpec.EnvVars
	c.Env = append(c.Env, resource.GetPodDefaultEnv()...)
	c.Env = append(c.Env, dccs.newSpecificEnvs(ddc, cg)...)

	c.LivenessProbe = dccs.newCGLivenessProbe(cvs)
	c.StartupProbe = dccs.newCGStartUpProbe(cvs)
	c.ReadinessProbe = dccs.newCGReadinessProbe(cvs)
	_, vms, _ := dccs.buildVolumesVolumeMountsAndPVCs(cvs, cg)
	c.VolumeMounts = vms
	return c
}

func (dccs *DisaggregatedComputeGroupsController) buildVolumesVolumeMountsAndPVCs(cvs map[string]interface{}, cg *dv1.ComputeGroup) ([]corev1.Volume, []corev1.VolumeMount, []corev1.PersistentVolumeClaim) {
	if cg.CommonSpec.PersistentVolume == nil {
		vs, vms := dccs.getDefaultVolumesVolumeMounts(cvs)
		return vs, vms, nil
	}

	var vs []corev1.Volume
	var vms []corev1.VolumeMount
	var pvcs []corev1.PersistentVolumeClaim
	logPath := dccs.getLogPath(cvs)
	vs = append(vs, corev1.Volume{Name: LogStoreName, VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: LogStoreName,
		}}})
	vms = append(vms, corev1.VolumeMount{Name: LogStoreName, MountPath: logPath})
	pvcs = append(pvcs, corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        LogStoreName,
			Annotations: cg.CommonSpec.PersistentVolume.Annotations,
		},
		Spec: cg.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec,
	})

	paths := dccs.getStoragePaths(cvs)
	for i, _ := range paths {
		vs = append(vs, corev1.Volume{Name: StorageStorePreName + strconv.Itoa(i), VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: StorageStorePreName + strconv.Itoa(i),
			}}})
		vms = append(vms, corev1.VolumeMount{Name: StorageStorePreName + strconv.Itoa(i), MountPath: paths[i]})
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        StorageStorePreName + strconv.Itoa(i),
				Annotations: cg.CommonSpec.PersistentVolume.Annotations,
			},
			Spec: cg.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec,
		})
	}

	return vs, vms, pvcs
}

// when not config persisentTemplateSpec, pod should mount emptyDir volume for storing data and log. mountPath resolve from config file.
func (dccs *DisaggregatedComputeGroupsController) getDefaultVolumesVolumeMounts(cvs map[string]interface{}) ([]corev1.Volume, []corev1.VolumeMount) {
	LogPath := dccs.getLogPath(cvs)
	vs := []corev1.Volume{
		{
			Name: LogStoreName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	vms := []corev1.VolumeMount{
		{
			Name:      LogStoreName,
			MountPath: LogPath,
		},
	}

	storagePaths := dccs.getStoragePaths(cvs)
	for i, path := range storagePaths {
		vs = append(vs, corev1.Volume{
			Name: StorageStorePreName + strconv.Itoa(i),
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		vms = append(vms, corev1.VolumeMount{
			Name:      StorageStorePreName + strconv.Itoa(i),
			MountPath: path,
		})
	}

	return vs, vms
}

func (dccs *DisaggregatedComputeGroupsController) getStoragePaths(cvs map[string]interface{}) []string {
	v := cvs[StoragePathKey]
	if v == nil {
		return []string{DefaultStorageRootPath}
	}

	//v format: /home/disk1/doris,medium:SSD;/home/disk2/doris,medium:SSD;/home/disk2/doris,medium:HDD
	spcs := strings.Split(v.(string), ":")
	var paths []string
	for _, spc := range spcs {
		a := strings.Split(spc, ",")
		paths = append(paths, a[0])
	}
	return paths
}

func (dccs *DisaggregatedComputeGroupsController) getLogPath(cvs map[string]interface{}) string {
	v := cvs[LogPathKey]
	if v == nil {
		return DefaultLogPath
	}
	//log path support use $DORIS_HOME as subPath.
	dev := map[string]string{
		"DORIS_HOME": "/opt/apache-doris/be",
	}
	mapping := func(key string) string {
		return dev[key]
	}
	path := os.Expand(v.(string), mapping)
	return path
}

func (dccs *DisaggregatedComputeGroupsController) newCGLivenessProbe(cvs /*config values*/ map[string]interface{}) *corev1.Probe {
	heartBeatPort := resource.GetPort(cvs, resource.HEARTBEAT_SERVICE_PORT)
	return resource.LivenessProbe(heartBeatPort, "")
}

func (dccs *DisaggregatedComputeGroupsController) newCGStartUpProbe(cvs /*config values*/ map[string]interface{}) *corev1.Probe {
	return dccs.newCGLivenessProbe(cvs)
}

func (dccs *DisaggregatedComputeGroupsController) newCGReadinessProbe(cvs /*config values*/ map[string]interface{}) *corev1.Probe {
	webserverPort := resource.GetPort(cvs, resource.WEBSERVER_PORT)
	return resource.ReadinessProbe(webserverPort, resource.HEALTH_API_PATH)
}

func (dccs *DisaggregatedComputeGroupsController) newSpecificEnvs(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) []corev1.EnvVar {
	var cgEnvs []corev1.EnvVar
	stsName := ddc.GetCGStatefulsetName(cg)
	clusterId := ddc.GetCGClusterId(cg)
	cloudUniqueIdPre := ddc.GetCGCloudUniqueIdPre(cg)

	//config in start reconcile, operator get DorisDisaggregatedMetaService to assign ms info.
	ms_endpoint := ddc.Status.MsEndpoint
	ms_token := ddc.Status.MsToken
	cgEnvs = append(cgEnvs,
		corev1.EnvVar{Name: MS_ENDPOINT, Value: ms_endpoint},
		corev1.EnvVar{Name: CLOUD_UNIQUE_ID_PRE, Value: cloudUniqueIdPre},
		corev1.EnvVar{Name: CLUSTER_ID, Value: clusterId},
		corev1.EnvVar{Name: INSTANCE_NAME, Value: ddc.Name},
		corev1.EnvVar{Name: INSTANCE_ID, Value: ddc.GetInstanceId()},
		corev1.EnvVar{Name: STATEFULSET_NAME, Value: stsName},
		corev1.EnvVar{Name: MS_TOKEN, Value: ms_token})
	return cgEnvs
}
