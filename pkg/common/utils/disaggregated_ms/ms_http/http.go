package ms_http

import (
	"bytes"
	"fmt"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_meta"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
)

const (
	CREATE_INSTANCE_PREFIX_TEMPLATE = `%s/MetaService/http/create_instance?token=%s`
	DELETE_INSTANCE_PREFIX_TEMPLATE = `%s/MetaService/http/drop_instance?token=%s`
	GET_INSTANCE_PREFIX_TEMPLATE    = `%s/MetaService/http/get_instance?token=%s&instance_id=%s`
)

//realize the metaservice interface
//https://doris.apache.org/zh-CN/docs/dev/separation-of-storage-and-compute/meta-service-resource-http-api#%E5%88%9B%E5%BB%BA-instance

func GetInstance(endpoint, token, instanceId string) (*MSResponse, error) {
	addr := fmt.Sprintf(GET_INSTANCE_PREFIX_TEMPLATE, endpoint, token, instanceId)
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	mr := &MSResponse{}
	if err := json.Unmarshal(body, mr); err != nil {
		return nil, err
	}
	return mr, nil
}

func DeleteInstance(endpoint, token, instanceId string) (*MSResponse, error) {
	addr := fmt.Sprintf(DELETE_INSTANCE_PREFIX_TEMPLATE, endpoint, token)
	delReq := map[string]string{}
	delReq[ms_meta.Instance_id] = instanceId
	delReqBytes, err := json.Marshal(delReq)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(delReqBytes)
	req, err := http.NewRequest("DELETE", addr, r)
	if err != nil {
		return nil, err
	}
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	mr := &MSResponse{}
	if err = json.Unmarshal(body, mr); err != nil {
		return mr, err
	}

	return mr, nil
}

func CreateInstance(endpoint, token string, instanceInfo []byte) (*MSResponse, error) {
	addr := fmt.Sprintf(CREATE_INSTANCE_PREFIX_TEMPLATE, endpoint, token)
	r := bytes.NewReader(instanceInfo)
	req, err := http.NewRequest("PUT", addr, r)
	if err != nil {
		return nil, err
	}

	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	mr := &MSResponse{}
	if err := json.Unmarshal(body, mr); err != nil {
		return mr, err
	}

	return mr, nil
}
