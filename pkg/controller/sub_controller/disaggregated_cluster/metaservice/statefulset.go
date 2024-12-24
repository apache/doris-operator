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

package metaservice

import (
	"context"
	"github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/metadata"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kr "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

const (
	defaultLogPrefixName       = "log"
	fdbClusterFileKey          = "cluster-file"
	logPathKey                 = "log_dir"
	defaultLogPath             = "/opt/apache-doris/ms/log"
	DefaultStorageSize   int64 = 107374182400
)

func (dms *DisaggregatedMSController) newMSPodsSelector(ddcName string) map[string]string {
	return map[string]string{
		v1.DorisDisaggregatedClusterName:    ddcName,
		v1.DorisDisaggregatedPodType:        "ms",
		v1.DorisDisaggregatedOwnerReference: ddcName,
	}
}

func (dms *DisaggregatedMSController) newMSSchedulerLabels(ddcName string) map[string]string {
	return map[string]string{
		v1.DorisDisaggregatedClusterName: ddcName,
		v1.DorisDisaggregatedPodType:     "ms",
	}
}

func (dms *DisaggregatedMSController) newStatefulset(ddc *v1.DorisDisaggregatedCluster, confMap map[string]interface{}) *appv1.StatefulSet {
	st := dms.NewDefaultStatefulset(ddc)
	func() {
		st.Name = ddc.GetMSStatefulsetName()
		st.Labels = dms.newMSSchedulerLabels(ddc.Name)
	}()

	msSpec := ddc.Spec.MetaService
	matchLabels := dms.newMSPodsSelector(ddc.Name)
	var volumeClaimTemplates []corev1.PersistentVolumeClaim
	cpv := msSpec.PersistentVolume
	if cpv != nil {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        defaultLogPrefixName,
				Annotations: resource.NewAnnotations(),
			},
			Spec: cpv.PersistentVolumeClaimSpec,
		}
		volumeClaimTemplates = append(volumeClaimTemplates, pvc)
	}

	replicas := metadata.GetInt32Pointer(v1.DefaultMetaserviceNumber)

	func() {
		st.Spec.Replicas = replicas
		st.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
		st.Spec.Template = dms.NewPodTemplateSpec(ddc, matchLabels, confMap)
		st.Spec.ServiceName = ddc.GetMSServiceName()
		st.Spec.VolumeClaimTemplates = volumeClaimTemplates
	}()

	return st
}

func (dms *DisaggregatedMSController) NewPodTemplateSpec(ddc *v1.DorisDisaggregatedCluster, selector map[string]string, confMap map[string]interface{}) corev1.PodTemplateSpec {
	pts := resource.NewPodTemplateSpecWithCommonSpec(&ddc.Spec.MetaService.CommonSpec, v1.DisaggregatedMS)
	//pod template metadata.
	func() {
		l := (resource.Labels)(selector)
		l.AddLabel(pts.Labels)
		pts.Labels = l
	}()

	c := dms.NewMSContainer(ddc, confMap)
	pts.Spec.Containers = append(pts.Spec.Containers, c)
	vs, _, _ := dms.buildVolumesVolumeMountsAndPVCs(confMap, &ddc.Spec.MetaService)
	configVolumes, _ := dms.BuildDefaultConfigMapVolumesVolumeMounts(ddc.Spec.MetaService.ConfigMaps)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)
	pts.Spec.Volumes = append(pts.Spec.Volumes, configVolumes...)
	pts.Spec.Affinity = dms.ConstructDefaultAffinity(v1.DorisDisaggregatedClusterName, selector[v1.DorisDisaggregatedClusterName], ddc.Spec.MetaService.Affinity)

	if len(ddc.Spec.MetaService.Secrets) != 0 {
		secretVolumes, _ := resource.GetMultiSecretVolumeAndVolumeMountWithCommonSpec(&ddc.Spec.MetaService.CommonSpec)
		pts.Spec.Volumes = append(pts.Spec.Volumes, secretVolumes...)
	}

	return pts
}

func (dms *DisaggregatedMSController) buildVolumesVolumeMountsAndPVCs(confMap map[string]interface{}, ms *v1.MetaService) ([]corev1.Volume, []corev1.VolumeMount, []corev1.PersistentVolumeClaim) {
	if ms.PersistentVolume == nil {
		vs, vms := dms.getDefaultVolumesVolumeMounts(confMap)
		return vs, vms, nil
	}

	var vs []corev1.Volume
	var vms []corev1.VolumeMount
	var pvcs []corev1.PersistentVolumeClaim

	func() {
		defQuantity := kr.NewQuantity(DefaultStorageSize, kr.BinarySI)
		if ms.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests == nil {
			ms.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests = map[corev1.ResourceName]kr.Quantity{}
		}
		pvcSize := ms.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage]
		cmp := defQuantity.Cmp(pvcSize)
		if cmp > 0 {
			ms.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage] = *defQuantity
		}

		if len(ms.PersistentVolume.PersistentVolumeClaimSpec.AccessModes) == 0 {
			ms.PersistentVolume.PersistentVolumeClaimSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		}
	}()

	vs = append(vs, corev1.Volume{Name: defaultLogPrefixName, VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: defaultLogPrefixName,
		}}})
	vms = append(vms, corev1.VolumeMount{Name: defaultLogPrefixName, MountPath: dms.getLogPath(confMap)})
	pvcs = append(pvcs, corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        defaultLogPrefixName,
			Annotations: ms.CommonSpec.PersistentVolume.Annotations,
		},
		Spec: *ms.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec.DeepCopy(),
	})

	return vs, vms, pvcs
}

func (dms *DisaggregatedMSController) getDefaultVolumesVolumeMounts(confMap map[string]interface{}) ([]corev1.Volume, []corev1.VolumeMount) {
	vs := []corev1.Volume{
		{
			Name: "ms-log",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	vms := []corev1.VolumeMount{
		{
			Name:      "ms-log",
			MountPath: dms.getLogPath(confMap),
		},
	}
	return vs, vms
}

func (dms *DisaggregatedMSController) getLogPath(confMap map[string]interface{}) string {
	v := confMap[logPathKey]
	if v == nil {
		return defaultLogPath
	}
	//log path support use $DORIS_HOME as subPath.
	dev := map[string]string{
		"DORIS_HOME": "/opt/apache-doris/ms",
	}
	mapping := func(key string) string {
		return dev[key]
	}
	path := os.Expand(v.(string), mapping)
	return path
}

func (dms *DisaggregatedMSController) NewMSContainer(ddc *v1.DorisDisaggregatedCluster, cvs map[string]interface{}) corev1.Container {
	c := resource.NewContainerWithCommonSpec(&ddc.Spec.MetaService.CommonSpec)

	c.Lifecycle = resource.LifeCycleWithPreStopScript(c.Lifecycle, sc.GetDisaggregatedPreStopScript(v1.DisaggregatedMS))
	cmd, args := sc.GetDisaggregatedCommand(v1.DisaggregatedMS)
	c.Command = cmd
	c.Args = args
	c.Name = "metaservice"

	c.Ports = resource.GetDisaggregatedContainerPorts(cvs, v1.DisaggregatedMS)
	c.Env = ddc.Spec.MetaService.CommonSpec.EnvVars
	c.Env = append(c.Env, resource.GetPodDefaultEnv()...)
	c.Env = append(c.Env, dms.newSpecificEnvs(ddc)...)
	resource.BuildDisaggregatedProbe(&c, &ddc.Spec.MetaService.CommonSpec, v1.DisaggregatedMS)
	_, vms, _ := dms.buildVolumesVolumeMountsAndPVCs(cvs, &ddc.Spec.MetaService)
	_, cmvms := dms.BuildDefaultConfigMapVolumesVolumeMounts(ddc.Spec.MetaService.ConfigMaps)
	c.VolumeMounts = vms
	if c.VolumeMounts == nil {
		c.VolumeMounts = cmvms
	} else {
		c.VolumeMounts = append(c.VolumeMounts, cmvms...)
	}

	if len(ddc.Spec.MetaService.Secrets) != 0 {
		_, secretVolumeMounts := resource.GetMultiSecretVolumeAndVolumeMountWithCommonSpec(&ddc.Spec.MetaService.CommonSpec)
		c.VolumeMounts = append(c.VolumeMounts, secretVolumeMounts...)
	}

	return c
}

func (dms *DisaggregatedMSController) newSpecificEnvs(ddc *v1.DorisDisaggregatedCluster) []corev1.EnvVar {
	msSpec := ddc.Spec.MetaService
	if msSpec.FDB.Address == "" && (msSpec.FDB.ConfigMapNamespaceName.Namespace == "" || msSpec.FDB.ConfigMapNamespaceName.Name == "") {
		dms.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.FDBAddressNotConfiged), "fdb not configed in spec")
		return nil
	}

	var fdbEndpoint string
	if msSpec.FDB.ConfigMapNamespaceName.Namespace != "" && msSpec.FDB.ConfigMapNamespaceName.Name != "" {
		cm, err := k8s.GetConfigMap(context.Background(), dms.K8sclient, msSpec.FDB.ConfigMapNamespaceName.Namespace, msSpec.FDB.ConfigMapNamespaceName.Name)
		if err != nil {
			dms.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.FDBAddressNotConfiged), "configmap "+"namespace"+msSpec.FDB.ConfigMapNamespaceName.Namespace+" name "+msSpec.FDB.ConfigMapNamespaceName.Name+" find failed "+err.Error())
			return nil
		}

		if cm.Data == nil {
			dms.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.FDBAddressNotConfiged), "configmap  "+"namespace"+msSpec.FDB.ConfigMapNamespaceName.Namespace+" name "+msSpec.FDB.ConfigMapNamespaceName.Name+" not have data.")
			return nil
		}

		if _, ok := cm.Data[fdbClusterFileKey]; !ok {
			dms.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.FDBAddressNotConfiged), "configmap  "+"namespace"+msSpec.FDB.ConfigMapNamespaceName.Namespace+" name "+msSpec.FDB.ConfigMapNamespaceName.Name+" not have cluster-file")
			return nil
		}
		fdbEndpoint = cm.Data[fdbClusterFileKey]
	}
	if msSpec.FDB.Address != "" {
		fdbEndpoint = msSpec.FDB.Address
	}

	return []corev1.EnvVar{{
		Name:  resource.FDB_ENDPOINT,
		Value: fdbEndpoint,
	}}
}
