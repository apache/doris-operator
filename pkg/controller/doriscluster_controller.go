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
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller/be"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller/fe"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"os"

	dorisv1 "github.com/selectdb/doris-operator/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	name             = "starrockscluster-controller"
	feControllerName = "fe-controller"
	cnControllerName = "cn-controller"
	beControllerName = "be-controller"
)

// DorisClusterReconciler reconciles a DorisCluster object
type DorisClusterReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Scs      map[string]sub_controller.SubController
}

//+kubebuilder:rbac:groups=doris.selectdb.com,resources=dorisclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=doris.selectdb.com,resources=dorisclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=doris.selectdb.com,resources=dorisclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="core",resources=endpoints,verbs=get;watch;list
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

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
	klog.Info("StarRocksClusterReconciler reconcile the update crd name ", req.Name, " namespace ", req.Namespace)
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

	//subControllers reconcile for create or update sub resource.
	for _, rc := range r.Scs {
		if err := rc.Sync(ctx, dcr); err != nil {
			klog.Error("DorisClusterReconciler reconcile ", " sub resource reconcile failed ", "namespace ", dcr.Namespace, " name ", dcr.Name, " controller ", rc.GetControllerName(), " faield ", err)
			return requeueIfError(err)
		}
	}

	//generate the src status.
	r.reconcileStatus(ctx, dcr)
	return ctrl.Result{}, r.UpdateStarRocksClusterStatus(ctx, dcr)
}

func (r *DorisClusterReconciler) reconcileStatus(context context.Context, cluster *dorisv1.DorisCluster) {

}

func (r *DorisClusterReconciler) UpdateStarRocksClusterStatus(ctx context.Context, dcr *dorisv1.DorisCluster) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var edcr dorisv1.DorisCluster
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: dcr.Namespace, Name: dcr.Name}, &edcr); err != nil {
			return err
		}

		edcr.Status = dcr.Status
		return r.Client.Status().Update(ctx, &edcr)
	})
}

// clean all resource deploy by DorisCluster
func (r *DorisClusterReconciler) resourceClean(ctx context.Context, dcr *dorisv1.DorisCluster) {
	for _, rc := range r.Scs {
		rc.ClearResources(ctx, dcr)
	}

	return
}

// SetupWithManager sets up the controller with the Manager.
func (r *DorisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dorisv1.DorisCluster{}).
		Owns(&appv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// Init initial the StarRocksClusterReconciler for reconcile.
func (r *DorisClusterReconciler) Init(mgr ctrl.Manager) {
	subcs := make(map[string]sub_controller.SubController)
	fc := fe.New(mgr.GetClient(), mgr.GetEventRecorderFor(feControllerName))
	subcs[feControllerName] = fc
	be := be.New(mgr.GetClient(), mgr.GetEventRecorderFor(beControllerName))
	subcs[beControllerName] = be

	if err := (&DorisClusterReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor(name),
		Scs:      subcs,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, " unable to create controller ", "controller ", "StarRocksCluster ")
		os.Exit(1)
	}
}
