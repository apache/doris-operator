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

package computegroups

import (
	"encoding/json"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	sub "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kr "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"os"
	"strconv"
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
	DefaultCacheRootPath = "/opt/apache-doris/be/storage"
	//default cache storage size: unit=B
	DefaultCacheSize               int64 = 107374182400
	FileCachePathKey                     = "file_cache_path"
	FileCacheSubConfigPathKey            = "path"
	FileCacheSubConfigTotalSizeKey       = "total_size"
	DefaultLogPath                       = "/opt/apache-doris/be/log"
	LogPathKey                           = "LOG_DIR"
	LogStoreName                         = "be-log"
	StorageStorePreName                  = "be-storage"
)

const (
	BE_PROBE_COMMAND = "/opt/apache-doris/be_disaggregated_probe.sh"
)

// generate statefulset or service labels
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
		st.Spec.PodManagementPolicy = appv1.ParallelPodManagement
		st.Spec.ServiceName = ddc.GetCGServiceName(cg)
		pts := dccs.NewPodTemplateSpec(ddc, matchLabels, cvs, cg)
		st.Spec.Template = pts
	}()

	return st
}

func (dccs *DisaggregatedComputeGroupsController) NewPodTemplateSpec(ddc *dv1.DorisDisaggregatedCluster, selector map[string]string, cvs map[string]interface{}, cg *dv1.ComputeGroup) corev1.PodTemplateSpec {
	pts := resource.NewPodTemplateSpecWithCommonSpec(&cg.CommonSpec, dv1.DisaggregatedBE)
	//pod template metadata.
	func() {
		l := (resource.Labels)(selector)
		l.AddLabel(pts.Labels)
		pts.Labels = l
	}()

	c := dccs.NewCGContainer(ddc, cvs, cg)
	pts.Spec.Containers = append(pts.Spec.Containers, c)

	vs, _, _ := dccs.buildVolumesVolumeMountsAndPVCs(cvs, cg)
	configVolumes, _ := dccs.buildConfigMapVolumesVolumeMounts(cg)
	pts.Spec.Volumes = append(pts.Spec.Volumes, configVolumes...)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)

	cgClusterId := selector[dv1.DorisDisaggregatedComputeGroupClusterId]
	defAffinity := dccs.newCGDefaultAffinity(dv1.DorisDisaggregatedComputeGroupClusterId, cgClusterId)
	if pts.Spec.Affinity == nil {
		pts.Spec.Affinity = defAffinity
		return pts
	}

	if pts.Spec.Affinity.PodAntiAffinity == nil {
		pts.Spec.Affinity.PodAntiAffinity = defAffinity.PodAntiAffinity
	} else {
		pts.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(pts.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			defAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
	}

	return pts
}

func (dccs *DisaggregatedComputeGroupsController) newCGDefaultAffinity(matchKey, value string) *corev1.Affinity {
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
	_, cmvms := dccs.buildConfigMapVolumesVolumeMounts(cg)
	c.VolumeMounts = vms
	if c.VolumeMounts == nil {
		c.VolumeMounts = cmvms
	} else {
		c.VolumeMounts = append(c.VolumeMounts, cmvms...)
	}
	return c
}

func (dccs *DisaggregatedComputeGroupsController) buildConfigMapVolumesVolumeMounts(cg *dv1.ComputeGroup) ([]corev1.Volume, []corev1.VolumeMount) {
	var vs []corev1.Volume
	var vms []corev1.VolumeMount
	for _, cm := range cg.ConfigMaps {
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

func (dccs *DisaggregatedComputeGroupsController) buildVolumesVolumeMountsAndPVCs(cvs map[string]interface{}, cg *dv1.ComputeGroup) ([]corev1.Volume, []corev1.VolumeMount, []corev1.PersistentVolumeClaim) {
	if cg.CommonSpec.PersistentVolume == nil {
		vs, vms := dccs.getDefaultVolumesVolumeMounts(cvs)
		return vs, vms, nil
	}

	var vs []corev1.Volume
	var vms []corev1.VolumeMount
	var pvcs []corev1.PersistentVolumeClaim

	paths, maxSize := dccs.getCacheMaxSizeAndPaths(cvs)

	//fill defect fields of pvcSpec.
	func() {
		if maxSize > 0 {
			cs := kr.NewQuantity(maxSize, kr.BinarySI)
			if cg.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests == nil {
				cg.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests = map[corev1.ResourceName]kr.Quantity{}
			}
			pvcSize := cg.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage]
			cmp := cs.Cmp(pvcSize)
			if cmp > 0 {
				cg.PersistentVolume.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage] = *cs
			}
		}
		if len(cg.PersistentVolume.PersistentVolumeClaimSpec.AccessModes) == 0 {
			cg.PersistentVolume.PersistentVolumeClaimSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		}
	}()

	//generate log volume, volumeMount, pvc
	func() {
		if !cg.CommonSpec.PersistentVolume.LogNotStore {
			logPath := dccs.getLogPath(cvs)
			vs = append(vs, corev1.Volume{Name: LogStoreName, VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: LogStoreName,
				}}})
			vms = append(vms, corev1.VolumeMount{Name: LogStoreName, MountPath: logPath})
			logPvc := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:        LogStoreName,
					Annotations: cg.CommonSpec.PersistentVolume.Annotations,
				},
				Spec: cg.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec,
			}
			logPvc.Spec.Resources.Requests[corev1.ResourceStorage] = kr.MustParse("200Gi")
			pvcs = append(pvcs, logPvc)
		}
	}()

	//merge mountPaths
	for _, p := range cg.PersistentVolume.MountPaths {
		plen := len(paths)
		for ; plen > 0; plen-- {
			if paths[plen-1] == p {
				break
			}
		}

		if plen <= 0 {
			paths = append(paths, p)
		}
	}

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

	storagePaths, _ := dccs.getCacheMaxSizeAndPaths(cvs)
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

func (dccs *DisaggregatedComputeGroupsController) getCacheMaxSizeAndPaths(cvs map[string]interface{}) ([]string, int64) {
	v := cvs[FileCachePathKey]
	if v == nil {
		return []string{DefaultCacheRootPath}, DefaultCacheSize
	}

	var paths []string
	var maxCacheSize int64
	vbys := v.(string)
	var pa []map[string]interface{}
	err := json.Unmarshal([]byte(vbys), &pa)
	if err != nil {
		klog.Errorf("disaggregatedComputeGroupsController getStorageMaxSizeAndPaths json unmarshal file_cache_paht failed, err=%s", err.Error())
		return []string{}, 0
	}

	for i, mp := range pa {
		pv := mp[FileCacheSubConfigPathKey]
		pv_str, ok := pv.(string)
		if !ok {
			klog.Errorf("disaggregatedComputeGroupsController getStorageMaxSizeAndPaths index %d have not path config.", i)
			continue
		}
		paths = append(paths, pv_str)
		cache_v := mp[FileCacheSubConfigTotalSizeKey]
		fc_size, ok := cache_v.(float64)
		cache_size := int64(fc_size)
		if !ok {
			klog.Errorf("disaggregatedComputeGroupsController getStorageMaxSizeAndPaths index %d total_size is not number.", i)
			continue
		}
		if maxCacheSize < cache_size {
			maxCacheSize = cache_size
		}
	}
	return paths, maxCacheSize
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
	//resolve relative path to absolute path
	path := os.Expand(v.(string), mapping)
	return path
}

func (dccs *DisaggregatedComputeGroupsController) newCGLivenessProbe(cvs /*config values*/ map[string]interface{}) *corev1.Probe {
	heartBeatPort := resource.GetPort(cvs, resource.HEARTBEAT_SERVICE_PORT)
	commands := []string{BE_PROBE_COMMAND, "alive"}
	return resource.LivenessProbe(heartBeatPort, "", commands, resource.Exec)
}

func (dccs *DisaggregatedComputeGroupsController) newCGStartUpProbe(cvs /*config values*/ map[string]interface{}) *corev1.Probe {
	return dccs.newCGLivenessProbe(cvs)
}

func (dccs *DisaggregatedComputeGroupsController) newCGReadinessProbe(cvs /*config values*/ map[string]interface{}) *corev1.Probe {
	webserverPort := resource.GetPort(cvs, resource.WEBSERVER_PORT)
	commands := []string{BE_PROBE_COMMAND, "ready"}
	return resource.ReadinessProbe(webserverPort, "", commands, resource.Exec)
}

func (dccs *DisaggregatedComputeGroupsController) newSpecificEnvs(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) []corev1.EnvVar {
	var cgEnvs []corev1.EnvVar
	stsName := ddc.GetCGStatefulsetName(cg)
	clusterId := ddc.GetCGClusterId(cg)
	cloudUniqueIdPre := ddc.GetCGCloudUniqueIdPre()

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
