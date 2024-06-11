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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strconv"
	"strings"
)

// ControlClusterPhaseAndPreOperation means Pre-operation and status control on the client side
func (fc *Controller) controlClusterPhaseAndPreOperation(ctx context.Context, cluster *v1.DorisCluster) error {

	var oldSt appv1.StatefulSet
	err := fc.K8sclient.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: v1.GenerateComponentStatefulSetName(cluster, v1.Component_FE)}, &oldSt)
	phase, _ := k8s.GetDorisClusterPhase(ctx, fc.K8sclient, cluster.Name, cluster.Namespace)

	if err != nil || phase == nil || phase.Phase == v1.PHASE_INITIALIZING {
		klog.Infof("fe controller operator skip cluster operation, cluster is in INITIALIZING Phase")
		return nil
	}
	scaleNumber := *(cluster.Spec.FeSpec.Replicas) - *(oldSt.Spec.Replicas)

	if phase.Phase != v1.PHASE_OPERABLE && phase.Retry != v1.RETRY_OPERATOR_FE {
		// means other task running, send Event warning
		fc.K8srecorder.Eventf(
			cluster, sub_controller.EventWarning,
			sub_controller.ClusterOperationalConflicts,
			"There is a conflict in crd operation. currently, cluster Phase is %+v ", phase.Phase,
		)
		return errors.New(fmt.Sprintf("There is a conflict in crd operation. currently, cluster Phase is %+v ", phase.Phase))
	}

	if phase.Phase == v1.PHASE_OPERABLE || phase.Retry == v1.RETRY_OPERATOR_FE {

		// fe scale
		if scaleNumber != 0 { // set Phase as SCALING
			clusterPhase := v1.ClusterPhase{
				Phase: v1.PHASE_SCALING,
				Retry: v1.RETRY_OPERATOR_FE, // must set Retry as RETRY_OPERATOR_FE for an error occurs, Retry will be reset as RETRY_OPERATOR_NO after a success.
			}
			cluster.Status.ClusterPhase = clusterPhase
			if err := k8s.SetDorisClusterPhase(ctx, fc.K8sclient, cluster.Name, cluster.Namespace, clusterPhase); err != nil {
				klog.Errorf("SetDorisClusterPhase 'SCALING' failed err:%s ", err.Error())
				return err
			}
		}
		if scaleNumber < 0 {
			if err := fc.dropObserverFromSqlClient(ctx, fc.K8sclient, cluster); err != nil {
				klog.Errorf("ScaleDownObserver failed, err:%s ", err.Error())
				return err
			}
			if scaleNumber != -1 {
				klog.Info("controlClusterPhaseAndPreOperation scale down observer task is not completed, %d tasks are left. ", -scaleNumber-1)
				subReplicas := *(oldSt.Spec.Replicas) - 1
				cluster.Spec.FeSpec.Replicas = &subReplicas
			}
			return nil
		}

		//TODO check upgrade ,restart
	}

	return nil
}

// DropObserverFromSqlClient handles doris'SQL(drop frontend) through the MySQL client when dealing with scale in observer
// targetDCR is new dcr
// scaleNumber is the number of Observer needing scale down
func (fc *Controller) dropObserverFromSqlClient(ctx context.Context, k8sclient client.Client, targetDCR *v1.DorisCluster) error {

	// get adminuserName and pwd
	secret, _ := k8s.GetSecret(ctx, k8sclient, targetDCR.Namespace, targetDCR.Spec.AuthSecret)
	adminUserName, password := v1.GetClusterSecret(targetDCR, secret)
	// get host and port
	serviceName := v1.GenerateExternalServiceName(targetDCR, v1.Component_FE)
	maps, _ := k8s.GetConfig(ctx, k8sclient, &targetDCR.Spec.FeSpec.ConfigMapInfo, targetDCR.Namespace, v1.Component_FE)
	queryPort := resource.GetPort(maps, resource.QUERY_PORT)
	// connect to doris sql
	dbConf := mysql.DBConfig{
		User:     adminUserName,
		Password: password,
		Host:     serviceName,
		Port:     strconv.FormatInt(int64(queryPort), 10),
		Database: "mysql",
	}
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

	if int32(len(allObserves)) <= *(targetDCR.Spec.FeSpec.Replicas)-*(targetDCR.Spec.FeSpec.ElectionNumber) {
		klog.Errorf("DropObserverFromSqlClient failed, Observers size(%d) is not larger than scale number(%d) ", len(allObserves), *(targetDCR.Spec.FeSpec.Replicas)-*(targetDCR.Spec.FeSpec.ElectionNumber))
		return nil
	}

	// get scale Observes
	var frontendMap map[int]*mysql.Frontend // frontendMap key is fe pod index ,value is frontend
	podTemplateName := resource.GeneratePodTemplateName(targetDCR, v1.Component_FE)

	if resource.GetStartMode(maps) == resource.START_MODEL_FQDN { // use host
		frontendMap, err = buildSeqNumberToFrontend(allObserves, nil, podTemplateName)
		if err != nil {
			klog.Errorf("DropObserverFromSqlClient failed, buildSeqNumberToFrontend err:%s", err.Error())
			return nil
		}
	} else { // use ip
		podMap := make(map[string]string) // key is pod ip, value is pod name
		pods, err := k8s.GetPods(ctx, k8sclient, *targetDCR)
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

	observes := getTopFrontends(frontendMap, 1)
	// drop node and return
	return db.DropObserver(observes)

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
	var topFrontends []*mysql.Frontend
	if int(scaleNumber) <= len(frontendMap) {
		keys := make([]int, 0, len(frontendMap))
		for k := range frontendMap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] > keys[j]
		})

		for i := 0; i < int(scaleNumber); i++ {
			topFrontends = append(topFrontends, frontendMap[keys[i]])
		}
	} else {
		klog.Errorf("getTopFrontends frontendMap size(%d) not larger than scaleNumber(%d)", len(frontendMap), scaleNumber)
	}
	return topFrontends
}
