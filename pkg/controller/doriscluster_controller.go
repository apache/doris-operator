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

package controller

import (
	"context"
	dorisv1 "github.com/apache/doris-operator/api/doris/v1"
	"github.com/apache/doris-operator/pkg/common/utils/k8s"
	"github.com/apache/doris-operator/pkg/common/utils/resource"
	"github.com/apache/doris-operator/pkg/controller/sub_controller"
	"github.com/apache/doris-operator/pkg/controller/sub_controller/be"
	bk "github.com/apache/doris-operator/pkg/controller/sub_controller/broker"
	cn "github.com/apache/doris-operator/pkg/controller/sub_controller/cn"
	"github.com/apache/doris-operator/pkg/controller/sub_controller/fe"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"os"
	controller_builder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	name                 = "doris-cluster-controller"
	feControllerName     = "fe-controller"
	cnControllerName     = "cn-controller"
	beControllerName     = "be-controller"
	brokerControllerName = "broker-controller"
)

// DorisClusterReconciler reconciles a DorisCluster object
type DorisClusterReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Scs      map[string]sub_controller.SubController
	//record configmap response instance. key: configMap namespacedName, value: DorisCluster namespacedName
	WatchConfigMaps map[string]string
}

var (
	_ reconcile.Reconciler = &DorisClusterReconciler{}

	_ Controller = &DorisClusterReconciler{}
)

//+kubebuilder:rbac:groups=doris.selectdb.com,resources=dorisclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=doris.selectdb.com,resources=dorisclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=doris.selectdb.com,resources=dorisclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="core",resources=endpoints,verbs=get;watch;list
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;update;watch
//+kubebuilder:rbac:groups=admissionregistration,resources=validatingwebhookconfigurations,verbs=get;list;update;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DorisCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *DorisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.FromContext(ctx)
	klog.Info("DorisClusterReconciler reconcile the update crd name ", req.Name, " namespace ", req.Namespace)
	var edcr dorisv1.DorisCluster
	err := r.Client.Get(ctx, req.NamespacedName, &edcr)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error(err, " the req kind is not exists ", req.NamespacedName, " name ", req.Name)
		return requeueIfError(err)
	}

	dcr := edcr.DeepCopy()

	if !dcr.DeletionTimestamp.IsZero() {
		r.resourceClean(ctx, dcr)
		return ctrl.Result{}, nil
	}

	if dcr.Spec.EnableRestartWhenConfigChange {
		coreConfigMaps := resource.GetDorisCoreConfigMapNames(dcr)
		for componentType := range coreConfigMaps {
			cmnn := types.NamespacedName{Namespace: dcr.Namespace, Name: coreConfigMaps[componentType]}
			dcrnn := types.NamespacedName{Namespace: dcr.Namespace, Name: dcr.Name}
			r.WatchConfigMaps[cmnn.String()] = dcrnn.String()
		}
	}

	//subControllers reconcile for create or update sub resource.
	for _, rc := range r.Scs {
		if err := rc.Sync(ctx, dcr); err != nil {
			klog.Error("DorisClusterReconciler reconcile ", " sub resource reconcile failed ", "namespace: ", dcr.Namespace, " name: ", dcr.Name, " controller: ", rc.GetControllerName(), " error: ", err)
			return requeueIfError(err)
		}
	}

	//generate the dcr status.
	r.clearNoEffectResources(ctx, dcr)
	for _, rc := range r.Scs {
		//update component status.

		if err := rc.UpdateComponentStatus(dcr); err != nil {
			klog.Errorf("DorisClusterReconciler reconcile update component %s status failed.err=%s\n", rc.GetControllerName(), err.Error())
			return requeueIfError(err)
		}
	}

	//if dcr has updated by doris operator, should update it in apiserver. if not ignore it.
	if err = r.revertDorisClusterSomeFields(ctx, &edcr, dcr); err != nil {
		klog.Errorf("DorisClusterReconciler updateDorisClusterToOld update dorisCluster namespace=%s, name=%s failed, err=%s", dcr.Namespace, dcr.Name, err.Error())
		return requeueIfError(err)
	}

	return r.updateDorisClusterStatus(ctx, dcr)
}

// if cluster spec be reverted, doris operator should revert to old.
// this action is not good, but this will be a good shield for scale down of fe.
func (r *DorisClusterReconciler) revertDorisClusterSomeFields(ctx context.Context, getDcr, updatedDcr *dorisv1.DorisCluster) error {
	if *getDcr.Spec.FeSpec.Replicas != *updatedDcr.Spec.FeSpec.Replicas {
		return k8s.ApplyDorisCluster(ctx, r.Client, updatedDcr)
	}

	return nil
}

func (r *DorisClusterReconciler) updateDorisCluster(ctx context.Context, dcr *dorisv1.DorisCluster) error {
	return k8s.ApplyDorisCluster(ctx, r.Client, dcr)
}

func (r *DorisClusterReconciler) clearNoEffectResources(context context.Context, cluster *dorisv1.DorisCluster) {
	//calculate the status of doris cluster by subresource's status.
	//clear resources when sub resource deleted. example: deployed fe,be,cn, when cn spec is deleted we should delete cn resources.
	for _, rc := range r.Scs {
		rc.ClearResources(context, cluster)
	}

	return
}

func (r *DorisClusterReconciler) updateDorisClusterStatus(ctx context.Context, dcr *dorisv1.DorisCluster) (ctrl.Result, error) {
	var edcr dorisv1.DorisCluster
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: dcr.Namespace, Name: dcr.Name}, &edcr); err != nil {
		return ctrl.Result{}, err
	}

	// if the status is not equal before reconcile and now the status is not available we should requeue.
	if !inconsistentStatus(&dcr.Status, &edcr) {
		if r.reconcile(dcr) {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	dcr.Status.DeepCopyInto(&edcr.Status)
	return ctrl.Result{}, retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Client.Status().Update(ctx, &edcr)
	})
}

func (r *DorisClusterReconciler) reconcile(dcr *dorisv1.DorisCluster) bool {
	if dcr.Spec.FeSpec != nil {
		if dcr.Status.FEStatus.ComponentCondition.Phase != dorisv1.Available {
			return true
		}
	}

	if dcr.Spec.BeSpec != nil {
		if dcr.Status.BEStatus.ComponentCondition.Phase != dorisv1.Available {
			return true
		}
	}

	if dcr.Spec.CnSpec != nil {
		if dcr.Status.CnStatus.ComponentCondition.Phase != dorisv1.Available {
			return true
		}
	}

	if dcr.Spec.BrokerSpec != nil {
		if dcr.Status.BrokerStatus.ComponentCondition.Phase != dorisv1.Available {
			return true
		}
	}

	return false
}

// clean all resource deploy by DorisCluster
func (r *DorisClusterReconciler) resourceClean(ctx context.Context, dcr *dorisv1.DorisCluster) {
	for _, rc := range r.Scs {
		rc.ClearResources(ctx, dcr)
	}

	return
}

func (r *DorisClusterReconciler) resourceBuilder(builder *ctrl.Builder) *ctrl.Builder {
	return builder.For(&dorisv1.DorisCluster{}).
		Owns(&appv1.StatefulSet{}).
		Owns(&corev1.Service{})
}

func (r *DorisClusterReconciler) watchPodBuilder(builder *ctrl.Builder) *ctrl.Builder {
	mapFn := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, a client.Object) []reconcile.Request {
			labels := a.GetLabels()
			dorisName := labels[dorisv1.DorisClusterLabelKey]
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
			if _, ok := e.Object.GetLabels()[dorisv1.DorisClusterLabelKey]; !ok {
				return false
			}

			return true
		},
		UpdateFunc: func(u event.UpdateEvent) bool {
			if _, ok := u.ObjectOld.GetLabels()[dorisv1.DorisClusterLabelKey]; !ok {
				return false
			}

			return u.ObjectOld != u.ObjectNew
		},
		DeleteFunc: func(d event.DeleteEvent) bool {
			if _, ok := d.Object.GetLabels()[dorisv1.DorisClusterLabelKey]; !ok {
				return false
			}

			return true
		},
	}

	return builder.Watches(&corev1.Pod{},
		mapFn, controller_builder.WithPredicates(p))
}

func (r *DorisClusterReconciler) watchConfigMapBuilder(builder *ctrl.Builder) *ctrl.Builder {
	mapFn := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, a client.Object) []reconcile.Request {
			cmnn := types.NamespacedName{Namespace: a.GetNamespace(), Name: a.GetName()}
			if dcrNamespacedNameStr, ok := r.WatchConfigMaps[cmnn.String()]; ok {
				// nna[0] is namespace, nna[1] is dcrName,
				nna := strings.Split(dcrNamespacedNameStr, "/")
				// not run only for code standard
				if len(nna) != 2 {
					return nil
				}

				return []reconcile.Request{{NamespacedName: types.NamespacedName{
					Namespace: nna[0],
					Name:      nna[1],
				}}}
			}
			return nil
		})

	p := predicate.Funcs{
		CreateFunc: func(u event.CreateEvent) bool {

			cmnn := types.NamespacedName{Namespace: u.Object.GetNamespace(), Name: u.Object.GetName()}
			_, ok := r.WatchConfigMaps[cmnn.String()]
			return ok
		},

		UpdateFunc: func(u event.UpdateEvent) bool {

			cmnn := types.NamespacedName{Namespace: u.ObjectNew.GetNamespace(), Name: u.ObjectNew.GetName()}

			if _, ok := r.WatchConfigMaps[cmnn.String()]; !ok {
				return false
			}

			return true
		},
	}

	return builder.Watches(&corev1.ConfigMap{},
		mapFn, controller_builder.WithPredicates(p))
}

// SetupWithManager sets up the controller with the Manager.
func (r *DorisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := r.resourceBuilder(ctrl.NewControllerManagedBy(mgr))
	builder = r.watchPodBuilder(builder)
	builder = r.watchConfigMapBuilder(builder)
	return builder.Complete(r)
}

// Init initial the DorisClusterReconciler for reconcile.
func (r *DorisClusterReconciler) Init(mgr ctrl.Manager, options *Options) {
	subcs := make(map[string]sub_controller.SubController)
	fc := fe.New(mgr.GetClient(), mgr.GetEventRecorderFor(feControllerName))
	subcs[feControllerName] = fc
	be := be.New(mgr.GetClient(), mgr.GetEventRecorderFor(beControllerName))
	subcs[beControllerName] = be
	cn := cn.New(mgr.GetClient(), mgr.GetEventRecorderFor(cnControllerName))
	subcs[cnControllerName] = cn
	brk := bk.New(mgr.GetClient(), mgr.GetEventRecorderFor(brokerControllerName))
	subcs[brokerControllerName] = brk

	if err := (&DorisClusterReconciler{
		Client:          mgr.GetClient(),
		Recorder:        mgr.GetEventRecorderFor(name),
		Scs:             subcs,
		WatchConfigMaps: make(map[string]string),
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, " unable to create controller ", "controller ", "DorisCluster ")
		os.Exit(1)
	}
	klog.Infof("dorisclusterreconcile %t", options.EnableWebHook)
	if options.EnableWebHook {
		if err := (&dorisv1.DorisCluster{}).SetupWebhookWithManager(mgr); err != nil {
			klog.Error(err, " unable to create unnamedwatches ", " controller ", " DorisCluster ")
			os.Exit(1)
		}
	}
}
