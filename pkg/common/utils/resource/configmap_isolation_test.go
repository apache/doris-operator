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

package resource

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"

	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

func TestResolveConfigMapsConcurrentIsolation(t *testing.T) {
	t.Setenv("DORIS_HOME", "/operator-home")
	var wg sync.WaitGroup
	errors := make(chan error, 4)
	start := make(chan struct{})
	for _, component := range []dorisv1.ComponentType{dorisv1.Component_FE, dorisv1.Component_BE, dorisv1.Component_CN, dorisv1.Component_Broker} {
		wg.Add(1)
		go func(component dorisv1.ComponentType) {
			defer wg.Done()
			<-start
			cm := &corev1.ConfigMap{Data: map[string]string{getDefaultResolveKey(component): "component_marker=" + string(component) + "\nlog_dir=${DORIS_HOME}/log\n"}}
			for i := 0; i < 200; i++ {
				settings, err := ResolveConfigMaps([]*corev1.ConfigMap{cm}, component)
				if err != nil || settings["component_marker"] != string(component) || settings["log_dir"] != getDefaultDorisHome(component)+"/log" {
					errors <- fmt.Errorf("%s parse %d: settings=%v error=%v", component, i, settings, err)
					return
				}
				if _, ok := settings["doris_home"]; ok {
					errors <- fmt.Errorf("injected home leaked into config: %v", settings)
					return
				}
			}
		}(component)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if got := os.Getenv("DORIS_HOME"); got != "/operator-home" {
		t.Errorf("changed process DORIS_HOME to %q", got)
	}
}

func TestResolveConfigMapsPreservesPropertiesSemantics(t *testing.T) {
	t.Setenv("DORIS_HOME", "/operator-home")
	t.Setenv("QUERY_PORT", "9040")
	cm := &corev1.ConfigMap{Data: map[string]string{"fe.conf": `# configuration comment
DORIS_HOME=/custom-fe
LOG_DIR=${DORIS_HOME}/log
meta_dir=${LOG_DIR}/meta
query_port=9030
literal_shell=$DORIS_HOME/log
java_opts="-Xmx1g -Dpath=${LOG_DIR}"
escaped_path=a\:b
dotted.key=value
query_port=9031
`}}
	settings, err := ResolveConfigMaps([]*corev1.ConfigMap{cm}, dorisv1.Component_FE)
	if err != nil {
		t.Fatal(err)
	}
	// Native parsing historically overrides declared keys from the environment,
	// including its per-component DORIS_HOME, while properties expand file values.
	expected := map[string]interface{}{
		"doris_home": "/opt/apache-doris/fe", "log_dir": "/custom-fe/log", "meta_dir": "/custom-fe/log/meta",
		"query_port": "9040", "literal_shell": "$DORIS_HOME/log", "java_opts": `"-Xmx1g -Dpath=/custom-fe/log"`,
		"escaped_path": "a:b", "dotted": map[string]interface{}{"key": "value"},
	}
	if !reflect.DeepEqual(settings, expected) {
		t.Errorf("settings=%#v, expected=%#v", settings, expected)
	}
	if got := os.Getenv("DORIS_HOME"); got != "/operator-home" {
		t.Errorf("changed process home: %q", got)
	}
}

func TestResolveConfigMapsRejectsInvalidProperties(t *testing.T) {
	for _, value := range []string{`bad=${bad}`, `bad=${unfinished`, `bad=\uZZZZ`} {
		t.Run(value, func(t *testing.T) {
			settings, err := ResolveConfigMaps([]*corev1.ConfigMap{{Data: map[string]string{"be.conf": value}}}, dorisv1.Component_BE)
			if err == nil || settings != nil {
				t.Errorf("invalid properties accepted: settings=%v error=%v", settings, err)
			}
		})
	}
}

func TestResolveConfigMapsDoesNotUseGlobalViperOverrides(t *testing.T) {
	viper.Set("query_port", "9999")
	t.Cleanup(viper.Reset)
	settings, err := ResolveConfigMaps([]*corev1.ConfigMap{{Data: map[string]string{"fe.conf": "query_port=9030"}}}, dorisv1.Component_FE)
	if err != nil {
		t.Fatal(err)
	}
	if settings["query_port"] != "9030" {
		t.Errorf("global override leaked into component config: %v", settings)
	}
	if viper.GetString("query_port") != "9999" {
		t.Error("component parser modified global Viper")
	}
}
