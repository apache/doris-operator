package fe

import (
	"context"
	"fmt"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/mysql"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strconv"
	"strings"
)

func (fc *Controller) buildFEStatefulSet(dcr *v1.DorisCluster) appv1.StatefulSet {
	st := resource.NewStatefulSet(dcr, v1.Component_FE)
	st.Spec.Template = fc.buildFEPodTemplateSpec(dcr)
	return st
}

// ScaleDownObserver handles doris'SQL(drop frontend) through the MySQL client when dealing with scale in observer
// targetStatefulSet is new StatefulSet
// targetDCR is new dcr
// scaleNumber is the number of Observer needing scale in
func (fc *Controller) ScaleDownObserver(
	ctx context.Context,
	k8sclient client.Client,
	targetStatefulSet *appv1.StatefulSet,
	targetDCR *v1.DorisCluster,
	scaleNumber int32) error {

	// get adminuserName and pwd
	secret, _ := k8s.GetSecret(ctx, k8sclient, targetStatefulSet.Namespace, targetDCR.Spec.AuthSecret)
	adminUserName, password := v1.GetClusterSecret(targetDCR, secret)
	// get host and port
	serviceName := v1.GenerateExternalServiceName(targetDCR, v1.Component_FE)

	maps, _ := k8s.GetConfig(ctx, k8sclient, &targetDCR.Spec.FeSpec.ConfigMapInfo, targetStatefulSet.Namespace, v1.Component_FE)
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
	klog.Infof("dbConf %+v", dbConf)
	db, err := mysql.NewDorisSqlDB(dbConf)
	if err != nil {
		klog.Errorf("NewDorisSqlDB err:%s", err.Error())
		return err
	}
	defer db.Close()

	// get all Observes
	allObserves, err := db.GetObserves()

	if err != nil {
		klog.Errorf("GetObserves err:%s", err.Error())
		return err
	}
	klog.Errorf("GetObserves res :%+v", allObserves)

	// get scale Observes
	var frontendMap map[int]mysql.Frontend // frontendMap key is fe index ,value is frontend
	podNamePre := resource.GeneratePodTemplateName(targetDCR, v1.Component_FE)

	if resource.IsFQDN(maps) { // use host
		frontendMap, err = buildFrontendMap(*allObserves, nil, podNamePre)
		if err != nil {
			klog.Errorf("buildFrontendMap err:%s", err.Error())
			return nil
		}
	} else { // use ip
		podMap := make(map[string]string) // key is pod ip, value is pod name
		pods, err := k8s.GetPods(ctx, k8sclient, targetStatefulSet.Namespace)
		if err != nil {
			klog.Errorf("Get pods err:%s", err)
			return nil
		}
		for _, item := range pods.Items {
			if strings.HasPrefix(item.GetName(), podNamePre) {
				podMap[item.Status.PodIP] = item.GetName()
			}
		}
		frontendMap, err = buildFrontendMap(*allObserves, podMap, podNamePre)
		if err != nil {
			klog.Errorf("buildFrontendMap err:%s", err.Error())
			return nil
		}
	}
	klog.Errorf("frontendMap is %+v", frontendMap)
	observes := getTopFrontends(frontendMap, scaleNumber)
	klog.Errorf("drop observes is :%+v", observes)
	// drop node and return
	err = db.DropObserver(observes)
	return err

}

// buildFrontendMap
// input ipMap key is podIP,value is fe.podName(from 'kubectl get pods -owide')
// return frontendMap key is fe index ,value is frontend
func buildFrontendMap(frontends []mysql.Frontend, ipMap map[string]string, podNamePre string) (map[int]mysql.Frontend, error) {
	frontendMap := make(map[int]mysql.Frontend)
	for _, fe := range frontends {
		var podName string
		if strings.HasPrefix(fe.Host, podNamePre) {
			// use fqdn, not need ipMap
			split := strings.Split(strings.Split(fe.Host, ".")[0], "-")
			num, err := strconv.Atoi(split[len(split)-1])
			if err != nil {
				klog.Errorf("BuildFrontendMap (HOST) can not split pod name,pod name: %s,err:%s", fe.Host, err.Error())
				return nil, err
			}
			frontendMap[num] = fe
		} else {
			// ip
			podName = ipMap[fe.Host]
			split := strings.Split(podName, "-")
			num, err := strconv.Atoi(split[len(split)-1])
			if err != nil {
				klog.Errorf("BuildFrontendMap (IP) can not split pod name,pod name: %s,err:%s", podName, err.Error())
				return nil, err
			}
			frontendMap[num] = fe
		}
	}
	return frontendMap, nil

}

// sort fe by index and return top scaleNumber
func getTopFrontends(frontendMap map[int]mysql.Frontend, scaleNumber int32) []mysql.Frontend {
	keys := make([]int, 0, len(frontendMap))
	for k := range frontendMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] > keys[j]
	})
	topFrontends := make([]mysql.Frontend, scaleNumber)
	for i := 0; i < int(scaleNumber); i++ {
		topFrontends[i] = frontendMap[keys[i]]
	}
	return topFrontends
}
