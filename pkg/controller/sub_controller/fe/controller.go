package fe

import (
	"context"
	"errors"
	"fmt"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/mysql"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Controller struct {
	sub_controller.SubDefaultController
}

func (fc *Controller) ClearResources(ctx context.Context, cluster *v1.DorisCluster) (bool, error) {
	//if the doris is not have fe.
	if cluster.Status.FEStatus == nil {
		return true, nil
	}

	if cluster.DeletionTimestamp.IsZero() {
		return true, nil
	}

	return fc.ClearCommonResources(ctx, cluster, v1.Component_FE)
}

func (fc *Controller) UpdateComponentStatus(cluster *v1.DorisCluster) error {
	//if spec is not exist, status is empty. but before clear status we must clear all resource about be used by ClearResources.
	if cluster.Spec.FeSpec == nil {
		cluster.Status.FEStatus = nil
		return nil
	}

	fs := &v1.ComponentStatus{
		ComponentCondition: v1.ComponentCondition{
			SubResourceName:    v1.GenerateComponentStatefulSetName(cluster, v1.Component_FE),
			Phase:              v1.Reconciling,
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}

	if cluster.Status.FEStatus != nil {
		fs = cluster.Status.FEStatus.DeepCopy()
	}

	cluster.Status.FEStatus = fs
	fs.AccessService = v1.GenerateExternalServiceName(cluster, v1.Component_FE)

	return fc.ClassifyPodsByStatus(cluster.Namespace, fs, v1.GenerateStatefulSetSelector(cluster, v1.Component_FE), *cluster.Spec.FeSpec.Replicas)
}

func (fc *Controller) GetComponentStatus(cluster *v1.DorisCluster) v1.ComponentPhase {
	return cluster.Status.FEStatus.ComponentCondition.Phase
}

// New construct a FeController.
func New(k8sclient client.Client, k8sRecorder record.EventRecorder) *Controller {
	return &Controller{
		SubDefaultController: sub_controller.SubDefaultController{
			K8sclient:   k8sclient,
			K8srecorder: k8sRecorder,
		},
	}
}

func (fc *Controller) GetControllerName() string {
	return "feController"
}

// Sync DorisCluster to fe statefulset and service.
func (fc *Controller) Sync(ctx context.Context, cluster *v1.DorisCluster) error {
	if cluster.Spec.FeSpec == nil {
		klog.Info("fe Controller Sync ", "the fe component is not needed ", "namespace ", cluster.Namespace, " doris cluster name ", cluster.Name)
		return nil
	}

	feSpec := cluster.Spec.FeSpec
	//get the fe configMap for resolve ports.
	config, err := fc.GetConfig(ctx, &feSpec.BaseSpec.ConfigMapInfo, cluster.Namespace, v1.Component_FE)
	if err != nil {
		klog.Error("fe Controller Sync ", "resolve fe configmap failed, namespace ", cluster.Namespace, " error :", err)
		return err
	}
	fc.CheckConfigMountPath(cluster, v1.Component_FE)

	//generate new fe service.
	svc := resource.BuildExternalService(cluster, v1.Component_FE, config)
	//create or update fe external and domain search service, update the status of fe on src.
	internalService := resource.BuildInternalService(cluster, v1.Component_FE, config)
	if err := k8s.ApplyService(ctx, fc.K8sclient, &internalService, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("fe controller sync apply internalService name=%s, namespace=%s, clusterName=%s failed.message=%s.",
			internalService.Name, internalService.Namespace, cluster.Name, err.Error())
		return err
	}
	if err := k8s.ApplyService(ctx, fc.K8sclient, &svc, resource.ServiceDeepEqual); err != nil {
		klog.Errorf("fe controller sync apply external service name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			svc.Name, svc.Namespace, cluster.Name, err.Error())
		return err
	}

	st := fc.buildFEStatefulSet(cluster)
	if !fc.PrepareReconcileResources(ctx, cluster, v1.Component_FE) {
		klog.Infof("fe controller sync preparing resource for reconciling namespace %s name %s!", cluster.Namespace, cluster.Name)
		return nil
	}

	// fe cluster operator
	if err2 := fc.controlClusterPhaseAndPreOperation(ctx, *cluster); err2 != nil {
		return err
	}

	if err = k8s.ApplyStatefulSet(ctx, fc.K8sclient, &st, func(new *appv1.StatefulSet, old *appv1.StatefulSet) bool {
		fc.RestrictConditionsEqual(new, old)
		return resource.StatefulSetDeepEqual(new, old, false)
	}); err != nil {
		klog.Errorf("fe controller sync statefulset name=%s, namespace=%s, clusterName=%s failed. message=%s.",
			st.Name, st.Namespace, cluster.Name, err.Error())
		return err
	}

	currentPhase, err := k8s.GetDorisClusterPhase(ctx, fc.K8sclient, cluster.Name, cluster.Namespace)
	if err != nil {
		klog.Errorf("fe controller sync after cluster operation GetDorisClusterPhase failed, err:%s ", err.Error())
	}

	if currentPhase.Phase != v1.PHASE_INITIALIZING && currentPhase.Phase != "" {
		err = k8s.SetDorisClusterPhase(ctx, fc.K8sclient, cluster.Name, cluster.Namespace, v1.ClusterPhase{v1.PHASE_OPERABLE, v1.RETRY_OPERATOR_NO})
		if err != nil {
			klog.Errorf("fe controller sync SetDorisClusterPhase 'OPERABLE' failed, err:%s ", err.Error())
		}
	}

	return nil
}

// ControlClusterPhaseAndPreOperation means Pre-operation and status control on the client side
func (fc *Controller) controlClusterPhaseAndPreOperation(ctx context.Context, cluster v1.DorisCluster) error {

	var oldSt appv1.StatefulSet
	err := fc.K8sclient.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: v1.GenerateComponentStatefulSetName(&cluster, v1.Component_FE)}, &oldSt)
	phase, _ := k8s.GetDorisClusterPhase(ctx, fc.K8sclient, cluster.Name, cluster.Namespace)

	if err != nil || phase == nil || phase.Phase == v1.PHASE_INITIALIZING {
		klog.Infof("fe controller operator skip cluster operation, cluster is in INITIALIZING Phase")
		return nil
	}

	// update cluster not start cluster

	klog.Errorf("-----new.Spec.Replicas: %d ", *(cluster.Spec.FeSpec.Replicas))
	klog.Errorf("-----old.Spec.Replicas: %d ", *(oldSt.Spec.Replicas))
	klog.Errorf("-----cluster.Spec.FeSpec.Replicas: %d ", *(cluster.Spec.FeSpec.Replicas))
	scaleNumber := *(cluster.Spec.FeSpec.Replicas) - *(oldSt.Spec.Replicas)
	klog.Errorf("-----scaleNumber : %d ", scaleNumber)

	if phase.Phase != v1.PHASE_OPERABLE && phase.Retry != v1.RETRY_OPERATOR_FE {
		// means other task running, send Event warning
		fc.K8srecorder.Eventf(
			&cluster, sub_controller.EventWarning,
			sub_controller.ClusterOperationalConflicts,
			"There is a conflict in crd operation. currently, cluster Phase is %+v ", phase.Phase,
		)
		return errors.New(fmt.Sprintf("There is a conflict in crd operation. currently, cluster Phase is %+v ", phase.Phase))
	}

	if phase.Phase == v1.PHASE_OPERABLE || phase.Retry == v1.RETRY_OPERATOR_FE {

		// fe scale
		if scaleNumber != 0 { // set Phase as SCALING
			if err := k8s.SetDorisClusterPhase(ctx, fc.K8sclient, cluster.Name, cluster.Namespace,
				v1.ClusterPhase{
					Phase: v1.PHASE_SCALING,
					Retry: v1.RETRY_OPERATOR_FE, // must set Retry as RETRY_OPERATOR_FE for an error occurs, Retry will be reset as RETRY_OPERATOR_NO after a success.
				},
			); err != nil {
				klog.Errorf("SetDorisClusterPhase 'SCALING' failed err:%s ", err.Error())
				return err
			}
		}
		if scaleNumber < 0 {
			if err := fc.dropObserverFromSqlClient(ctx, fc.K8sclient, cluster, -scaleNumber); err != nil {
				klog.Errorf("ScaleDownObserver failed, err:%s ", err.Error())
				return err
			}
		}

		//TODO check upgrade ,restart
	}

	return nil
}

// DropObserverFromSqlClient handles doris'SQL(drop frontend) through the MySQL client when dealing with scale in observer
// targetDCR is new dcr
// scaleNumber is the number of Observer needing scale in
func (fc *Controller) dropObserverFromSqlClient(
	ctx context.Context,
	k8sclient client.Client,
	targetDCR v1.DorisCluster,
	scaleNumber int32) error {

	// get adminuserName and pwd
	secret, _ := k8s.GetSecret(ctx, k8sclient, targetDCR.Namespace, targetDCR.Spec.AuthSecret)
	adminUserName, password := v1.GetClusterSecret(&targetDCR, secret)
	// get host and port
	serviceName := v1.GenerateExternalServiceName(&targetDCR, v1.Component_FE)

	maps, _ := k8s.GetConfig(ctx, k8sclient, &targetDCR.Spec.FeSpec.ConfigMapInfo, targetDCR.Namespace, v1.Component_FE)
	queryPort := resource.GetPort(maps, resource.QUERY_PORT)
	fmt.Printf("queryPort : %d \n", queryPort)
	// connect to doris sql
	dbConf := mysql.DBConfig{
		User:     adminUserName,
		Password: password,
		Host:     serviceName,
		Port:     strconv.FormatInt(int64(queryPort), 10),
		Database: "mysql",
	}
	klog.Infof("-----dbConf %+v", dbConf)
	db, err := mysql.NewDorisSqlDB(dbConf)
	if err != nil {
		klog.Errorf("DropObserverFromSqlClient failed, NewDorisSqlDB err:%s", err.Error())
		return err
	}
	defer db.Close()

	// get all Observes
	allObserves, err := db.GetObservers()

	if err != nil {
		klog.Errorf("DropObserverFromSqlClient failed, GetObservers err:%s", err.Error())
		return err
	}
	for i := range allObserves {
		a := allObserves[i]
		klog.Errorf("-----GetObservers res %d :%+v", i, *a)
	}

	// get scale Observes
	var frontendMap map[int]*mysql.Frontend // frontendMap key is fe pod index ,value is frontend
	podTemplateName := resource.GeneratePodTemplateName(&targetDCR, v1.Component_FE)

	if resource.GetStartMode(maps) == resource.START_MODEL_FQDN { // use host
		frontendMap, err = buildSeqNumberToFrontend(allObserves, nil, podTemplateName)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	} else { // use ip
		podMap := make(map[string]string) // key is pod ip, value is pod name
		pods, err := k8s.GetPods(ctx, k8sclient, targetDCR)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, GetPods err:%s", err)
			return nil
		}
		for _, item := range pods.Items {
			if strings.HasPrefix(item.GetName(), podTemplateName) {
				podMap[item.Status.PodIP] = item.GetName()
			}
		}
		frontendMap, err = buildSeqNumberToFrontend(allObserves, podMap, podTemplateName)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	}
	for i, frontend := range frontendMap {
		a := frontend
		klog.Errorf("----frontendMap %d is %+v", i, *a)
	}

	// There is a probability that a situation will be triggered:
	// when the connected fe is the one that needs to be dropped,
	// the drop command will be terminated in the order in which
	// it is executed, and the subsequent drops will be refused
	// to execute due to the interruption of the connection.
	// Therefore, when retrying, the number of fe observers will
	// be less than scaleNumber, causing the array to be out of
	// bounds. Therefore, a minimum value is taken.
	realDropNum := scaleNumber
	if len(frontendMap) < int(scaleNumber) {
		realDropNum = int32(len(frontendMap))
	}

	observes := getTopFrontends(frontendMap, realDropNum)
	klog.Errorf("----drop observes is :%+v", observes)
	// drop node and return
	err = db.DropObserver(observes)
	return err

}

// buildSeqNumberToFrontend
// input ipMap key is podIP,value is fe.podName(from 'kubectl get pods -owide')
// return frontendMap key is fe pod index ,value is frontend
func buildSeqNumberToFrontend(frontends []*mysql.Frontend, ipMap map[string]string, podTemplateName string) (map[int]*mysql.Frontend, error) {
	frontendMap := make(map[int]*mysql.Frontend)
	for _, fe := range frontends {
		var podSignName string
		if strings.HasPrefix(fe.Host, podTemplateName) {
			// use fqdn, not need ipMap
			// podSignName like: doriscluster-sample-fe-0.doriscluster-sample-fe-internal.doris.svc.cluster.local
			podSignName = fe.Host
		} else {
			// use ip
			// podSignName like: doriscluster-sample-fe-0
			podSignName = ipMap[fe.Host]
		}
		split := strings.Split(strings.Split(strings.Split(podSignName, podTemplateName)[1], ".")[0], "-")
		num, err := strconv.Atoi(split[len(split)-1])
		if err != nil {
			klog.Errorf("buildSeqNumberToFrontend can not split pod name,pod name: %s,err:%s", podSignName, err.Error())
			return nil, err
		}
		frontendMap[num] = fe
	}
	return frontendMap, nil
}

// sort fe by index and return top scaleNumber
func getTopFrontends(frontendMap map[int]*mysql.Frontend, scaleNumber int32) []*mysql.Frontend {
	keys := make([]int, 0, len(frontendMap))
	for k := range frontendMap {
		keys = append(keys, k)
	}
	klog.Errorf("-----lennnnnnnnn %d", len(frontendMap))
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] > keys[j]
	})
	klog.Errorf("-----lennnnnnnnn2 %d", len(keys))

	topFrontends := make([]*mysql.Frontend, scaleNumber)
	for i := 0; i < int(scaleNumber); i++ {
		topFrontends[i] = frontendMap[keys[i]]
	}
	return topFrontends
}
