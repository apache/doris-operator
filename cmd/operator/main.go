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

/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"github.com/FoundationDB/fdb-kubernetes-operator/api/v1beta2"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	dmsv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/cmd/operator/conf"
	"github.com/selectdb/doris-operator/pkg/common/utils/certificate"
	"github.com/selectdb/doris-operator/pkg/controller"
	"github.com/selectdb/doris-operator/pkg/controller/unnamedwatches"
	"io"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	printVar bool
)

var (
	VERSION    string
	GOVERSION  string
	COMMIT     string
	BUILD_DATE string
)

// Print version information to a given out writer.
func Print(out io.Writer) {
	if printVar {
		fmt.Fprint(out, "version="+VERSION+"\ncommit="+COMMIT+"\nbuild_date="+BUILD_DATE+"\n")
	}
}

// initial all controllers for reconciling resource.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(dorisv1.AddToScheme(scheme))
	utilruntime.Must(dv1.AddToScheme(scheme))
	utilruntime.Must(dmsv1.AddToScheme(scheme))
	//add foundationdb scheme
	utilruntime.Must(v1beta2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	controller.Controllers = append(controller.Controllers, &controller.DorisClusterReconciler{}, &unnamedwatches.WResource{})
	start := os.Getenv("START_DISAGGREGATED_OPERATOR")
	if start == "true" {
		controller.Controllers = append(controller.Controllers, &controller.DisaggregatedClusterReconciler{})
	}
}

func main() {
	//parse then parameters from console.
	f := conf.ParseFlags()
	//parse env
	envs := conf.ParseEnvs()
	//print version infos.
	printVersionInfos(f.PrintVar)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&f.Opts)))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     f.MetricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: f.ProbeAddr,
		Namespace:              f.Namespace,
		LeaderElection:         f.EnableLeaderElection,
		LeaderElectionID:       "e1370669.selectdb.com",
		//if one reconcile failed, others will not be affected.
		Controller: v1alpha1.ControllerConfigurationSpec{
			RecoverPanic: pointer.Bool(true),
		},

		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	options := conf.NewControllerOptions(envs)
	//initial all controllers
	for _, c := range controller.Controllers {
		c.Init(mgr, options)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// enable webhook, check webhook certificate
	if options.EnableWebHook {
		//wait for the secret have
		interval := time.Second * 1
		timeout := time.Second * 30
		keyPath := filepath.Join(mgr.GetWebhookServer().CertDir, certificate.TLsCertName)
		err = wait.PollImmediate(interval, timeout, func() (bool, error) {
			_, err := os.Stat(keyPath)
			if os.IsNotExist(err) {
				setupLog.Info("webhook certificate have not present waiting kubelet update", "file", keyPath)
				return false, nil
			} else if err != nil {
				setupLog.Info("check webhook certificate ", "path", keyPath, "err=", err.Error())
				return false, err
			}

			setupLog.Info("webhook certificate file exit.")
			return true, nil
		})

		if err != nil {
			setupLog.Error(err, "check webhook certificate failed")
			os.Exit(1)
		}
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// print version information of now operator.
func printVersionInfos(print bool) {
	if print {
		Print(os.Stdout)
		os.Exit(0)
	}
}
