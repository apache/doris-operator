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
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	sub "github.com/apache/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kr "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"os"
	"strconv"
)

const (
	DefaultCacheRootPath = "/opt/apache-doris/be/file_cache"
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

// generate statefulset or service labels
func (dcgs *DisaggregatedComputeGroupsController) newCG2LayerSchedulerLabels(ddcName /*DisaggregatedClusterName*/, uniqueId string) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName:          ddcName,
		dv1.DorisDisaggregatedComputeGroupUniqueId: uniqueId,
		dv1.DorisDisaggregatedOwnerReference:       ddcName,
	}
}

func (dcgs *DisaggregatedComputeGroupsController) newCGPodsSelector(ddcName /*DisaggregatedClusterName*/, uniqueId string) map[string]string {
	return map[string]string{
		dv1.DorisDisaggregatedClusterName:          ddcName,
		dv1.DorisDisaggregatedComputeGroupUniqueId: uniqueId,
		dv1.DorisDisaggregatedPodType:              "compute",
	}
}

func (dcgs *DisaggregatedComputeGroupsController) NewStatefulset(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup, cvs map[string]interface{}) *appv1.StatefulSet {
	st := dcgs.NewDefaultStatefulset(ddc)
	uniqueId := cg.UniqueId
	matchLabels := dcgs.newCGPodsSelector(ddc.Name, uniqueId)

	//build metadata
	func() {
		st.Name = ddc.GetCGStatefulsetName(cg)
		st.Labels = dcgs.newCG2LayerSchedulerLabels(ddc.Name, uniqueId)
	}()

	// build st.spec
	func() {
		st.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
		_, _, vcts := dcgs.buildVolumesVolumeMountsAndPVCs(cvs, cg)
		st.Spec.Replicas = cg.Replicas
		st.Spec.VolumeClaimTemplates = vcts
		st.Spec.ServiceName = ddc.GetCGServiceName(cg)
		pts := dcgs.NewPodTemplateSpec(ddc, matchLabels, cvs, cg)
		st.Spec.Template = pts
	}()

	return st
}

func (dcgs *DisaggregatedComputeGroupsController) NewPodTemplateSpec(ddc *dv1.DorisDisaggregatedCluster, selector map[string]string, cvs map[string]interface{}, cg *dv1.ComputeGroup) corev1.PodTemplateSpec {
	pts := resource.NewPodTemplateSpecWithCommonSpec(&cg.CommonSpec, dv1.DisaggregatedBE)
	//pod template metadata.
	func() {
		l := (resource.Labels)(selector)
		l.AddLabel(pts.Labels)
		pts.Labels = l
	}()

	c := dcgs.NewCGContainer(ddc, cvs, cg)
	pts.Spec.Containers = append(pts.Spec.Containers, c)

	vs, _, _ := dcgs.buildVolumesVolumeMountsAndPVCs(cvs, cg)
	configVolumes, _ := dcgs.BuildDefaultConfigMapVolumesVolumeMounts(cg.ConfigMaps)
	pts.Spec.Volumes = append(pts.Spec.Volumes, configVolumes...)
	pts.Spec.Volumes = append(pts.Spec.Volumes, vs...)

	cgUniqueId := selector[dv1.DorisDisaggregatedComputeGroupUniqueId]
	pts.Spec.Affinity = dcgs.ConstructDefaultAffinity(dv1.DorisDisaggregatedComputeGroupUniqueId, cgUniqueId, pts.Spec.Affinity)

	return pts
}

func (dcgs *DisaggregatedComputeGroupsController) NewCGContainer(ddc *dv1.DorisDisaggregatedCluster, cvs map[string]interface{}, cg *dv1.ComputeGroup) corev1.Container {
	c := resource.NewContainerWithCommonSpec(&cg.CommonSpec)
	resource.LifeCycleWithPreStopScript(c.Lifecycle, sub.GetDisaggregatedPreStopScript(dv1.DisaggregatedBE))
	cmd, args := sub.GetDisaggregatedCommand(dv1.DisaggregatedBE)
	c.Command = cmd
	c.Args = args
	c.Name = "compute"

	c.Ports = resource.GetDisaggregatedContainerPorts(cvs, dv1.DisaggregatedBE)
	c.Env = cg.CommonSpec.EnvVars
	c.Env = append(c.Env, resource.GetPodDefaultEnv()...)
	c.Env = append(c.Env, dcgs.newSpecificEnvs(ddc, cg)...)

	resource.BuildDisaggregatedProbe(&c, &cg.CommonSpec, dv1.DisaggregatedBE)
	_, vms, _ := dcgs.buildVolumesVolumeMountsAndPVCs(cvs, cg)
	_, cmvms := dcgs.BuildDefaultConfigMapVolumesVolumeMounts(cg.ConfigMaps)
	c.VolumeMounts = vms
	if c.VolumeMounts == nil {
		c.VolumeMounts = cmvms
	} else {
		c.VolumeMounts = append(c.VolumeMounts, cmvms...)
	}
	return c
}

func (dcgs *DisaggregatedComputeGroupsController) buildVolumesVolumeMountsAndPVCs(cvs map[string]interface{}, cg *dv1.ComputeGroup) ([]corev1.Volume, []corev1.VolumeMount, []corev1.PersistentVolumeClaim) {
	if cg.CommonSpec.PersistentVolume == nil {
		vs, vms := dcgs.getDefaultVolumesVolumeMounts(cvs)
		return vs, vms, nil
	}

	var vs []corev1.Volume
	var vms []corev1.VolumeMount
	var pvcs []corev1.PersistentVolumeClaim

	paths, maxSize := dcgs.getCacheMaxSizeAndPaths(cvs)

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
			logPath := dcgs.getLogPath(cvs)
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
				Spec: *cg.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec.DeepCopy(),
			}
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
			Spec: *cg.CommonSpec.PersistentVolume.PersistentVolumeClaimSpec.DeepCopy(),
		})
	}

	return vs, vms, pvcs
}

// when not config persisentTemplateSpec, pod should mount emptyDir volume for storing data and log. mountPath resolve from config file.
func (dcgs *DisaggregatedComputeGroupsController) getDefaultVolumesVolumeMounts(cvs map[string]interface{}) ([]corev1.Volume, []corev1.VolumeMount) {
	LogPath := dcgs.getLogPath(cvs)
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

	storagePaths, _ := dcgs.getCacheMaxSizeAndPaths(cvs)
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

func (dcgs *DisaggregatedComputeGroupsController) getCacheMaxSizeAndPaths(cvs map[string]interface{}) ([]string, int64) {
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
		klog.Errorf("disaggregatedComputeGroupsController getStorageMaxSizeAndPaths json unmarshal file_cache_path failed, err=%s", err.Error())
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

func (dcgs *DisaggregatedComputeGroupsController) getLogPath(cvs map[string]interface{}) string {
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

// add specific envs for be, the env will used by be_disaggregated_entrypoint script.
func (dcgs *DisaggregatedComputeGroupsController) newSpecificEnvs(ddc *dv1.DorisDisaggregatedCluster, cg *dv1.ComputeGroup) []corev1.EnvVar {
	var cgEnvs []corev1.EnvVar
	stsName := ddc.GetCGStatefulsetName(cg)

	//get fe config for find query port
	confMap := dcgs.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.FE_RESOLVEKEY, ddc.Spec.FeSpec.ConfigMaps)
	fqp := resource.GetPort(confMap, resource.QUERY_PORT)
	fqpStr := strconv.FormatInt(int64(fqp), 10)
	//use fe service name as access address.
	feAddr := ddc.GetFEServiceNameForAccess()
	cgEnvs = append(cgEnvs,
		corev1.EnvVar{Name: resource.STATEFULSET_NAME, Value: stsName},
		corev1.EnvVar{Name: resource.COMPUTE_GROUP_NAME, Value: ddc.GetCGName(cg)},
		corev1.EnvVar{Name: resource.ENV_FE_ADDR, Value: feAddr},
		corev1.EnvVar{Name: resource.ENV_FE_PORT, Value: fqpStr})
	return cgEnvs
}
