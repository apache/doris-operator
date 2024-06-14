package fdb

import (
	"context"
	"errors"
	"github.com/FoundationDB/fdb-kubernetes-operator/api/v1beta2"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
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

var _ sub_controller.DisaggregatedSubController = &DisaggregatedFDBController{}

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
	ddc := obj.(*mv1.DorisDisaggregatedMetaService)
	if ddc.Spec.FDB == nil {
		klog.Errorf("disaggregatedFDBController disaggregatedMetaService namespace=%s name=%s have not fdb spec.!", ddc.Namespace, ddc.Name)
		fdbc.k8sRecorder.Event(ddc, "Failed", string(FDBSpecEmpty), "disaggregatedMetaService fdb spec not empty!")
		return errors.New("disaggregatedMetaService namespace=" + ddc.Namespace + " name=" + ddc.Name + "fdb spec empty!")
	}

	fdb := fdbc.buildFDBClusterResource(ddc)
	return k8s.ApplyFoundationDBCluster(ctx, fdbc.k8sClient, fdb)
}

// convert DorisDisaggregatedMetaSerivce's fdb to FoundationDBCluster resource.
func (fdbc *DisaggregatedFDBController) buildFDBClusterResource(ddc *mv1.DorisDisaggregatedMetaService) *v1beta2.FoundationDBCluster {
	fdb := &v1beta2.FoundationDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ddc.Namespace,
			Name:      ddc.GenerateFDBClusterName(),
			Labels:    ddc.GenerateFDBLabels(),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: ddc.APIVersion,
					Kind:       ddc.Kind,
					Name:       ddc.Name,
					UID:        ddc.UID,
				},
			},
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
				MatchLabels:          ddc.GenerateFDBLabels(),
				ProcessClassLabels:   []string{ProcessClassLabel},
				ProcessGroupIDLabels: []string{ProcessGroupIDLabel},
			},
			MinimumUptimeSecondsForBounce: 60,
			ProcessCounts: v1beta2.ProcessCounts{
				ClusterController: 1,
				Stateless:         -1,
			},

			Processes: map[v1beta2.ProcessClass]v1beta2.ProcessSettings{
				v1beta2.ProcessClassGeneral: v1beta2.ProcessSettings{
					PodTemplate:         fdbc.buildGeneralPodTemplate(ddc.Spec.FDB),
					VolumeClaimTemplate: ddc.Spec.FDB.VolumeClaimTemplate,
				},
			},

			Routing: v1beta2.RoutingConfig{
				DefineDNSLocalityFields: pointer.Bool(true),
				UseDNSInClusterFile:     pointer.Bool(true),
			},
		},
	}

	if ddc.Spec.FDB.Image == "" {
		return fdb
	}

	bi, v, err := imageSplit(ddc.Spec.FDB.Image)
	if err != nil {
		klog.Infof("disaggregatedFDBController split config image error, err=%s", err.Error())
		fdbc.k8sRecorder.Event(ddc, "Warning", string(ImageFormatError), ddc.Spec.FDB.Image+" format not provided, please reference docker definition.")
		return fdb

	}

	co := v1beta2.ContainerOverrides{
		ImageConfigs: []v1beta2.ImageConfig{
			v1beta2.ImageConfig{
				Version:   v,
				BaseImage: bi,
			},
		},
	}

	fdb.Spec.MainContainer = co
	fdb.Spec.SidecarContainer = co
	return fdb
}

func (fdbc *DisaggregatedFDBController) buildGeneralPodTemplate(ddc *mv1.FoundationDB) *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{fdbc.buildFDBContainer(ddc), fdbc.buildDefaultFDBSidecarContainer()},
			InitContainers: []corev1.Container{fdbc.buildDefaultFDBInitContainer()},
			NodeSelector:   ddc.NodeSelector,
			Affinity:       ddc.Affinity,
			Tolerations:    ddc.Tolerations,
		},
	}
}

// construct the fdb container for running fdb server.
func (fdbc *DisaggregatedFDBController) buildFDBContainer(ddc *mv1.FoundationDB) corev1.Container {
	return corev1.Container{
		Name:      v1beta2.MainContainerName,
		Resources: ddc.ResourceRequirements,
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
	if fdb.Status.Health.Healthy && fdb.Status.Health.Available {
		ddm.Status.FDBStatus.AvailableStatus = mv1.Available
	}

	return nil
}
