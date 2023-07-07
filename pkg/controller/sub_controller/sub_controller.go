package sub_controller

import (
	"context"
	dorisv1 "github.com/selectdb/doris-operator/api/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SubController interface {
	//Sync reconcile for sub controller. bool represent the component have updated.
	Sync(ctx context.Context, cluster *dorisv1.DorisCluster) error
	//clear all resource about sub-component.
	ClearResources(ctx context.Context, cluster *dorisv1.DorisCluster) (bool, error)

	//return the controller name, beController, feController,cnController for log.
	GetControllerName() string

	//UpdateStatus update the component status on src.
	UpdateComponentStatus(cluster *dorisv1.DorisCluster) error
}

type SubDefaultController struct {
	SubController
	K8sclient   client.Client
	K8srecorder record.EventRecorder
}

// UpdateStatus update the component status on src.
func (d *SubDefaultController) UpdateStatus(namespace string, status *dorisv1.ComponentStatus, labels map[string]string, replicas int32) error {
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

	status.ComponentCondition.Phase = dorisv1.Reconciling
	if len(readys) == int(replicas) {
		status.ComponentCondition.Phase = dorisv1.Available
	} else if len(faileds) != 0 {
		status.ComponentCondition.Phase = dorisv1.HaveMemberFailed
		status.ComponentCondition.Reason = podmap[faileds[0]].Status.Reason
		status.ComponentCondition.Message = podmap[faileds[0]].Status.Message
	} else if len(creatings) != 0 {
		status.ComponentCondition.Reason = podmap[creatings[0]].Status.Reason
		status.ComponentCondition.Message = podmap[creatings[0]].Status.Message
	}

	status.RunningMembers = readys
	status.FailedMembers = faileds
	status.CreatingMembers = creatings
	return nil
}
