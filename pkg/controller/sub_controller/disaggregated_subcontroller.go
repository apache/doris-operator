package sub_controller

import (
	"context"
	"fmt"
	mv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

type DisaggregatedSubController interface {
	//Sync reconcile for sub controller. bool represent the component have updated.
	Sync(ctx context.Context, obj client.Object) error
	//clear all resource about sub-component.
	ClearResources(ctx context.Context, obj client.Object) (bool, error)

	//return the controller name, beController, feController,cnController for log.
	GetControllerName() string

	//UpdateStatus update the component status on src.
	UpdateComponentStatus(obj client.Object) error
}

type DisaggregatedSubDefaultController struct {
	K8sclient      client.Client
	K8srecorder    record.EventRecorder
	ControllerName string
}

func (d *DisaggregatedSubDefaultController) GetMSConfig(ctx context.Context, cms []mv1.ConfigMap, namespace string, componentType mv1.ComponentType) (map[string]interface{}, error) {
	if len(cms) == 0 {
		return make(map[string]interface{}), nil
	}
	configMaps, err := k8s.GetDisaggregatedConfigMaps(ctx, d.K8sclient, namespace, cms)
	if err != nil {
		klog.Errorf("DisaggregatedSubDefaultController GetConfig get configmap failed, namespace: %s,err: %s \n", namespace, err.Error())
	}
	res, resolveErr := resource.ResolveDMSConfigMaps(configMaps, componentType)
	return res, utils.MergeError(err, resolveErr)
}

// generate map for mountpath:configmap
func (d *DisaggregatedSubDefaultController) CheckMSConfigMountPath(dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) {
	cms := resource.GetDMSBaseSpecFromCluster(dms, componentType).ConfigMaps

	var mountsMap = make(map[string]mv1.ConfigMap)
	for _, cm := range cms {
		path := cm.MountPath
		if m, exist := mountsMap[path]; exist {
			klog.Errorf("CheckMSConfigMountPath error: the mountPath %s is repeated between configmap: %s and configmap: %s.", path, cm.Name, m.Name)
			d.K8srecorder.Event(dms, EventWarning, ConfigMapPathRepeated, fmt.Sprintf("the mountPath %s is repeated between configmap: %s and configmap: %s.", path, cm.Name, m.Name))
		}
		mountsMap[path] = cm
	}
}

// RestrictConditionsEqual adds two StatefulSet,
// It is used to control the conditions for comparing.
// nst StatefulSet - a new StatefulSet
// est StatefulSet - an old StatefulSet
func (d *DisaggregatedSubDefaultController) RestrictConditionsEqual(nst *appv1.StatefulSet, est *appv1.StatefulSet) {
	//shield persistent volume update when the pvcProvider=Operator
	//in webhook should intercept the volume spec updated when use statefulset pvc.
	// TODO: updates to statefulset spec for fields other than 'replicas', 'template', 'updateStrategy', 'persistentVolumeClaimRetentionPolicy' and 'minReadySeconds' are forbidden
	nst.Spec.VolumeClaimTemplates = est.Spec.VolumeClaimTemplates
}

// PrepareMSReconcileResources prepare resource for reconcile
// response: bool, if true presents resource have ready for reconciling, if false presents resource is preparing.
func (d *DisaggregatedSubDefaultController) PrepareMSReconcileResources(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) bool {
	switch componentType {
	case mv1.Component_MS:
		return d.prepareMSReconcileResources(ctx, dms)
	default:
		klog.Infof("prepareReconcileResource not support type=", componentType)
		return true
	}
}

// prepareFEReconcileResources prepare resource for fe reconcile
// response: bool, if true presents resource have ready for fe reconciling, if false presents resource is preparing.
func (d *DisaggregatedSubDefaultController) prepareMSReconcileResources(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService) bool {
	if len(dms.Spec.MS.PersistentVolumes) != 0 {
		return d.prepareMSPersistentVolumeClaim(ctx, dms, mv1.Component_MS)
	}

	return true
}

// 1. list pvcs, create or update,
// 1.1 labels use statefulset selector.
// 2. classify pvcs by mv1.PersistentVolume.name
// 2.1 travel pvcs, use key="-^"+volume.name, value=pvc put into map. starting with "-^" as the k8s resource name not allowed start with it.
func (d *DisaggregatedSubDefaultController) prepareMSPersistentVolumeClaim(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) bool {

	spec := resource.GetDMSBaseSpecFromCluster(dms, componentType)
	replicas := *spec.Replicas

	pvcList := corev1.PersistentVolumeClaimList{}
	selector := mv1.GenerateStatefulSetSelector(dms, componentType)
	stsName := mv1.GenerateComponentStatefulSetName(dms, componentType)
	if err := d.K8sclient.List(ctx, &pvcList, client.InNamespace(dms.Namespace), client.MatchingLabels(selector)); err != nil {
		d.K8srecorder.Event(dms, EventWarning, PVCListFailed, string("list component "+componentType+" failed!"))
		return false
	}
	//classify pvc by volume.Name, pvc.name generate by volume.Name + statefulset.Name + ordinal
	pvcMap := make(map[string][]corev1.PersistentVolumeClaim)

	for _, pvc := range pvcList.Items {
		//start with unique string for classify pvc, avoid empty string match all pvc.Name
		key := "-^"
		for _, volume := range spec.PersistentVolumes {
			if volume.Name != "" && strings.HasPrefix(pvc.Name, volume.Name) {
				key = key + volume.Name
				break
			}
		}

		if _, ok := pvcMap[key]; !ok {
			pvcMap[key] = []corev1.PersistentVolumeClaim{}
		}
		pvcMap[key] = append(pvcMap[key], pvc)
	}

	//presents the pvc have all created or updated to new version.
	prepared := true
	for _, volume := range spec.PersistentVolumes {
		// if provider not `operator` should not manage pvc.
		if volume.PVCProvisioner != mv1.PVCProvisionerOperator {
			continue
		}

		if !d.patchMSPVCs(ctx, dms, selector, pvcMap["-^"+volume.Name], stsName, volume, replicas) {
			prepared = false
		}
	}

	return prepared
}

func (d *DisaggregatedSubDefaultController) patchMSPVCs(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService, selector map[string]string,
	pvcs []corev1.PersistentVolumeClaim, stsName string, volume mv1.PersistentVolume, replicas int32) bool {
	//patch already exist in k8s .
	prepared := true
	for _, pvc := range pvcs {
		oldCapacity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		newCapacity := volume.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage]
		if !oldCapacity.Equal(newCapacity) {
			// if pvc need update, the resource have not prepared, return false.
			prepared = false
			eventType := EventNormal
			reason := PVCUpdate
			message := pvc.Name + " update successfully!"
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = newCapacity
			if err := d.K8sclient.Patch(ctx, &pvc, client.Merge); err != nil {
				klog.Errorf("DisaggregatedSubDefaultController namespace %s name %s patch pvc %s failed, %s", dms.Namespace, dms.Name, pvc.Name, err.Error())
				eventType = EventWarning
				reason = PVCUpdateFailed
				message = pvc.Name + " update failed, " + err.Error()
			}

			d.K8srecorder.Event(dms, eventType, reason, message)
		}
	}

	// if need add new pvc, the resource prepared not finished, return false.
	if len(pvcs) < int(replicas) {
		prepared = false
		d.K8srecorder.Event(dms, EventNormal, PVCCreate, fmt.Sprintf("create PVC ordinal %d - %d", len(pvcs), replicas))
	}

	baseOrdinal := len(pvcs)
	for ; baseOrdinal < int(replicas); baseOrdinal++ {
		pvc := resource.BuildDMSPVC(volume, selector, dms.Namespace, stsName, strconv.Itoa(baseOrdinal))
		if err := d.K8sclient.Create(ctx, &pvc); err != nil && !apierrors.IsAlreadyExists(err) {
			d.K8srecorder.Event(dms, EventWarning, PVCCreateFailed, err.Error())
			klog.Errorf("DisaggregatedSubDefaultController namespace %s name %s create pvc %s failed, %s.", dms.Namespace, dms.Name, pvc.Name)
		}
	}

	return prepared
}

// ClearCommonResources clear common resources all component have, as statefulset, service.
// response `bool` represents all resource have deleted, if not and delete resource failed return false for next reconcile retry.
func (d *DisaggregatedSubDefaultController) ClearCommonResources(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) (bool, error) {
	//if the doris is not have cn.
	stName := mv1.GenerateComponentStatefulSetName(dms, componentType)
	//externalServiceName := mv1.GenerateExternalServiceName(dms, componentType)
	internalServiceName := mv1.GenerateInternalCommunicateServiceName(dms, componentType)
	if err := k8s.DeleteStatefulset(ctx, d.K8sclient, dms.Namespace, stName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("DisaggregatedSubDefaultController ClearCommonResources delete statefulset failed, namespace=%s,name=%s, error=%s.", dms.Namespace, stName, err.Error())
		return false, err
	}

	if err := k8s.DeleteService(ctx, d.K8sclient, dms.Namespace, internalServiceName); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("DisaggregatedSubDefaultController ClearCommonResources delete search service, namespace=%s,name=%s,error=%s.", dms.Namespace, internalServiceName, err.Error())
		return false, err
	}

	return true, nil
}

// RecycleResources pvc resource for recycle
func (d *DisaggregatedSubDefaultController) RecycleResources(ctx context.Context, dcr *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) error {
	switch componentType {
	case mv1.Component_MS:
		return d.recycleMSResources(ctx, dcr)
	default:
		klog.Infof("RecycleResources not support type=%s", componentType)
		return nil
	}
}

// recycleMSResources pvc resource for meta-service recycle
func (d *DisaggregatedSubDefaultController) recycleMSResources(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService) error {
	if len(dms.Spec.MS.PersistentVolumes) != 0 {
		return d.listAndDeletePersistentVolumeClaim(ctx, dms, mv1.Component_MS)
	}
	return nil
}

// listAndDeletePersistentVolumeClaim:
// 1. list pvcs by statefulset selector labels .
// 2. get pvcs by mv1.PersistentVolume.name
// 2.1 travel pvcs, use key="-^"+volume.name, value=pvc put into map. starting with "-^" as the k8s resource name not allowed start with it.
// 3. delete pvc
func (d *DisaggregatedSubDefaultController) listAndDeletePersistentVolumeClaim(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService, componentType mv1.ComponentType) error {
	spec := resource.GetDMSBaseSpecFromCluster(dms, componentType)
	volumes := spec.PersistentVolumes
	replicas := spec.Replicas

	pvcList := corev1.PersistentVolumeClaimList{}
	selector := mv1.GenerateStatefulSetSelector(dms, componentType)
	stsName := mv1.GenerateComponentStatefulSetName(dms, componentType)
	if err := d.K8sclient.List(ctx, &pvcList, client.InNamespace(dms.Namespace), client.MatchingLabels(selector)); err != nil {
		d.K8srecorder.Event(dms, EventWarning, PVCListFailed, string("list component "+componentType+" failed!"))
		return err
	}
	//classify pvc by volume.Name, pvc.name generate by volume.Name + statefulset.Name + ordinal
	pvcMap := make(map[string][]corev1.PersistentVolumeClaim)

	for _, pvc := range pvcList.Items {
		//start with unique string for classify pvc, avoid empty string match all pvc.Name
		key := "-^"
		for _, volume := range volumes {
			if volume.Name != "" && strings.HasPrefix(pvc.Name, volume.Name) {
				key = key + volume.Name
				break
			}
		}

		if _, ok := pvcMap[key]; !ok {
			pvcMap[key] = []corev1.PersistentVolumeClaim{}
		}
		pvcMap[key] = append(pvcMap[key], pvc)
	}

	var mergeError error
	for _, volume := range volumes {
		// Clean up the existing PVC that is larger than expected
		claims := pvcMap["-^"+volume.Name]
		if len(claims) <= int(*replicas) {
			continue
		}
		if err := d.deletePVCs(ctx, dms, selector, len(claims), stsName, volume.Name, *replicas); err != nil {
			mergeError = utils.MergeError(mergeError, err)
		}
	}
	return mergeError
}

// deletePVCs will Loop to remove excess pvc
func (d *DisaggregatedSubDefaultController) deletePVCs(ctx context.Context, dms *mv1.DorisDisaggregatedMetaService, selector map[string]string,
	pvcSize int, stsName, volumeName string, replicas int32) error {
	maxOrdinal := pvcSize
	var mergeError error
	for ; maxOrdinal > int(replicas); maxOrdinal-- {
		pvcName := resource.BuildPVCName(stsName, strconv.Itoa(maxOrdinal-1), volumeName)
		if err := k8s.DeletePVC(ctx, d.K8sclient, dms.Namespace, pvcName, selector); err != nil {
			d.K8srecorder.Event(dms, EventWarning, PVCDeleteFailed, err.Error())
			klog.Errorf("DisaggregatedSubDefaultController deletePVCs failed: namespace %s, name %s delete pvc %s, err: %s .", dms.Namespace, dms.Name, pvcName, err.Error())
			mergeError = utils.MergeError(mergeError, err)
		}
	}
	return mergeError
}

func (d *DisaggregatedSubDefaultController) ClassifyPodsByStatus(namespace string, status *mv1.BaseStatus, labels map[string]string, replicas int32) error {
	var podList corev1.PodList
	if err := d.K8sclient.List(context.Background(), &podList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return err
	}

	var creatings, readys, faileds []string
	podmap := make(map[string]corev1.Pod)
	//get all pod status that controlled by st.
	for _, pod := range podList.Items {
		podmap[pod.Name] = pod
		if ready := k8s.PodIsReady(&pod.Status); ready {
			readys = append(readys, pod.Name)
		} else if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			creatings = append(creatings, pod.Name)
		} else {
			faileds = append(faileds, pod.Name)
		}
	}

	if len(readys) == int(replicas) {
		status.Phase = mv1.Ready
	} else if len(faileds) != 0 {
		status.Phase = mv1.Failed
	} else if len(creatings) != 0 {
		status.Phase = mv1.Creating
	}

	status.AvailableStatus = mv1.UnAvailable
	if status.Phase == mv1.Ready {
		status.AvailableStatus = mv1.Available
	}
	return nil
}
