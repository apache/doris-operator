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

package sub_controller

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func configRestartFixture(t *testing.T) (*SubDefaultController, client.Client, *dorisv1.DorisCluster, *corev1.ConfigMap) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "be-config", Namespace: "test"}, Data: map[string]string{"be.conf": "component_marker=be\nlog_dir=${DORIS_HOME}/log\nheartbeat_service_port=9050\n"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	d := &SubDefaultController{K8sclient: c, K8srecorder: record.NewFakeRecorder(20)}
	cluster := &dorisv1.DorisCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}, Spec: dorisv1.DorisClusterSpec{BeSpec: &dorisv1.BeSpec{BaseSpec: dorisv1.BaseSpec{ConfigMapInfo: dorisv1.ConfigMapInfo{ConfigMapName: cm.Name}}}}, Status: dorisv1.DorisClusterStatus{BEStatus: &dorisv1.ComponentStatus{ComponentCondition: dorisv1.ComponentCondition{Phase: dorisv1.Available}}}}
	cluster.Status.BEStatus.CoreConfigMapHashValue = d.BuildCoreConfigmapStatusHash(context.Background(), cluster, dorisv1.Component_BE)
	if cluster.Status.BEStatus.CoreConfigMapHashValue == "" {
		t.Fatal("missing baseline hash")
	}
	return d, c, cluster, cm
}

func TestUnchangedConfigDoesNotRestartDuringConcurrentDdcParsing(t *testing.T) {
	t.Setenv("DORIS_HOME", "/operator-home")
	d, _, cluster, _ := configRestartFixture(t)
	oldStatus := *cluster.Status.BEStatus.DeepCopy()
	var wg sync.WaitGroup
	errors := make(chan error, 4)
	start := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 300; i++ {
			current := cluster.DeepCopy()
			d.CompareConfigmapAndTriggerRestart(current, oldStatus, dorisv1.Component_BE)
			if current.Annotations[dorisv1.BERestartAt] != "" || current.Status.BEStatus.ComponentCondition.Phase != dorisv1.Available {
				errors <- fmt.Errorf("unchanged BE configuration triggered restart at iteration %d", i)
				return
			}
			if got := d.BuildCoreConfigmapStatusHash(context.Background(), current, dorisv1.Component_BE); got != oldStatus.CoreConfigMapHashValue {
				errors <- fmt.Errorf("unchanged BE configuration changed hash at iteration %d: %s", i, got)
				return
			}
		}
	}()
	for _, key := range []string{resource.FE_RESOLVEKEY, resource.BE_RESOLVEKEY, resource.MS_RESOLVEKEY} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			<-start
			ddc := &DisaggregatedSubDefaultController{}
			var firstError error
			for i := 0; i < 300; i++ {
				settings := ddc.resolveStartConfig([]byte("component_marker="+key+"\nlog_dir=${DORIS_HOME}/log\n"), key)
				if settings["component_marker"] != key && firstError == nil {
					firstError = fmt.Errorf("DDC %s read another component's configuration: %v", key, settings)
				}
			}
			if firstError != nil {
				errors <- firstError
			}
		}(key)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if got := os.Getenv("DORIS_HOME"); got != "/operator-home" {
		t.Errorf("process home changed to %q", got)
	}
}

func TestChangedConfigStillTriggersRestart(t *testing.T) {
	d, c, cluster, cm := configRestartFixture(t)
	oldStatus := *cluster.Status.BEStatus.DeepCopy()
	cm.Data["be.conf"] = "component_marker=be\nheartbeat_service_port=9150\n"
	if err := c.Update(context.Background(), cm); err != nil {
		t.Fatal(err)
	}
	d.CompareConfigmapAndTriggerRestart(cluster, oldStatus, dorisv1.Component_BE)
	if cluster.Annotations[dorisv1.BERestartAt] == "" || cluster.Status.BEStatus.ComponentCondition.Phase != dorisv1.Restarting {
		t.Fatal("real configuration change did not trigger restart")
	}
}

func TestInvalidConfigDoesNotTriggerRestart(t *testing.T) {
	d, c, cluster, cm := configRestartFixture(t)
	oldStatus := *cluster.Status.BEStatus.DeepCopy()
	cm.Data["be.conf"] = "bad=${bad}"
	if err := c.Update(context.Background(), cm); err != nil {
		t.Fatal(err)
	}
	d.CompareConfigmapAndTriggerRestart(cluster, oldStatus, dorisv1.Component_BE)
	if cluster.Annotations[dorisv1.BERestartAt] != "" || cluster.Status.BEStatus.ComponentCondition.Phase != dorisv1.Available {
		t.Fatal("invalid configuration triggered restart")
	}
	if hash := d.BuildCoreConfigmapStatusHash(context.Background(), cluster, dorisv1.Component_BE); hash != "" {
		t.Errorf("invalid configuration produced usable hash %q", hash)
	}
}

func TestDdcConfigParsingPreservesLocalExpansionAndEnvironmentPolicy(t *testing.T) {
	t.Setenv("DORIS_HOME", "/operator-home")
	t.Setenv("QUERY_PORT", "9040")
	d := &DisaggregatedSubDefaultController{}
	for _, tt := range []struct{ key, home string }{{resource.FE_RESOLVEKEY, "/opt/apache-doris/fe"}, {resource.BE_RESOLVEKEY, "/opt/apache-doris/be"}, {resource.MS_RESOLVEKEY, "/opt/apache-doris/ms"}} {
		values := d.resolveStartConfig([]byte("query_port=9030\nlog_dir=${DORIS_HOME}/log\n"), tt.key)
		if values["query_port"] != "9030" || values["log_dir"] != tt.home+"/log" {
			t.Errorf("%s values=%v", tt.key, values)
		}
		if _, ok := values["doris_home"]; ok {
			t.Error("default home leaked into parsed configuration")
		}
	}
	values := d.resolveStartConfig([]byte("DORIS_HOME=/custom\nlog_dir=${DORIS_HOME}/log\n"), resource.FE_RESOLVEKEY)
	if values["doris_home"] != "/custom" || values["log_dir"] != "/custom/log" {
		t.Errorf("explicit home changed: %v", values)
	}
	if values := d.resolveStartConfig([]byte("bad=${bad}"), resource.MS_RESOLVEKEY); values != nil {
		t.Errorf("invalid DDC configuration accepted: %v", values)
	}
	if got := os.Getenv("DORIS_HOME"); got != "/operator-home" {
		t.Errorf("process home changed to %q", got)
	}
}
