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
	"strings"

	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/doris"
	"github.com/apache/doris-operator/pkg/common/utils/hash"
	"github.com/apache/doris-operator/pkg/common/utils/set"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var (
	pvc_finalizer          = "selectdb.doris.com/pvc-finalizer"
	pvc_manager_annotation = "selectdb.doris.com/pvc-manager"
)

const (
	pvcFinalizerApache         = "apache.doris.org/pvc-finalizer"
	PVCManagerAnnotationApache = "apache.doris.org/pvc-manager"
)

func BuildPVCName(stsName, ordinal, volumeName string) string {
	pvcName := stsName + "-" + ordinal
	if volumeName != "" {
		pvcName = volumeName + "-" + pvcName
	}
	return pvcName
}

func BuildPVC(volume dorisv1.PersistentVolume, labels map[string]string, namespace, stsName, ordinal string) corev1.PersistentVolumeClaim {
	annotations := buildPVCAnnotations(volume)

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        BuildPVCName(stsName, ordinal, volume.Name),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
			Finalizers:  []string{pvc_finalizer},
		},
		Spec: volume.PersistentVolumeClaimSpec,
	}
	return pvc
}

func BuildDisaggregatedPVC(
	pvcTemplate corev1.PersistentVolumeClaim,
	labels map[string]string,
	namespace, stsName, ordinal string) corev1.PersistentVolumeClaim {

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        BuildPVCName(stsName, ordinal, pvcTemplate.Name),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: pvcTemplate.Annotations,
			Finalizers:  []string{pvcFinalizerApache},
		},
		Spec: pvcTemplate.Spec,
	}
	return pvc
}

// finalAnnotations is a combination of user annotations and operator default annotations
func buildPVCAnnotations(volume dorisv1.PersistentVolume) Annotations {
	annotations := Annotations{}
	if volume.PVCProvisioner == dorisv1.PVCProvisionerOperator {
		annotations.Add(pvc_manager_annotation, "operator")
		annotations.Add(dorisv1.ComponentResourceHash, hash.HashObject(volume.PersistentVolumeClaimSpec))
	}

	if volume.Annotations != nil && len(volume.Annotations) > 0 {
		annotations.AddAnnotation(volume.Annotations)
	}
	return annotations
}

// BuildDisaggregatedPVCAnnotations finalAnnotations is a combination of user annotations and operator default annotations
func BuildDisaggregatedPVCAnnotations(volume dv1.PersistentVolume) Annotations {
	annotations := Annotations{}
	if volume.PVCProvisioner == dv1.PVCProvisionerOperator {
		annotations.Add(PVCManagerAnnotationApache, "operator")
		annotations.Add(dorisv1.ComponentResourceHash, hash.HashObject(volume.PersistentVolumeClaimSpec))
	}

	if volume.Annotations != nil && len(volume.Annotations) > 0 {
		annotations.AddAnnotation(volume.Annotations)
	}
	return annotations
}

func getDefaultDorisHome(componentType dorisv1.ComponentType) string {
	switch componentType {
	case dorisv1.Component_FE:
		return DEFAULT_ROOT_PATH + "/fe"
	case dorisv1.Component_BE, dorisv1.Component_CN:
		return DEFAULT_ROOT_PATH + "/be"
	case dorisv1.Component_Broker:
		return DEFAULT_ROOT_PATH + "/apache_hdfs_broker"
	default:
		klog.Infof("the componentType: %s have not default DORIS_HOME", componentType)
	}
	return ""
}

// GenerateEveryoneMountPathDorisPersistentVolume is used to process the pvc template configuration in CRD.
// The template is defined as follows:
// - PersistentVolume.MountPath is "", it`s template configuration.
// - PersistentVolume.MountPath is not "", it`s actual pvc configuration.
// The Explain rules are as follows:
// 1. Non-templated PersistentVolumes are returned directly in the result list.
// 2. If there is a pvc template, return the actual list of pvcs after processing.
// 3. The template needs to parse the configuration of the doris config file to create the pvc.
// 4. If there are multiple templates, the last valid template will be used.
func GenerateEveryoneMountPathDorisPersistentVolume(spec *dorisv1.BaseSpec, excludePaths []string, config map[string]interface{}, componentType dorisv1.ComponentType) ([]dorisv1.PersistentVolume, error) {
	// Only the last data pvc template configuration takes effect
	var template *dorisv1.PersistentVolume
	// dorisPersistentVolumes is the pvc that needs to be actually created, specified by the user
	var dorisPersistentVolumes []dorisv1.PersistentVolume
	for i := range spec.PersistentVolumes {
		if spec.PersistentVolumes[i].MountPath != "" {
			path := spec.PersistentVolumes[i].MountPath
			if strings.HasSuffix(path, "/") {
				path = path[:len(path)-1]
			}
			if !set.ArrayContains(excludePaths, path) {
				dorisPersistentVolumes = append(dorisPersistentVolumes, spec.PersistentVolumes[i])
			} else {
				klog.Errorf("GenerateEveryoneMountPathDorisPersistentVolume SharedPersistentVolumeClaim.MountPath (%s) conflicts with the MountPath configured in BaseSpec.PersistentVolumes, "+
					"and the SharedPersistentVolumeClaims configuration takes precedence, skipping the processing of the BaseSpec.PersistentVolumes for the PVC. "+
					"If it does not meet expectations, please handle the conflict and rebuild the cluster.", path)
			}
		} else {
			template = (&spec.PersistentVolumes[i]).DeepCopy()
		}
	}

	if template == nil {
		return dorisPersistentVolumes, nil
	}

	// Processing pvc template
	var dataPathKey, dataDefaultPath string
	var dataPaths []string
	dorisHome := getDefaultDorisHome(componentType)
	switch componentType {
	case dorisv1.Component_FE:
		dataPathKey = "meta_dir"
		dataDefaultPath = dorisHome + "/doris-meta"
	case dorisv1.Component_BE, dorisv1.Component_CN:
		dataPathKey = "storage_root_path"
		dataDefaultPath = dorisHome + "/storage"
	default:
		klog.Infof("GenerateEveryoneMountPathDorisPersistentVolume the componentType: %s is not supported, PersistentVolume template will not work ", componentType)
		return dorisPersistentVolumes, nil
	}

	dataPathValue, dataExist := config[dataPathKey]
	if !dataExist {
		klog.Infof("GenerateEveryoneMountPathDorisPersistentVolume: dataPathKey '%s' not found in config, default value will effect", dataPathKey)
		dataPaths = append(dataPaths, dataDefaultPath)
	} else {
		dataPaths = doris.ResolveStorageRootPath(dataPathValue.(string))
	}

	if len(dataPaths) == 1 {
		tmp := *template.DeepCopy()
		tmp.MountPath = dataPaths[0]
		if !set.ArrayContains(excludePaths, dataPaths[0]) {
			dorisPersistentVolumes = append(dorisPersistentVolumes, tmp)
		} else {
			klog.Errorf("GenerateEveryoneMountPathDorisPersistentVolume SharedPersistentVolumeClaims.MountPath (%s) conflicts with the MountPath configured in BaseSpec.PersistentVolumes, "+
				"and the SharedPersistentVolumeClaims configuration takes precedence, skipping the processing of the BaseSpec.PersistentVolumes for the PVC. "+
				"If it does not meet expectations, please handle the conflict and rebuild the cluster.", dataPaths[0])
		}
	} else {
		pathNames := doris.GetNameOfEachPath(dataPaths)
		for i := range dataPaths {
			tmp := *template.DeepCopy()
			tmp.Name = tmp.Name + "-" + pathNames[i]
			tmp.MountPath = dataPaths[i]
			if !set.ArrayContains(excludePaths, dataPaths[i]) {
				dorisPersistentVolumes = append(dorisPersistentVolumes, tmp)
			} else {
				klog.Errorf("GenerateEveryoneMountPathDorisPersistentVolume SharedPersistentVolumeClaims.MountPath (%s) conflicts with the MountPath configured in BaseSpec.PersistentVolumes, "+
					"and the SharedPersistentVolumeClaims configuration takes precedence, skipping the processing of the BaseSpec.PersistentVolumes for the PVC. "+
					"If it does not meet expectations, please handle the conflict and rebuild the cluster.", dataPaths[i])
			}
		}
	}

	return dorisPersistentVolumes, nil
}
