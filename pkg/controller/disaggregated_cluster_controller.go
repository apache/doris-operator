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

package controller

import (
	"context"
	"errors"
	dv1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"github.com/apache/doris-operator/pkg/common/utils/hash"
	sc "github.com/apache/doris-operator/pkg/controller/sub_controller"
	dcgs "github.com/apache/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/computegroups"
	dfe "github.com/apache/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/disaggregated_fe"
	"github.com/apache/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/metaservice"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	controller_builder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

var (
	_ reconcile.Reconciler = &DisaggregatedClusterReconciler{}
	_ Controller           = &DisaggregatedClusterReconciler{}
)

var (
	disaggregatedClusterController = "disaggregatedClusterController"
)

type DisaggregatedClusterReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Scs      map[string]sc.DisaggregatedSubController
	//record configmap response instance. key: configMap namespacedName, value: DorisDisaggregatedCluster namespacedName
	//wcms map[string]string
}

func (dc *DisaggregatedClusterReconciler) Init(mgr ctrl.Manager, options *Options) {
	//wcms := make(map[string]string)
	scs := make(map[string]sc.DisaggregatedSubController)
	msc := metaservice.New(mgr)
	scs[msc.GetControllerName()] = msc

	dfec := dfe.New(mgr)
	scs[dfec.GetControllerName()] = dfec
	dccsc := dcgs.New(mgr)
	scs[dccsc.GetControllerName()] = dccsc

	if err := (&DisaggregatedClusterReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor(disaggregatedClusterController),
		Scs:      scs,
		//wcms:     wcms,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller ", "disaggregatedClusterReconciler")
		os.Exit(1)
	}

	if options.EnableWebHook {
		if err := (&dv1.DorisDisaggregatedCluster{}).SetupWebhookWithManager(mgr); err != nil {
			klog.Error(err, " unable to create unnamedwatches ", " controller ", " DorisDisaggregatedCluster ")
			os.Exit(1)
		}
	}
}

func (dc *DisaggregatedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := dc.resourceBuilder(ctrl.NewControllerManagedBy(mgr))
	builder = dc.watchPodBuilder(builder)
	//builder = dc.watchConfigMapBuilder(builder)
	return builder.Complete(dc)
}

func (dc *DisaggregatedClusterReconciler) watchPodBuilder(builder *ctrl.Builder) *ctrl.Builder {
	mapFn := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, a client.Object) []reconcile.Request {
			labels := a.GetLabels()
			dorisName := labels[dv1.DorisDisaggregatedClusterName]
			if dorisName != "" {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      dorisName,
						Namespace: a.GetNamespace(),
					}},
				}
			}

			return nil
		})

	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if _, ok := e.Object.GetLabels()[dv1.DorisDisaggregatedClusterName]; !ok {
				return false
			}

			return true
		},
		UpdateFunc: func(u event.UpdateEvent) bool {
			if _, ok := u.ObjectOld.GetLabels()[dv1.DorisDisaggregatedClusterName]; !ok {
				return false
			}

			return u.ObjectOld != u.ObjectNew
		},
		DeleteFunc: func(d event.DeleteEvent) bool {
			if _, ok := d.Object.GetLabels()[dv1.DorisDisaggregatedClusterName]; !ok {
				return false
			}

			return true
		},
	}

	return builder.Watches(&corev1.Pod{},
		mapFn, controller_builder.WithPredicates(p))
}

//func (dc *DisaggregatedClusterReconciler) watchConfigMapBuilder(builder *ctrl.Builder) *ctrl.Builder {
//	mapFn := handler.EnqueueRequestsFromMapFunc(
//		func(a client.Object) []reconcile.Request {
//			namespace := a.GetNamespace()
//			name := a.GetName()
//			cmnn := types.NamespacedName{Namespace: namespace, Name: name}
//			cmnnStr := cmnn.String()
//			if ddc, ok := dc.wcms[cmnnStr]; ok {
//				nna := strings.Split(ddc, "/")
//				// not run only for code standard
//				if len(nna) != 2 {
//					return nil
//				}
//
//				return []reconcile.Request{{NamespacedName: types.NamespacedName{
//					Namespace: nna[0],
//					Name:      nna[1],
//				}}}
//			}
//			return nil
//		})
//
//	p := predicate.Funcs{
//		UpdateFunc: func(u event.UpdateEvent) bool {
//			ns := u.ObjectNew.GetNamespace()
//			name := u.ObjectNew.GetName()
//			nsn := ns + "/" + name
//			_, ok := dc.wcms[nsn]
//			return ok
//		},
//	}
//
//	return builder.Watches(&source.Kind{Type: &corev1.ConfigMap{}},
//		mapFn, controller_builder.WithPredicates(p))
//}

func (dc *DisaggregatedClusterReconciler) resourceBuilder(builder *ctrl.Builder) *ctrl.Builder {
	return builder.For(&dv1.DorisDisaggregatedCluster{}).
		Owns(&appv1.StatefulSet{}).
		Owns(&corev1.Service{})
}

// Reconcile steps:
// 1. check and register instance info. info register in memory. periodical sync.
// 2. sync resource.
// 3. clear need delete resource.
// 4. display new status(eorganize status, update cr or status)
func (dc *DisaggregatedClusterReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var ddc dv1.DorisDisaggregatedCluster
	err := dc.Get(ctx, req.NamespacedName, &ddc)
	if apierrors.IsNotFound(err) {
		klog.Warningf("disaggreatedClusterReconciler not find resource DorisDisaggregatedCluster namespaceName %s", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	hv := hash.HashObject(ddc.Spec)

	var res ctrl.Result
	var msg string
	reconRes, reconErr := dc.reconcileSub(ctx, &ddc)
	if reconErr != nil {
		msg = msg + reconErr.Error()
	}
	if !reconRes.IsZero() {
		res = reconRes
	}

	// clear unused resources.
	clearRes, clearErr := dc.clearUnusedResources(ctx, &ddc)
	if clearErr != nil {
		msg = msg + reconErr.Error()
	}

	if !clearRes.IsZero() {
		res = clearRes
	}

	//display new status.
	disRes, disErr := func() (ctrl.Result, error) {
		//reorganize status.
		var stsRes ctrl.Result
		var stsErr error
		if stsRes, stsErr = dc.reorganizeStatus(&ddc); stsErr != nil {
			return stsRes, stsErr
		}

		//update cr or status
		if stsRes, stsErr = dc.updateObjectORStatus(ctx, &ddc, hv); stsErr != nil {
			return stsRes, stsErr
		}

		return stsRes, stsErr
	}()
	if disErr != nil {
		msg = msg + disErr.Error()
	}
	if !disRes.IsZero() {
		res = disRes
	}

	if msg != "" {
		return res, errors.New(msg)
	}
	return res, nil
}

func (dc *DisaggregatedClusterReconciler) clearUnusedResources(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	for _, subC := range dc.Scs {
		subC.ClearResources(ctx, ddc)
	}

	return ctrl.Result{}, nil
}

func (dc *DisaggregatedClusterReconciler) reorganizeStatus(ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	for _, sc := range dc.Scs {
		//update component status.
		if err := sc.UpdateComponentStatus(ddc); err != nil {
			klog.Errorf("DorisClusterReconciler reconcile update component %s status failed.err=%s\n", sc.GetControllerName(), err.Error())
			return requeueIfError(err)
		}
	}

	ddc.Status.ClusterHealth.Health = dv1.Green
	if ddc.Status.FEStatus.AvailableStatus != dv1.Available || ddc.Status.ClusterHealth.CGAvailableCount <= (ddc.Status.ClusterHealth.CGCount/2) {
		ddc.Status.ClusterHealth.Health = dv1.Red
	} else if ddc.Status.FEStatus.Phase != dv1.Ready || ddc.Status.ClusterHealth.CGAvailableCount < ddc.Status.ClusterHealth.CGCount {
		ddc.Status.ClusterHealth.Health = dv1.Yellow
	}
	return ctrl.Result{}, nil
}

func (dc *DisaggregatedClusterReconciler) reconcileSub(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	// recall all sub for check error.
	errs := []error{}
	for _, subC := range dc.Scs {
		if err := subC.Sync(ctx, ddc); err != nil {
			klog.Errorf("disaggreatedClusterReconciler sub reconciler %s sync err=%s.", subC.GetControllerName(), err.Error())
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		msg := ""
		for _, err := range errs {
			msg += err.Error()
		}
		return ctrl.Result{}, errors.New(msg)
	}
	return ctrl.Result{}, nil
}

// when spec revert by operator should update cr or directly update status.
func (dc *DisaggregatedClusterReconciler) updateObjectORStatus(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster, preHv string) (ctrl.Result, error) {
	postHv := hash.HashObject(ddc.Spec)
	deepCopyDDC := ddc.DeepCopy()
	if preHv != postHv {
		var eddc dv1.DorisDisaggregatedCluster
		if err := dc.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.Name}, &eddc); err == nil || !apierrors.IsNotFound(err) {
			if eddc.ResourceVersion != "" {
				ddc.ResourceVersion = eddc.ResourceVersion
			}
		}
		if err := dc.Update(ctx, ddc); err != nil {
			klog.Errorf("disaggreatedClusterReconciler update DorisDisaggregatedCluster namespace %s name %s  failed, err=%s", ddc.Namespace, ddc.Name, err.Error())
			//return ctrl.Result{}, err
		}
	}
	res, err := dc.updateDorisDisaggregatedClusterStatus(ctx, deepCopyDDC)

	if err != nil {
		return res, err
	}

	//if decommissioning, be is migrating data should wait it over, so return reconciling after 10 seconds.
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cgs.Phase == dv1.Decommissioning {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// If the cluster status is abnormal(Health is not Green), reconciling is required.
	if ddc.Status.ClusterHealth.Health != dv1.Green {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return res, nil

}

func (dc *DisaggregatedClusterReconciler) updateDorisDisaggregatedClusterStatus(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	var eddc dv1.DorisDisaggregatedCluster
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := dc.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.Name}, &eddc); err != nil {
			return err
		}
		ddc.Status.DeepCopyInto(&eddc.Status)
		return dc.Status().Update(ctx, &eddc)
	}); err != nil {
		klog.Errorf("updateDorisDisaggregatedClusterStatus update status failed err: %s", err.Error())
	}

	// if the status is not equal before reconcile and now the status is not available we should requeue.
	if !disAggregatedInconsistentStatus(&ddc.Status, &eddc) {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}
