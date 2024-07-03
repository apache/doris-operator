package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	dmsv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_http"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_meta"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	sc "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	dcgs "github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/computegroups"
	dfe "github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/disaggregated_fe"
	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
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
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"
)

var (
	_ reconcile.Reconciler = &DisaggregatedClusterReconciler{}
	_ Controller           = &DisaggregatedClusterReconciler{}
)

const (
	ms_http_token_key = "http_token"
	object_info_key   = "vault"
	ms_conf_name      = "selectdb_cloud.conf"
)

var (
	disaggregatedClusterController = "disaggregatedClusterController"
)

type DisaggregatedClusterReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Scs      map[string]sc.DisaggregatedSubController
	//record instance metadata for checking instance need create or update.
	instanceMeta map[string] /*instanceId*/ interface{}
	//record configmap response instance. key: configMap namespacedName, value: DorisDisaggregatedCluster namespacedName
	wcms map[string]string
}

func (dc *DisaggregatedClusterReconciler) Init(mgr ctrl.Manager, options *Options) {
	dc.instanceMeta = make(map[string]interface{})
	scs := make(map[string]sc.DisaggregatedSubController)
	dfec := dfe.New(mgr)
	scs[dfec.GetControllerName()] = dfec
	dcgsc := dcgs.New(mgr)
	scs[dcgsc.GetControllerName()] = dcgsc

	if err := (&DisaggregatedClusterReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor(disaggregatedClusterController),
		Scs:      scs,
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
	return builder.Complete(dc)
}

func (dc *DisaggregatedClusterReconciler) watchPodBuilder(builder *ctrl.Builder) *ctrl.Builder {
	mapFn := handler.EnqueueRequestsFromMapFunc(
		func(a client.Object) []reconcile.Request {
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

	return builder.Watches(&source.Kind{Type: &corev1.Pod{}},
		mapFn, controller_builder.WithPredicates(p))
}
func (dc *DisaggregatedClusterReconciler) watchConfigMapBuilder(builder *ctrl.Builder) *ctrl.Builder {
	mapFn := handler.EnqueueRequestsFromMapFunc(
		func(a client.Object) []reconcile.Request {
			namespace := a.GetNamespace()
			name := a.GetName()
			cmnn := types.NamespacedName{Namespace: namespace, Name: name}
			if ddc, ok := dc.wcms[cmnn.String()]; ok {
				nna := strings.Split(ddc, "/")
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
		UpdateFunc: func(u event.UpdateEvent) bool {
			ns := u.ObjectNew.GetNamespace()
			name := u.ObjectNew.GetName()
			nsn := ns + "/" + name
			_, ok := dc.wcms[nsn]
			return ok
		},
	}

	return builder.Watches(&source.Kind{Type: &corev1.Pod{}},
		mapFn, controller_builder.WithPredicates(p))
}

func (dc *DisaggregatedClusterReconciler) resourceBuilder(builder *ctrl.Builder) *ctrl.Builder {
	return builder.For(&dv1.DorisDisaggregatedCluster{}).
		Owns(&appv1.StatefulSet{}).
		Owns(&corev1.Service{})
}

// reconcile steps:
// 1. check and register instance info. info register in memory. periodical sync.
// 2. sync resource.
// 3. clear need delete resource.
// 4. reorganize status.
// 5. update cr or status.
func (dc *DisaggregatedClusterReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var ddc dv1.DorisDisaggregatedCluster
	err := dc.Get(ctx, req.NamespacedName, &ddc)
	if apierrors.IsNotFound(err) {
		klog.Warningf("disaggreatedClusterReconciler not find resource DorisDisaggregatedCluster namespaceName %s", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	if err = dc.getMSInfo(ctx, &ddc); err != nil {
		return ctrl.Result{}, err
	}

	//TODO: wait interface fixed.
	event, err := dc.ApplyInstanceInfo(ctx, &ddc)
	if event != nil {
		dc.Recorder.Event(&ddc, string(event.Type), string(event.Reason), string(event.Message))
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	//sync resource.
	if res, err := dc.reconcileSub(ctx, &ddc); err != nil {
		return res, err
	}

	// clear unused resources.
	if res, err := dc.clearUnusedResources(ctx, &ddc); err != nil {
		return res, err
	}

	//reorganize status.
	if res, err := dc.reorganizeStatus(&ddc); err != nil {
		return res, err
	}

	//update cr or status
	if res, err := dc.updateObjectORStatus(ctx, &ddc); err != nil {
		return res, err
	}

	return ctrl.Result{}, nil
}

func (dc *DisaggregatedClusterReconciler) ApplyInstanceInfo(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (*sc.Event, error) {
	if ddc.Spec.VaultConfigmap == "" {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.ObjectInfoInvalid, Message: "vaultConfigmap should config a configMap that have object info."}, errors.New("vaultConfigmap for object info should be specified")
	}

	cmName := ddc.Spec.VaultConfigmap
	var cm corev1.ConfigMap
	if err := dc.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: cmName}, &cm); err != nil {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.ObjectInfoInvalid, Message: fmt.Sprintf("name %s configmap get failed, err=%s", cmName, err.Error())}, err
	}

	if _, ok := cm.Data[object_info_key]; !ok {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.ObjectInfoInvalid, Message: fmt.Sprintf("%s configmap data have not config key %s for object info.", cmName, object_info_key)}, errors.New(fmt.Sprintf("%s configmap data have not config key %s for object info.", cmName, object_info_key))
	}

	v := cm.Data[object_info_key]
	instance := map[string]interface{}{}
	err := json.Unmarshal([]byte(v), &instance)
	if err != nil {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.ObjectInfoInvalid, Message: fmt.Sprintf("json unmarshal error=%s", err.Error())}, err
	}
	if err := dc.validateVaultInfo(instance); err != nil {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.ObjectInfoInvalid, Message: "validate failed, " + err.Error()}, err
	}

	endpoint := ddc.Status.MsEndpoint
	token := ddc.Status.MsToken
	return dc.CreateOrUpdateObjectInfo(endpoint, token, instance)
}

func (dc *DisaggregatedClusterReconciler) isModified(instance map[string]interface{}) bool {
	//TODO: the kernel interface not fixed, now not provided update function.
	return false
}

func (dc *DisaggregatedClusterReconciler) haveCreated(instanceId string) bool {
	_, ok := dc.instanceMeta[instanceId]
	return ok
}

func (dc *DisaggregatedClusterReconciler) CreateOrUpdateObjectInfo(endpoint, token string, instance map[string]interface{}) (*sc.Event, error) {
	idv := instance[ms_meta.Instance_id]
	instanceId := idv.(string)
	if !dc.haveCreated(instanceId) {
		return dc.createObjectInfo(endpoint, token, instance)
	}

	if !dc.isModified(instance) {
		return nil, nil
	}

	return nil, nil
}

func (dc *DisaggregatedClusterReconciler) createObjectInfo(endpoint, token string, instance map[string]interface{}) (*sc.Event, error) {
	str, _ := json.Marshal(instance)
	mr, err := ms_http.CreateInstance(endpoint, token, str)
	if err != nil {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.MSInteractError, Message: err.Error()}, errors.New("createObjectInfo failed, err " + err.Error())
	}
	if mr.Code != ms_http.SuccessCode {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.ObjectConfigError, Message: mr.Msg}, errors.New("createObjectInfo " + mr.Code + mr.Msg)
	}
	return &sc.Event{Type: sc.EventNormal, Reason: sc.InstanceMetaCreated}, nil
}

func (dc *DisaggregatedClusterReconciler) validateVaultInfo(instance map[string]interface{}) error {
	if obj, ok := instance[ms_meta.Obj_info]; ok {
		objMap, ok := obj.(map[string]interface{})
		if !ok {
			return errors.New("obj_info not json format")
		}

		return dc.validateS3(objMap)
	}

	if i, ok := instance[ms_meta.Key_hdfs_info]; ok {
		hdfsMap, ok := i.(map[string]interface{})
		if !ok {
			return errors.New("hdfs not json format")
		}
		return dc.validateHDFS(hdfsMap)
	}

	return errors.New("s3 and hdfs all empty")
}

func (dc *DisaggregatedClusterReconciler) validateHDFS(m map[string]interface{}) error {
	if err := dc.validateString(m, ms_meta.Key_hdfs_info_prefix); err != nil {
		return err
	}

	if err := dc.validateString(m, ms_meta.Key_hdfs_info_build_conf); err != nil {
		return err
	}
	bv := m[ms_meta.Key_hdfs_info_build_conf]
	bm, ok := bv.(map[string]interface{})
	if !ok {
		return errors.New("hdfs build_conf not json format")
	}

	if err := dc.validateString(bm, ms_meta.Key_hdfs_info_build_conf_fs_name); err != nil {
		return err
	}
	if err := dc.validateString(bm, ms_meta.Key_hdfs_info_build_conf_user); err != nil {
		return err
	}
	return nil
}

func (dc *DisaggregatedClusterReconciler) validateS3(m map[string]interface{}) error {
	cks := []string{ms_meta.Obj_info_ak, ms_meta.Obj_info_sk, ms_meta.Obj_info_bucket, ms_meta.Obj_info_prefix, ms_meta.Obj_info_prefix}
	msg := ""
	for _, ck := range cks {
		if err := dc.validateString(m, ck); err != nil {
			msg += err.Error() + ";"
		}
	}

	if msg == "" {
		return nil
	}

	return errors.New(msg)
}

func (dc *DisaggregatedClusterReconciler) validateString(m map[string]interface{}, key string) error {
	v := m[key]
	if v == nil {
		return errors.New("not config")
	}
	str, ok := v.(string)
	if !ok || str == "" {
		return errors.New("not string or empty")
	}
	return nil
}

func (dc *DisaggregatedClusterReconciler) getMSInfo(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) error {
	if ddc.Status.MsEndpoint != "" && ddc.Status.MsToken != "" {
		return nil
	}

	var ddms dmsv1.DorisDisaggregatedMetaService
	msNamespace := ddc.Spec.MetaService.Namespace
	msName := ddc.Spec.MetaService.Name
	if err := dc.Get(ctx, types.NamespacedName{Namespace: msNamespace, Name: msName}, &ddms); err != nil {
		klog.Errorf("disaggregatedClusterReconciler getMSInfo namespace %s name %s faild, err=%s", msNamespace, msName, err.Error())
		dc.Recorder.Event(ddc, string(sc.EventWarning), string(sc.DisaggregatedMetaServiceGetFailed), fmt.Sprintf("namespace %s name %s get failed,err%s", msNamespace, msName, err.Error()))
		return err
	}

	//TODO: get metaservice serviceName
	msSvcName := ddms.GetMSServiceName()
	msPort := dmsv1.MsPort
	msEndpoint := msSvcName + "." + msNamespace + ":" + msPort
	ddc.Status.MsEndpoint = msEndpoint
	ddc.Status.MsToken = dmsv1.DefaultMsToken

	if ddms.Spec.MS == nil || len(ddms.Spec.MS.ConfigMaps) == 0 {
		return nil
	}

	mscvs := map[string]interface{}{}
	for _, cm := range ddms.Spec.MS.ConfigMaps {
		var kcm corev1.ConfigMap
		if err := dc.Get(ctx, types.NamespacedName{Namespace: msNamespace, Name: cm.Name}, &kcm); err != nil {
			klog.Errorf("disaggregatedClusterReconciler getMSInfo get configmap namespace %s name %s failed, err=%s", msNamespace, cm.Name, err.Error())
			continue
		}

		if _, ok := kcm.Data[ms_conf_name]; ok {
			v := kcm.Data[ms_conf_name]
			viper.ReadConfig(bytes.NewBuffer([]byte(v)))
			mscvs = viper.AllSettings()
			break
		}
	}
	if v := mscvs[ms_http_token_key]; v != nil {
		token := v.(string)
		ddc.Status.MsToken = token
	}

	return nil
}

func (dc *DisaggregatedClusterReconciler) clearUnusedResources(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	for _, sc := range dc.Scs {
		sc.ClearResources(ctx, ddc)
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
	return ctrl.Result{}, nil
}

func (dc *DisaggregatedClusterReconciler) reconcileSub(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	for _, sc := range dc.Scs {
		if err := sc.Sync(ctx, ddc); err != nil {
			klog.Errorf("disaggreatedClusterReconciler sub reconciler %s sync err=%s.", sc.GetControllerName(), err.Error())
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// when spec revert by operator should update cr or directly update status.
func (dc *DisaggregatedClusterReconciler) updateObjectORStatus(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	old_hv := ddc.Annotations[dv1.DisaggregatedSpecHashValueAnnotation]
	hv := hash.HashObject(ddc.Spec)
	if ddc.Annotations == nil {
		ddc.Annotations = map[string]string{dv1.DisaggregatedSpecHashValueAnnotation: hv}
	}
	if old_hv != hv {
		ddc.Annotations[dv1.DisaggregatedSpecHashValueAnnotation] = hv
		//TODO: test status updated or not.
		if err := dc.Update(ctx, ddc); err != nil {
			klog.Errorf("disaggreatedClusterReconciler update DorisDisaggregatedCluster namespace %s name %s  failed, err=%s", ddc.Namespace, ddc.Name, err.Error())
			return ctrl.Result{}, err
		}

		//if cr updated, update cr. or update status.
		return ctrl.Result{}, nil
	}

	return dc.updateDorisDisaggregatedClusterStatus(ctx, ddc)
}

func (dc *DisaggregatedClusterReconciler) updateDorisDisaggregatedClusterStatus(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	var eddc dv1.DorisDisaggregatedCluster
	if err := dc.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.Name}, &eddc); err != nil {
		return ctrl.Result{}, err
	}

	ddc.Status.DeepCopyInto(&eddc.Status)
	retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return dc.Status().Update(ctx, &eddc)
	})

	// if the status is not equal before reconcile and now the status is not available we should requeue.
	if !disAggregatedInconsistentStatus(&ddc.Status, &eddc) {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}
