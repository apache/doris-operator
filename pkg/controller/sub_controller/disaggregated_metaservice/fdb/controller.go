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

package fdb

import (
	"context"
	"errors"
	"github.com/FoundationDB/fdb-kubernetes-operator/api/v1beta2"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	sc "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ sc.DisaggregatedSubController = &DisaggregatedFDBController{}

var (
	disaggregatedFDBController = "disaggregatedFDBController"
)

type DisaggregatedFDBController struct {
	k8sClient      client.Client
	k8sRecorder    record.EventRecorder
	controllerName string
}

func New(mgr ctrl.Manager) *DisaggregatedFDBController {
	return &DisaggregatedFDBController{
		k8sClient:      mgr.GetClient(),
		k8sRecorder:    mgr.GetEventRecorderFor(disaggregatedFDBController),
		controllerName: disaggregatedFDBController,
	}
}

// sync FoundationDBCluster
func (fdbc *DisaggregatedFDBController) Sync(ctx context.Context, obj client.Object) error {
	ddm := obj.(*mv1.DorisDisaggregatedMetaService)
	if ddm.Spec.FDB == nil {
		klog.Errorf("disaggregatedFDBController disaggregatedMetaService namespace=%s name=%s have not fdb spec.!", ddm.Namespace, ddm.Name)
		fdbc.k8sRecorder.Event(ddm, "Failed", string(sc.FDBSpecEmpty), "disaggregatedMetaService fdb spec not empty!")
		return errors.New("disaggregatedMetaService namespace=" + ddm.Namespace + " name=" + ddm.Name + "fdb spec empty!")
	}

	fdb := fdbc.buildFDBClusterResource(ddm)
	return k8s.ApplyFoundationDBCluster(ctx, fdbc.k8sClient, fdb)
}

// convert DorisDisaggregatedMetaSerivce's fdb to FoundationDBCluster resource.
func (fdbc *DisaggregatedFDBController) buildFDBClusterResource(ddm *mv1.DorisDisaggregatedMetaService) *v1beta2.FoundationDBCluster {
	fdb := &v1beta2.FoundationDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  ddm.Namespace,
			Name:       ddm.GenerateFDBClusterName(),
			Labels:     ddm.GenerateFDBLabels(),
			Finalizers: []string{ddm.Name},
			//delete ownerReference to prevent mistake delete ddm.
			//OwnerReferences: []metav1.OwnerReference{
			//	{
			//		APIVersion: ddms.APIVersion,
			//		Kind:       ddms.Kind,
			//		Name:       ddms.Name,
			//		UID:        ddms.UID,
			//	},
			//},
		},

		Spec: v1beta2.FoundationDBClusterSpec{
			Version: FoundationVersion,
			AutomationOptions: v1beta2.FoundationDBClusterAutomationOptions{
				DeletionMode:      v1beta2.PodUpdateModeZone,
				PodUpdateStrategy: v1beta2.PodUpdateStrategyTransactionReplacement,
				RemovalMode:       v1beta2.PodUpdateModeZone,
				Replacements: v1beta2.AutomaticReplacementOptions{
					Enabled:                   pointer.Bool(true),
					MaxConcurrentReplacements: pointer.Int(1),
				},
			},
			LabelConfig: v1beta2.LabelConfig{
				MatchLabels:             ddm.GenerateFDBLabels(),
				ProcessClassLabels:      []string{ProcessClassLabel},
				ProcessGroupIDLabels:    []string{ProcessGroupIDLabel},
				FilterOnOwnerReferences: pointer.Bool(false),
			},
			MinimumUptimeSecondsForBounce: 60,
			ProcessCounts: v1beta2.ProcessCounts{
				ClusterController: 1,
				Stateless:         -1,
				Log:               2,
				Storage:           2,
			},

			Processes: map[v1beta2.ProcessClass]v1beta2.ProcessSettings{
				v1beta2.ProcessClassGeneral: v1beta2.ProcessSettings{
					PodTemplate:         fdbc.buildGeneralPodTemplate(ddm.Spec.FDB),
					VolumeClaimTemplate: ddm.Spec.FDB.VolumeClaimTemplate,
				},
			},

			Routing: v1beta2.RoutingConfig{
				UseDNSInClusterFile: pointer.Bool(true),
			},
			SidecarContainer: v1beta2.ContainerOverrides{
				EnableLivenessProbe:  pointer.Bool(true),
				EnableReadinessProbe: pointer.Bool(false)},

			Skip:                                false,
			UseExplicitListenAddress:            pointer.Bool(true),
			ReplaceInstancesWhenResourcesChange: pointer.Bool(true),
		},
	}

	mainContainer, err := fdbImageOverride(ddm.Spec.FDB.Image)
	if err != nil {
		klog.Infof("disaggregatedFDBController split config Image error, err=%s", err.Error())
		fdbc.k8sRecorder.Event(ddm, "Warning", string(sc.ImageFormatError), ddm.Spec.FDB.Image+" format not provided, please reference docker definition.")
		return fdb
	}
	fdb.Spec.MainContainer = mainContainer

	sidecarContainer, err := fdbSidecarImageOverride(ddm.Spec.FDB.SidecarImage)
	if err != nil {
		klog.Infof("disaggregatedFDBController split config SidecarImage error, err=%s", err.Error())
		fdbc.k8sRecorder.Event(ddm, "Warning", string(sc.ImageFormatError), ddm.Spec.FDB.SidecarImage+" format not provided, please reference docker definition.")
		return fdb
	}
	fdb.Spec.SidecarContainer = sidecarContainer
	return fdb
}

func (fdbc *DisaggregatedFDBController) buildGeneralPodTemplate(fdb *mv1.FoundationDB) *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{fdbc.buildFDBContainer(fdb), fdbc.buildDefaultFDBSidecarContainer()},
			InitContainers: []corev1.Container{fdbc.buildDefaultFDBInitContainer()},
			NodeSelector:   fdb.NodeSelector,
			Affinity:       fdb.Affinity,
			Tolerations:    fdb.Tolerations,
		},
	}
}

// construct the fdb container for running fdb server.
func (fdbc *DisaggregatedFDBController) buildFDBContainer(fdb *mv1.FoundationDB) corev1.Container {
	return corev1.Container{
		Name:      v1beta2.MainContainerName,
		Resources: fdb.ResourceRequirements,
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: pointer.Int64(0),
		},
	}
}

// construct the init container for initialing environment of fdb.
func (fdbc *DisaggregatedFDBController) buildDefaultFDBInitContainer() corev1.Container {
	return corev1.Container{
		Name:      v1beta2.InitContainerName,
		Resources: getDefaultResources(),
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: pointer.Int64(0),
		},
	}
}

// construct the sidecar container for
func (fdbc *DisaggregatedFDBController) buildDefaultFDBSidecarContainer() corev1.Container {
	return corev1.Container{
		Name:      v1beta2.SidecarContainerName,
		Resources: getDefaultResources(),
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: pointer.Int64(0),
		},
	}
}

func (fdbc *DisaggregatedFDBController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	ddm := obj.(*mv1.DorisDisaggregatedMetaService)

	if ddm.DeletionTimestamp.IsZero() {
		return true, nil
	}

	fdbClusterName := ddm.GenerateFDBClusterName()
	if err := k8s.DeleteFoundationDBCluster(ctx, fdbc.k8sClient, ddm.Namespace, ddm.Name); err != nil {
		klog.Errorf("disaggregatedFDBController delete foundationDBCluster name %s failed,err=%s.", fdbClusterName, err.Error())
		return false, err
	}
	return true, nil
}

func (fdbc *DisaggregatedFDBController) GetControllerName() string {
	return fdbc.controllerName
}

func (fdbc *DisaggregatedFDBController) UpdateComponentStatus(obj client.Object) error {
	ddm := obj.(*mv1.DorisDisaggregatedMetaService)
	fdbClusterName := ddm.GenerateFDBClusterName()
	var fdb v1beta2.FoundationDBCluster
	if err := fdbc.k8sClient.Get(context.Background(), types.NamespacedName{Name: fdbClusterName, Namespace: ddm.Namespace}, &fdb); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("disaggregatedFDBController foundationDBCluster name =%s not found.", fdbClusterName)
			return nil
		}

		klog.Errorf("disaggregatedFDBController foundationDBCluster name=%s get failed, err=%s", fdbClusterName, err.Error())
		return err
	}

	ddm.Status.FDBStatus.FDBResourceName = fdbClusterName
	ddm.Status.FDBStatus.FDBAddress = fdb.Status.ConnectionString
	ddm.Status.FDBStatus.AvailableStatus = mv1.UnAvailable
	//use fdbcluster's Healthy and available for checking fdb normal or not normal.
	//Healthy  reports whether the database is in a fully healthy state.
	//Available reports whether the database is accepting reads and writes.
	if fdb.Status.Health.Available {
		if fdb.Status.Health.Healthy == false {
			fdbc.k8sRecorder.Event(ddm, string(sc.EventNormal), string(sc.FDBAvailableButUnhealth), "disaggregatedMetaService fdb status is not Healthy, but Available!")
		}
		ddm.Status.FDBStatus.AvailableStatus = mv1.Available
	}

	return nil
}
