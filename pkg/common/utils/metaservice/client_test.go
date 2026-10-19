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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetCluster(t *testing.T) {
	for _, state := range []string{`"NODE_STATUS_DECOMMISSIONING"`, `3`} {
		t.Run(state, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/MetaService/http/get_cluster" || r.URL.Query().Get("token") != "a+b&c" {
					t.Errorf("unexpected MS request: method=%s path=%s", r.Method, r.URL.Path)
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["instance_id"] != "instance" || body["cloud_unique_id"] != "1:instance:operator" || body["cluster_id"] != "cg" {
					t.Errorf("body=%v", body)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"code":"OK","result":{"cluster_id":"cg","nodes":[{"cloud_unique_id":"be-id","host":"doris-cg-1.example","heartbeat_port":9050,"status":` + state + `}]}}`))
			}))
			defer server.Close()
			cluster, err := NewClient(server.URL, "a+b&c").GetCluster(context.Background(), "instance", "1:instance:operator", "cg")
			if err != nil {
				t.Fatal(err)
			}
			if cluster.ID != "cg" || len(cluster.Nodes) != 1 || cluster.Nodes[0].Status != Decommissioning {
				t.Fatalf("unexpected cluster: %+v", cluster)
			}
		})
	}
}

func TestGetClusterRejectsUncertainResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"http failure", 503, `unavailable`},
		{"not found is not absence", 404, `{"code":"NOT_FOUND"}`},
		{"business failure", 200, `{"code":"INTERNAL_ERROR","result":{"cluster_id":"cg"}}`},
		{"missing code", 200, `{"result":{"cluster_id":"cg"}}`},
		{"missing result", 200, `{"code":"OK"}`},
		{"wrong cluster", 200, `{"code":"OK","result":{"cluster_id":"other"}}`},
		{"malformed", 200, `{"code":`},
		{"wrong result shape", 200, `{"code":"OK","result":{"cluster":[{"cluster_id":"cg"}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status); w.Write([]byte(tt.body)) }))
			defer server.Close()
			_, err := NewClient(server.URL, "sensitive-token").GetCluster(context.Background(), "instance", "id", "cg")
			if err == nil {
				t.Fatal("uncertain response must fail closed")
			}
			if strings.Contains(err.Error(), "sensitive-token") {
				t.Fatal("error leaked token")
			}
		})
	}
}

func TestGetClusterEmptyNodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":"OK","result":{"cluster_id":"cg"}}`))
	}))
	defer server.Close()
	cluster, err := NewClient(server.URL, "token").GetCluster(context.Background(), "instance", "id", "cg")
	if err != nil || len(cluster.Nodes) != 0 {
		t.Fatalf("empty protobuf repeated field: cluster=%v err=%v", cluster, err)
	}
}

func TestGetClusterCancellationAndRedirectDoNotLeakToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.invalid", http.StatusFound)
	}))
	defer server.Close()
	client := NewClient(server.URL, "sensitive-token")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{ctx, context.Background()} {
		_, err := client.GetCluster(ctx, "instance", "id", "cg")
		if err == nil || strings.Contains(err.Error(), "sensitive-token") {
			t.Fatalf("unsafe error: %v", err)
		}
	}
	client.httpClient.Timeout = time.Nanosecond
	_, err := client.GetCluster(context.Background(), "instance", "id", "cg")
	if err == nil || strings.Contains(err.Error(), "sensitive-token") {
		t.Fatalf("unsafe timeout error: %v", err)
	}
}

func TestGetInstanceIDForLegacyCloudUniqueID(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		wantErr    bool
	}{
		{name: "resolved", body: `{"code":"OK","result":{"instance_id":"legacy-instance","obj_info":[{"sk":"not-returned"}]}}`},
		{name: "missing ID", body: `{"code":"OK","result":{}}`, wantErr: true},
		{name: "missing instance", body: `{"code":"NOT_FOUND"}`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/MetaService/http/get_instance" || r.URL.Query().Get("cloud_unique_id") != "opaque-legacy-id" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.Write([]byte(tt.body))
			}))
			defer server.Close()
			instanceID, err := NewClient(server.URL, "token").GetInstanceID(context.Background(), "opaque-legacy-id")
			if (err != nil) != tt.wantErr {
				t.Fatalf("instance=%s err=%v", instanceID, err)
			}
			if !tt.wantErr && instanceID != "legacy-instance" {
				t.Fatalf("instance=%s", instanceID)
			}
		})
	}
}
