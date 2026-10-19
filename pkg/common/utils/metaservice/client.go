// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package metaservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type NodeStatus string

const (
	Running         NodeStatus = "NODE_STATUS_RUNNING"
	Decommissioning NodeStatus = "NODE_STATUS_DECOMMISSIONING"
	Decommissioned  NodeStatus = "NODE_STATUS_DECOMMISSIONED"
)

// Node is the subset of NodeInfoPB needed for compute group scale-down.
type Node struct {
	CloudUniqueID string     `json:"cloud_unique_id"`
	Host          string     `json:"host"`
	IP            string     `json:"ip"`
	HeartbeatPort int        `json:"heartbeat_port"`
	Status        NodeStatus `json:"status"`
}

// UnmarshalJSON accepts protobuf enum names and their numeric representation.
func (s *NodeStatus) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		*s = NodeStatus(name)
		return nil
	}
	var number int
	if err := json.Unmarshal(data, &number); err != nil {
		return errors.New("invalid MS node status")
	}
	switch number {
	case 1:
		*s = Running
	case 3:
		*s = Decommissioning
	case 4:
		*s = Decommissioned
	default:
		*s = NodeStatus(fmt.Sprintf("UNKNOWN_%d", number))
	}
	return nil
}

type Cluster struct {
	ID    string `json:"cluster_id"`
	Nodes []Node `json:"nodes"`
}

type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

func NewClient(endpoint, token string) *Client {
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	return &Client{endpoint: strings.TrimRight(endpoint, "/"), token: token,
		httpClient: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}}
}

// GetCluster uses the administrative HTTP API shared by SelectDB Cloud 3.x and
// 4.x. With cluster_id specified, result is a single ClusterPB, not a list.
func (c *Client) GetCluster(ctx context.Context, instanceID, cloudUniqueID, clusterID string) (*Cluster, error) {
	if cloudUniqueID == "" || clusterID == "" {
		return nil, errors.New("MS cluster query requires cloud_unique_id and cluster_id")
	}
	body, err := json.Marshal(map[string]string{"instance_id": instanceID, "cloud_unique_id": cloudUniqueID, "cluster_id": clusterID})
	if err != nil {
		return nil, err
	}
	var cluster Cluster
	if err := c.query(ctx, "get_cluster", nil, body, &cluster); err != nil {
		return nil, err
	}
	if cluster.ID != clusterID {
		return nil, errors.New("MS cluster response is missing the requested cluster")
	}
	return &cluster, nil
}

// GetInstanceID resolves legacy cloud_unique_id values instead of assuming that
// older deployments use the operator's cluster hash as their MS instance ID.
func (c *Client) GetInstanceID(ctx context.Context, cloudUniqueID string) (string, error) {
	if cloudUniqueID == "" {
		return "", errors.New("MS instance query requires cloud_unique_id")
	}
	var instance struct {
		ID string `json:"instance_id"`
	}
	if err := c.query(ctx, "get_instance", url.Values{"cloud_unique_id": {cloudUniqueID}}, nil, &instance); err != nil {
		return "", err
	}
	if instance.ID == "" {
		return "", errors.New("MS instance response is missing instance_id")
	}
	return instance.ID, nil
}

func (c *Client) query(ctx context.Context, path string, query url.Values, body []byte, result interface{}) error {
	endpoint, err := url.Parse(c.endpoint + "/MetaService/http/" + path)
	if err != nil || endpoint.Host == "" {
		return errors.New("invalid MS endpoint")
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("token", c.token)
	endpoint.RawQuery = query.Encode()
	method := http.MethodPost
	if body == nil {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return errors.New("cannot construct MS query")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// net/http errors include the request URL, which contains the MS token.
		return errors.New("MS query failed (network error or timeout)")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MS query returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Code   string          `json:"code"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&envelope); err != nil {
		return errors.New("invalid MS response")
	}
	if envelope.Code != "OK" {
		return errors.New("MS query did not return OK")
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return errors.New("MS response is missing result")
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return errors.New("invalid MS result")
	}
	return nil
}
