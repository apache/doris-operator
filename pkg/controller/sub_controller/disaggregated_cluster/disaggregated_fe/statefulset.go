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
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_http"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	sub "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kr "k8s.io/apimachinery/pkg/api/resource"
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
	DefaultMetaPath          = "/opt/apache-doris/fe/doris-meta"
	MetaPathKey              = "meta_dir"
	DefaultLogPath           = "/opt/apache-doris/fe/log"
	LogPathKey               = "LOG_DIR"
	LogStoreName             = "fe-log"
	MetaStoreName            = "fe-meta"
	DefaultStorageSize int64 = 107374182400
)

var (
	DefaultElectionNumber  int32 = 1
	DefaultFeReplicaNumber int32 = 2
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
	if *ddc.Spec.FeSpec.Replicas < DefaultFeReplicaNumber {
		ddc.Spec.FeSpec.Replicas = &(DefaultFeReplicaNumber)
	}
	selector := dfc.newFEPodsSelector(ddc.Name)
	_, _, vcts := dfc.buildVolumesVolumeMountsAndPVCs(confMap, &spec)
	pts := dfc.NewPodTemplateSpec(ddc, selector, confMap)
	st := dfc.NewDefaultStatefulset(ddc)
	//metadata
	func() {
		st.Name = ddc.GetFEStatefulsetName()
		st.Labels = dfc.newFESchedulerLabels(ddc.Name)
	}()

	func() {
		st.Spec.Replicas = ddc.Spec.FeSpec.Replicas
		st.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
		st.Spec.VolumeClaimTemplates = vcts
		st.Spec.ServiceName = ddc.GetFEServiceName()
		st.Spec.Template = pts
	}()

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
	configVolumes, _ := dfc.BuildDefaultConfigMapVolumesVolumeMounts(ddc.Spec.FeSpec.ConfigMaps)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)
	pts.Spec.Volumes = append(pts.Spec.Volumes, configVolumes...)

	pts.Spec.Affinity = dfc.ConstructDefaultAffinity(dv1.DorisDisaggregatedClusterName, selector[dv1.DorisDisaggregatedClusterName], ddc.Spec.FeSpec.Affinity)

	return pts
}

func (dfc *DisaggregatedFEController) NewFEContainer(ddc *dv1.DorisDisaggregatedCluster, cvs map[string]interface{}) corev1.Container {
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
	_, cmvms := dfc.BuildDefaultConfigMapVolumesVolumeMounts(ddc.Spec.FeSpec.ConfigMaps)
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

	func() {
		defQuantity := kr.NewQuantity(DefaultStorageSize, kr.BinarySI)
		if fe.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests == nil {
			fe.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests = map[corev1.ResourceName]kr.Quantity{}
		}
		pvcSize := fe.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage]
		cmp := defQuantity.Cmp(pvcSize)
		if cmp > 0 {
			fe.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage] = *defQuantity
		}

		if len(fe.PersistentVolume.PersistentVolumeClaimSpec.AccessModes) == 0 {
			fe.PersistentVolume.PersistentVolumeClaimSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		}
	}()

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
		Spec: *fe.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec.DeepCopy(),
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
		Spec: *fe.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec.DeepCopy(),
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
	//log path support use $DORIS_HOME as subPath.
	dev := map[string]string{
		"DORIS_HOME": "/opt/apache-doris/fe",
	}
	mapping := func(key string) string {
		return dev[key]
	}
	//resolve relative path to absolute path
	path := os.Expand(v.(string), mapping)
	return path
}

func (dfc *DisaggregatedFEController) newSpecificEnvs(ddc *dv1.DorisDisaggregatedCluster) []corev1.EnvVar {
	var feEnvs []corev1.EnvVar
	stsName := ddc.GetFEStatefulsetName()

	//config in start reconcile, operator get DorisDisaggregatedMetaService to assign ms info.
	ms_endpoint := ddc.Status.MetaServiceStatus.MetaServiceEndpoint
	ms_token := ddc.Status.MetaServiceStatus.MsToken
	feEnvs = append(feEnvs,
		corev1.EnvVar{Name: MS_ENDPOINT, Value: ms_endpoint},
		corev1.EnvVar{Name: CLUSTER_ID, Value: ms_http.FeClusterId},
		corev1.EnvVar{Name: CLUSTER_NAME, Value: ms_http.FeClusterName},
		corev1.EnvVar{Name: INSTANCE_NAME, Value: ddc.Name},
		corev1.EnvVar{Name: INSTANCE_ID, Value: ddc.GetInstanceId()},
		corev1.EnvVar{Name: STATEFULSET_NAME, Value: stsName},
		corev1.EnvVar{Name: MS_TOKEN, Value: ms_token},
		corev1.EnvVar{Name: resource.ENV_FE_ELECT_NUMBER, Value: strconv.FormatInt(int64(DefaultElectionNumber), 10)},
	)
	return feEnvs
}
