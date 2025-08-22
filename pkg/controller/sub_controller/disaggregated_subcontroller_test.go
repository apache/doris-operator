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
package sub_controller

import (
    v1 "github.com/apache/doris-operator/api/disaggregated/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    "testing"
)

func TestDisaggregatedSubDefaultController_BuildVolumesVolumeMountsAndPVCs_empty_persistentVolume(t *testing.T) {
    confMap := map[string]interface{}{}
    commonSpec :=v1.CommonSpec{}
    d := &DisaggregatedSubDefaultController{}
    fevs,fevms, fepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedFE, &commonSpec)
    bevs, bevms, bepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedBE, &commonSpec)
    msvs, msvms, mspvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedMS, &commonSpec)
    if len(fevs) != 2 || len(fevms) != 2 || len(fepvcs) != 0 {
        t.Errorf("build fe default volumes volumemounts and pvcs failed, the number is not right.")
    }
    if len(bevs) !=2 || len(bevms) !=2 || len(bepvcs) != 0 {
        t.Errorf("build be default volumes volumemounts and pvcs failed, the number is not right.")
    }
    if len(msvs) != 1 ||len(msvms) != 1 || len(mspvcs) != 0 {
        t.Errorf("build ms default volumes volumemounts and pvcs failed, the number is not right.")
    }
}

func TestDisaggregatedSubDefaultController_BuildVolumesVolumeMountsAndPVCs_persistentVolume(t *testing.T) {
    confMap := map[string]interface{}{
        "file_cache_path":"[{\"path\":\"/path/to/file_cache\",\"total_size\":21474836480},{\"path\":\"/path/to/file_cache2\",\"total_size\":21474836480}]",
    }
    commonSpec := v1.CommonSpec{
        PersistentVolume: &v1.PersistentVolume{
            PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
                AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
                Resources: corev1.VolumeResourceRequirements{
                    Requests: corev1.ResourceList{
                        "storage": resource.MustParse("200Gi"),
                    },
                },
            },
        },
    }

    beCommonSpec := commonSpec.DeepCopy()
    beCommonSpec.PersistentVolume.MountPaths = []string{"/path/to/file_cache12"}
    d := &DisaggregatedSubDefaultController{}
    fevs, fevms, fepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap,v1.DisaggregatedFE, &commonSpec)
    bevs, bevms, bepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedBE, beCommonSpec)
    msvs, msvms, mspvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedMS, &commonSpec)
    if len(fevs) != 2 || len(fevms) != 2 || len(fepvcs) != 2 {
        t.Errorf("build fe default volumes volumemounts and pvcs failed, the number is not right.")
    }
    if len(bevs) != 4 || len(bevms) != 4 || len(bepvcs) != 4 {
        t.Errorf("build be default volumes volumemounts and pvcs failed, the number is not right.")
    }
    if len(msvs) != 1 ||len(msvms) != 1 || len(mspvcs) != 1 {
        t.Errorf("build ms default volumes volumemounts and pvcs failed, the number is not right.")
    }
}

func TestDisaggregatedSubDefaultController_PersistentVolumeArrayBuildVolumesVolumeMountsAndPVCs(t *testing.T) {
    confMap := map[string]interface{}{
        "file_cache_path": "[{\"path\":\"/path/to/file_cache\",\"total_size\":21474836480},{\"path\":\"/path/to/file_cache2\",\"total_size\":21474836480}]",
    }

    commonSpec := v1.CommonSpec{
        PersistentVolumes: []v1.PersistentVolume{{
            PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
                AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
                Resources: corev1.VolumeResourceRequirements{
                    Requests: corev1.ResourceList{
                        "storage": resource.MustParse("200Gi"),
                    },
                },
            },
        }, {
            MountPaths: []string{"/path/to/file_cache"},
            PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
                AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
                Resources: corev1.VolumeResourceRequirements{
                    Requests: corev1.ResourceList{
                        "storage": resource.MustParse("500Gi"),
                    },
                },
            }},
        },
    }
    d := &DisaggregatedSubDefaultController{}
    fevs, fevms, fepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedFE, &commonSpec)
    bevs, bevms, bepvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedBE, &commonSpec)
    msvs, msvms, mspvcs := d.BuildVolumesVolumeMountsAndPVCs(confMap, v1.DisaggregatedMS, &commonSpec)
    if len(fevs) != 3 || len(fevms) != 3 || len(fepvcs) != 3 {
        t.Errorf("build fe default volumes volumemounts and pvcs failed, the number is not right.")
    }
    if len(bevs) !=3 || len(bevms) !=3 || len(bepvcs) != 3 {
        t.Errorf("build be default volumes volumemounts and pvcs failed, the number is not right.")
    }
    if len(msvs) != 2 ||len(msvms) != 2 || len(mspvcs) != 2 {
        t.Errorf("build ms default volumes volumemounts and pvcs failed, the number is not right.")
    }
}
