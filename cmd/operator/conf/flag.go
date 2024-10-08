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

package conf

import (
	"flag"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// definate the start options.
type Flag struct {
	MetricsAddr          string
	ProbeAddr            string
	Namespace            string
	EnableLeaderElection bool
	PrintVar             bool
	EnableWebhook        bool
	Opts                 zap.Options
}

func ParseFlags() *Flag {
	f := Flag{}
	flag.StringVar(&f.MetricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&f.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&f.Namespace, "namespace", v12.NamespaceAll, "The namespace to watch for changes.")
	flag.BoolVar(&f.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&f.PrintVar, "version", false, "Prints current version.")

	// check switch unnamedwatches on or off, if 'true' passed from console or config in env, will start unnamedwatches operator.
	flag.BoolVar(&f.EnableWebhook, "enable-unnamedwatches", true, "start the unnamedwatches.")
	f.Opts = zap.Options{
		Development: true,
	}
	f.Opts.BindFlags(flag.CommandLine)
	flag.Parse()
	return &f
}
