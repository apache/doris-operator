package disaggregated_fe

import (
	"context"
	"fmt"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// listAndDeletePersistentVolumeClaim:
// 1. list pvcs by statefulset selector labels .
// 2. buildVolumesVolumeMountsAndPVCs pvcs by new ddc
// 3. Compare the differences between the two and determine the PVC that needs to be cleared
// 4. delete pvc
func (dfc *DisaggregatedFEController) listAndDeletePersistentVolumeClaim(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) error {
	spec := ddc.Spec.FeSpec
	replicas := int(*spec.Replicas)
	currentPVCs := corev1.PersistentVolumeClaimList{}
	pvcLabels := dfc.newFEPodsSelector(ddc.Name)
	stsName := ddc.GetFEStatefulsetName()

	if err := dfc.K8sclient.List(ctx, &currentPVCs, client.InNamespace(ddc.Namespace), client.MatchingLabels(pvcLabels)); err != nil {
		dfc.K8srecorder.Event(ddc, string(sub_controller.EventWarning), sub_controller.PVCListFailed, fmt.Sprintf("DisaggregatedFEController listAndDeletePersistentVolumeClaim list pvc failed:%s!", err.Error()))
		return err
	}

	var reservePVCNameList []string
	for i := 0; i < replicas; i++ {
		reservePVCNameList = append(
			reservePVCNameList,
			resource.BuildPVCName(stsName, strconv.Itoa(i), LogStoreName),
			resource.BuildPVCName(stsName, strconv.Itoa(i), MetaStoreName),
		)
	}

	pvcMap := make(map[string]corev1.PersistentVolumeClaim)
	for _, pvc := range currentPVCs.Items {
		pvcMap[pvc.Name] = pvc
	}
	for _, pvcName := range reservePVCNameList {
		if _, ok := pvcMap[pvcName]; ok {
			delete(pvcMap, pvcName)
		}
	}

	var mergeError error
	for _, claim := range pvcMap {
		if err := k8s.DeletePVC(ctx, dfc.K8sclient, claim.Namespace, claim.Name, pvcLabels); err != nil {
			dfc.K8srecorder.Event(ddc, string(sub_controller.EventWarning), sub_controller.PVCDeleteFailed, err.Error())
			klog.Errorf("listAndDeletePersistentVolumeClaim deletePVCs failed: namespace %s, name %s delete pvc %s, err: %s .", claim.Namespace, claim.Name, claim.Name, err.Error())
			mergeError = utils.MergeError(mergeError, err)
		}
	}
	return mergeError
}
