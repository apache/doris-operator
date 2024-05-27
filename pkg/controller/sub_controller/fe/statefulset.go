package fe

import (
	"context"
	v1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/k8s"
	"github.com/selectdb/doris-operator/pkg/common/utils/mysql"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

func (fc *Controller) buildFEStatefulSet(dcr *v1.DorisCluster) appv1.StatefulSet {
	st := resource.NewStatefulSet(dcr, v1.Component_FE)
	st.Spec.Template = fc.buildFEPodTemplateSpec(dcr)
	return st
}

// ScaleOutObserver handles doris'SQL(drop frontend) through the MySQL client when dealing with scale out observer
// targetStatefulSet is new StatefulSet
// targetDCR is new dcr
// scaleNumber is the number of Observer needing scale out
func (fc *Controller) ScaleOutObserver(
	ctx context.Context,
	k8sclient client.Client,
	targetStatefulSet *appv1.StatefulSet,
	targetDCR *v1.DorisCluster,
	scaleNumber int) error {

	// get adminuserName and pwd
	secret, err := k8s.GetSecret(ctx, k8sclient, targetStatefulSet.Namespace, targetDCR.Spec.AuthSecret)
	if err != nil {
		klog.Infof("GetSecret failed, secret:%s, namespace:%s , err:%s", targetDCR.Spec.AuthSecret, targetStatefulSet.Namespace, err.Error())
	}
	adminUserName, password := v1.GetClusterSecret(secret)

	// get host and port
	serviceName := v1.GenerateExternalServiceName(targetDCR, v1.Component_FE)
	service, err := k8s.GetService(ctx, k8sclient, targetStatefulSet.Namespace, serviceName)
	if err != nil {
		klog.Errorf("ScaleOutObserver get service failed, service:%s, namespace:%s, err:%s", serviceName, targetStatefulSet.Namespace, err.Error())
		return err
	}

	var queryPort string
	for _, port := range service.Spec.Ports {
		if port.Name == resource.GetPortKey(resource.QUERY_PORT) {
			queryPort = strconv.FormatInt(int64(port.Port), 10)
			continue
		}
	}

	// connect to doris sql
	dbConf := mysql.DBConfig{
		User:     adminUserName,
		Password: password,
		Host:     service.Spec.ClusterIP,
		Port:     queryPort,
		Database: "mysql",
	}
	klog.Infof("dbConf %+v", dbConf)
	db, err := mysql.NewDorisSqlDB(dbConf)

	// get all Observes
	observes, err := db.GetObserves()
	// get scaleNumber Observes
	slicedObserves := mysql.SortAndSliceObserves(*observes, scaleNumber)
	// drop node and return
	return db.DropObserver(slicedObserves)

}
