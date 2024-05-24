package mysql

import (
	_ "crypto/tls"
	"fmt"
	"testing"
	"time"
)

func TestAPIs(t *testing.T) {

	cfg := DBConfig{
		User:     "root",
		Password: "",
		Host:     "127.0.0.1",
		Port:     "9030",
		Database: "mysql",
	}

	db, err := NewDorisSqlDB(cfg)
	if err != nil {
		fmt.Printf("NewDorisSqlDB err : %s\n", err.Error())
	}
	defer db.Close()

	// get master
	master, err := db.GetMaster()
	if err != nil {
		fmt.Printf("get master err:%s \n", err.Error())
	}
	fmt.Printf("getmaster :%+v \n", master)

	// ShowFrontends
	frontends, err := db.ShowFrontends()
	if err != nil {
		fmt.Printf("ShowFrontends err:%s \n", err.Error())
	}
	fmt.Printf("ShowFrontends :%+v \n", frontends)

	// ShowBackends
	bes, err := db.ShowBackends()
	if err != nil {
		fmt.Printf("ShowBackends err:%s \n", err.Error())
	}
	fmt.Printf("ShowBackends :%+v \n", bes)

	// DropObserver
	arr := []Frontend{
		Frontend{Host: "doriscluster-sample-fe-1.doriscluster-sample-fe-internal.doris.svc.cluster.local", EditLogPort: 9010},
		Frontend{Host: "doriscluster-sample-fe-2.doriscluster-sample-fe-internal.doris.svc.cluster.local", EditLogPort: 9010},
	}

	db.DropObserver(arr)

	bes, err = db.ShowBackends()
	if err != nil {
		fmt.Printf("ShowBackends err:%s \n", err.Error())
	}
	fmt.Printf("ShowBackends after drop %+v \n", bes)

	// DecommissionBE
	arr1 := []Backend{
		Backend{Host: "doriscluster-sample-be-3.doriscluster-sample-be-internal.doris.svc.cluster.local", HeartbeatPort: 9050},
		Backend{Host: "doriscluster-sample-be-4.doriscluster-sample-be-internal.doris.svc.cluster.local", HeartbeatPort: 9050},
	}
	db.DecommissionBE(arr1)
	for i := 0; i < 20000; i++ {
		finished, err := db.CheckDecommissionBE(arr1)
		fmt.Printf("DecommissionBE check %d : is_finished=%t } \n", i, finished)
		if err != nil {
			fmt.Printf("DecommissionBEcheck err:%s \n", err.Error())
		}
		if finished {
			fmt.Printf("DecommissionBE finished")
			break
		}
		time.Sleep(500 * time.Millisecond)

	}

	bes, err = db.ShowBackends()
	if err != nil {
		fmt.Printf("ShowBackends err: %s \n", err.Error())
	}
	fmt.Printf("ShowBackends after decommission%+v \n", bes)

}
