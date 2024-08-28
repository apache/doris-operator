package metaservice

import (
	"context"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	sc "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type DisaggregatedMSController struct {
	sc.DisaggregatedSubDefaultController
}

func (dms *DisaggregatedMSController) ClearResources(ctx context.Context, obj client.Object) (bool, error) {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)

	statefulsetName := ddc.GetMSStatefulsetName()
	serviceName := ddc.GetMSServiceName()

	if ddc.DeletionTimestamp.IsZero() {
		return true, nil
	}

	if err := k8s.DeleteService(ctx, dms.K8sclient, ddc.Namespace, serviceName); err != nil {
		klog.Errorf("dms controller delete service namespace %s name %s failed, err=%s", ddc.Namespace, serviceName, err.Error())
		dms.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.MSServiceDeletedFailed), err.Error())
		return false, err
	}

	if err := k8s.DeleteStatefulset(ctx, dms.K8sclient, ddc.Namespace, statefulsetName); err != nil {
		klog.Errorf("dms controller delete statefulset namespace %s name %s failed, err=%s", ddc.Namespace, statefulsetName, err.Error())
		dms.K8srecorder.Event(ddc, string(sc.EventWarning), string(sc.MSStatefulsetDeleteFailed), err.Error())
		return false, err
	}

	return true, nil
}

func (dms *DisaggregatedMSController) GetControllerName() string {
	return dms.ControllerName
}

func (dms *DisaggregatedMSController) UpdateComponentStatus(obj client.Object) error {
	var availableReplicas int32
	var creatingReplicas int32
	var failedReplicas int32

	ddc := obj.(*dv1.DorisDisaggregatedCluster)

	msSpec := ddc.Spec.MetaService
	confMap := dms.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.MS_RESOLVEKEY, msSpec.ConfigMaps)
	port := resource.GetPort(confMap, resource.BRPC_LISTEN_PORT)
	msEndPoint := ddc.GetMSServiceName() + "." + ddc.Namespace + ":" + strconv.Itoa(int(port))
	ddc.Status.MetaServiceStatus.MetaServiceEndpoint = msEndPoint
	token := resource.DefaultMsToken
	if v, ok := confMap[resource.DefaultMsTokenKey]; ok {
		token = v.(string)
	}
	ddc.Status.MetaServiceStatus.MsToken = token
	selector := dms.newMSPodsSelector(ddc.Name)
	var podList corev1.PodList
	if err := dms.K8sclient.List(context.Background(), &podList, client.InNamespace(ddc.Namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if ready := k8s.PodIsReady(&pod.Status); ready {
			availableReplicas++
		} else if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			creatingReplicas++
		} else {
			failedReplicas++
		}
	}

	if availableReplicas > 0 {
		ddc.Status.MetaServiceStatus.AvailableStatus = dv1.Available
		ddc.Status.MetaServiceStatus.Phase = dv1.Ready
	}

	return nil
}

var _ sc.DisaggregatedSubController = &DisaggregatedMSController{}

var (
	metaServiceController = "metaServiceController"
)

func New(mgr ctrl.Manager) *DisaggregatedMSController {
	return &DisaggregatedMSController{
		sc.DisaggregatedSubDefaultController{
			K8sclient:      mgr.GetClient(),
			K8srecorder:    mgr.GetEventRecorderFor(metaServiceController),
			ControllerName: metaServiceController,
		}}
}

func (dms *DisaggregatedMSController) Sync(ctx context.Context, obj client.Object) error {
	ddc := obj.(*dv1.DorisDisaggregatedCluster)
	msSpec := ddc.Spec.MetaService
	confMap := dms.GetConfigValuesFromConfigMaps(ddc.Namespace, resource.MS_RESOLVEKEY, msSpec.ConfigMaps)
	svc := dms.newService(ddc, confMap)

	st := dms.newStatefulset(ddc, confMap)
	dms.initMSStatus(ddc)

	event, err := dms.DefaultReconcileService(ctx, svc)
	if err != nil {
		if event != nil {
			dms.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
		}
		klog.Errorf("dms controller reconcile service namespace %s name %s failed, err=%s", svc.Namespace, svc.Name, err.Error())
		return err
	}

	event, err = dms.reconcileStatefulset(ctx, st)
	if err != nil {
		if event != nil {
			dms.K8srecorder.Event(ddc, string(event.Type), string(event.Reason), event.Message)
		}
		klog.Errorf("dms controller reconcile statefulset namespace %s name %s failed, err=%s", st.Namespace, st.Name, err.Error())
		return err
	}

	return nil
}

func (dms *DisaggregatedMSController) reconcileStatefulset(ctx context.Context, st *appv1.StatefulSet) (*sc.Event, error) {
	var est appv1.StatefulSet
	if err := dms.K8sclient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est); apierrors.IsNotFound(err) {
		if err = k8s.CreateClientObject(ctx, dms.K8sclient, st); err != nil {
			klog.Errorf("dms controller reconcileStatefulset create statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
			return &sc.Event{Type: sc.EventWarning, Reason: sc.CCCreateResourceFailed, Message: err.Error()}, err
		}

		return nil, nil
	} else if err != nil {
		klog.Errorf("dms controller reconcileStatefulset get statefulset failed, namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return nil, err
	}

	if err := k8s.ApplyStatefulSet(ctx, dms.K8sclient, st, func(st, est *appv1.StatefulSet) bool {
		return resource.StatefulsetDeepEqualWithOmitKey(st, est, dv1.DisaggregatedSpecHashValueAnnotation, true, false)
	}); err != nil {
		klog.Errorf("dms controller reconcileStatefulset apply statefulset namespace=%s name=%s failed, err=%s", st.Namespace, st.Name, err.Error())
		return &sc.Event{Type: sc.EventWarning, Reason: sc.CCApplyResourceFailed, Message: err.Error()}, err
	}

	return nil, nil
}

func (dms *DisaggregatedMSController) initMSStatus(ddc *dv1.DorisDisaggregatedCluster) {
	initPhase := dv1.Reconciling
	if ddc.Status.MetaServiceStatus.Phase != "" {
		initPhase = ddc.Status.MetaServiceStatus.Phase
	}
	//re initial status to un available
	ddc.Status.MetaServiceStatus.AvailableStatus = dv1.UnAvailable
	ddc.Status.MetaServiceStatus.Phase = initPhase
}
