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
	"bytes"
	"context"
	"errors"
	"fmt"
	dv1 "github.com/selectdb/doris-operator/api/disaggregated/cluster/v1"
	dmsv1 "github.com/selectdb/doris-operator/api/disaggregated/metaservice/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_http"
	"github.com/selectdb/doris-operator/pkg/common/utils/disaggregated_ms/ms_meta"
	"github.com/selectdb/doris-operator/pkg/common/utils/hash"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	sc "github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	dccs "github.com/selectdb/doris-operator/pkg/controller/sub_controller/disaggregated_cluster/computeclusters"
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
	instance_conf_key = "instance.conf"
	ms_conf_name      = "doris_cloud.conf"
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
	im := make(map[string]interface{})
	wcms := make(map[string]string)
	scs := make(map[string]sc.DisaggregatedSubController)
	dfec := dfe.New(mgr)
	scs[dfec.GetControllerName()] = dfec
	dccsc := dccs.New(mgr)
	scs[dccsc.GetControllerName()] = dccsc

	if err := (&DisaggregatedClusterReconciler{
		Client:       mgr.GetClient(),
		Recorder:     mgr.GetEventRecorderFor(disaggregatedClusterController),
		Scs:          scs,
		instanceMeta: im,
		wcms:         wcms,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller ", "disaggregatedClusterReconciler")
		os.Exit(1)
	}

	if options.EnableWebHook {
		if _, err := (&dv1.DorisDisaggregatedCluster{}).SetupWebhookWithManager(mgr); err != nil {
			klog.Error(err, " unable to create unnamedwatches ", " controller ", " DorisDisaggregatedCluster ")
			os.Exit(1)
		}
	}
}

func (dc *DisaggregatedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := dc.resourceBuilder(ctrl.NewControllerManagedBy(mgr))
	builder = dc.watchPodBuilder(builder)
	builder = dc.watchConfigMapBuilder(builder)
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

	return builder.Watches(&source.Kind{Type: &corev1.Pod{}},
		mapFn, controller_builder.WithPredicates(p))
}
func (dc *DisaggregatedClusterReconciler) watchConfigMapBuilder(builder *ctrl.Builder) *ctrl.Builder {
	mapFn := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, a client.Object) []reconcile.Request {
			namespace := a.GetNamespace()
			name := a.GetName()
			cmnn := types.NamespacedName{Namespace: namespace, Name: name}
			cmnnStr := cmnn.String()
			if ddc, ok := dc.wcms[cmnnStr]; ok {
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

	return builder.Watches(&source.Kind{Type: &corev1.ConfigMap{}},
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
// 4. display new status(eorganize status, update cr or status)
func (dc *DisaggregatedClusterReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var ddc dv1.DorisDisaggregatedCluster
	err := dc.Get(ctx, req.NamespacedName, &ddc)
	if apierrors.IsNotFound(err) {
		klog.Warningf("disaggreatedClusterReconciler not find resource DorisDisaggregatedCluster namespaceName %s", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	hv := hash.HashObject(ddc.Spec)

	if err = dc.setStatusMSInfo(ctx, &ddc); err != nil {
		return ctrl.Result{}, err
	}

	var res ctrl.Result
	//get instance config, validating config, display some instance info in DorisDisaggregatedCluster, apply instance info into ms.
	if res, err = func() (ctrl.Result, error) {
		instanceConf, err := dc.getInstanceConfig(ctx, &ddc)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := dc.validateInstanceInfo(instanceConf); err != nil {
			return ctrl.Result{}, err
		}
		//display InstanceInfo in DorisDisaggregatedCluster
		dc.displayInstanceInfo(instanceConf, &ddc)

		//TODO: wait interface fixed. realize update ak,sk.
		event, err := dc.ApplyInstanceMeta(ddc.Status.MsEndpoint, ddc.Status.MsToken, instanceConf)
		if event != nil {
			dc.Recorder.Event(&ddc, string(event.Type), string(event.Reason), event.Message)
		}
		return ctrl.Result{}, err
	}(); err != nil {
		return res, err
	}

	//sync resource.
	if res, err = dc.reconcileSub(ctx, &ddc); err != nil {
		return res, err
	}

	// clear unused resources.
	if res, err = dc.clearUnusedResources(ctx, &ddc); err != nil {
		return res, err
	}

	//display new status.
	res, err = func() (ctrl.Result, error) {
		//reorganize status.
		if res, err = dc.reorganizeStatus(&ddc); err != nil {
			return res, err
		}

		//update cr or status
		if res, err = dc.updateObjectORStatus(ctx, &ddc, hv); err != nil {
			return res, err
		}

		return ctrl.Result{}, nil
	}()

	return res, err
}

func (dc *DisaggregatedClusterReconciler) displayInstanceInfo(instanceConf map[string]interface{}, ddc *dv1.DorisDisaggregatedCluster) {
	instanceId := (instanceConf[ms_meta.Instance_id]).(string)
	ddc.Status.InstanceId = instanceId
}

func (dc *DisaggregatedClusterReconciler) getInstanceConfig(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (map[string]interface{}, error) {
	if ddc.Spec.InstanceConfigMap == "" {
		dc.Recorder.Event(ddc, string(sc.EventWarning), string(sc.ObjectInfoInvalid), "vaultConfigmap should config a configMap that have object info.")
		return nil, errors.New("vaultConfigmap for object info should be specified")
	}

	cmnn := types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.Spec.InstanceConfigMap}
	ddcnn := types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.Name}
	cmnnStr := cmnn.String()
	ddcnnStr := ddcnn.String()
	if _, ok := dc.wcms[cmnnStr]; !ok {
		dc.wcms[cmnnStr] = ddcnnStr
	}

	cmName := ddc.Spec.InstanceConfigMap
	var cm corev1.ConfigMap
	if err := dc.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: cmName}, &cm); err != nil {
		dc.Recorder.Event(ddc, string(sc.EventWarning), string(sc.ObjectInfoInvalid), fmt.Sprintf("name %s configmap get failed, err=%s", cmName, err.Error()))
		return nil, err
	}

	if _, ok := cm.Data[instance_conf_key]; !ok {
		dc.Recorder.Event(ddc, string(sc.EventWarning), string(sc.ObjectInfoInvalid), fmt.Sprintf("%s configmap data have not config key %s for object info.", cmName, instance_conf_key))
		return nil, errors.New(fmt.Sprintf("%s configmap data have not config key %s for object info.", cmName, instance_conf_key))
	}

	v := cm.Data[instance_conf_key]
	instance := map[string]interface{}{}
	err := json.Unmarshal([]byte(v), &instance)
	if err != nil {
		dc.Recorder.Event(ddc, string(sc.EventWarning), string(sc.ObjectInfoInvalid), fmt.Sprintf("json unmarshal error=%s", err.Error()))
		return nil, err
	}

	return instance, nil
}

func (dc *DisaggregatedClusterReconciler) ApplyInstanceMeta(endpoint, token string, instanceConf map[string]interface{}) (*sc.Event, error) {

	instanceId := (instanceConf[ms_meta.Instance_id]).(string)
	event, err := dc.CreateOrUpdateObjectMeta(endpoint, token, instanceConf)
	if err != nil {
		return event, err
	}

	// store instance info for next update ak, sk etc...
	dc.instanceMeta[instanceId] = instanceConf
	return nil, nil
}

func (dc *DisaggregatedClusterReconciler) isModified(instance map[string]interface{}) bool {
	//TODO: the kernel interface not fixed, now not provided update function.
	return false
}

func (dc *DisaggregatedClusterReconciler) haveCreated(instanceId string) bool {
	_, ok := dc.instanceMeta[instanceId]
	//TODO: get from ms check
	return ok
}

func (dc *DisaggregatedClusterReconciler) CreateOrUpdateObjectMeta(endpoint, token string, instance map[string]interface{}) (*sc.Event, error) {
	idv := instance[ms_meta.Instance_id]
	instanceId := idv.(string)
	if !dc.haveCreated(instanceId) {
		return dc.createObjectInfo(endpoint, token, instance)
	}

	// if not match in memory, should compare with ms.
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
	if mr.Code != ms_http.SuccessCode && mr.Code != ms_http.ALREADY_EXIST {
		return &sc.Event{Type: sc.EventWarning, Reason: sc.ObjectConfigError, Message: mr.Msg}, errors.New("createObjectInfo " + mr.Code + mr.Msg)
	}

	return &sc.Event{Type: sc.EventNormal, Reason: sc.InstanceMetaCreated}, nil
}

func (dc *DisaggregatedClusterReconciler) validateInstanceInfo(instanceConf map[string]interface{}) error {
	idv := instanceConf[ms_meta.Instance_id]
	if idv == nil {
		return errors.New("not config instance id")
	}
	id, ok := idv.(string)
	if !ok || id == "" {
		return errors.New("not config instance id")
	}
	return dc.validateVaultInfo(instanceConf)
}

func (dc *DisaggregatedClusterReconciler) validateVaultInfo(instanceConf map[string]interface{}) error {
	vi := instanceConf[ms_meta.Vault]
	if vi == nil {
		return errors.New("have not vault config")
	}

	vault, ok := vi.(map[string]interface{})
	if !ok {
		return errors.New("vault not json format")
	}

	if obj, ok := vault[ms_meta.Obj_info]; ok {
		objMap, ok := obj.(map[string]interface{})
		if !ok {
			return errors.New("obj_info not json format")
		}

		return dc.validateS3(objMap)
	}

	if i, ok := vault[ms_meta.Key_hdfs_info]; ok {
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

func (dc *DisaggregatedClusterReconciler) setStatusMSInfo(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) error {
	if ddc.Status.MsEndpoint != "" && ddc.Status.MsToken != "" {
		return nil
	}

	var ddms dmsv1.DorisDisaggregatedMetaService
	msNamespace := ddc.Spec.DisMS.Namespace
	msName := ddc.Spec.DisMS.Name
	if err := dc.Get(ctx, types.NamespacedName{Namespace: msNamespace, Name: msName}, &ddms); err != nil {
		klog.Errorf("disaggregatedClusterReconciler getMSInfo namespace %s name %s faild, err=%s", msNamespace, msName, err.Error())
		dc.Recorder.Event(ddc, string(sc.EventWarning), string(sc.DisaggregatedMetaServiceGetFailed), fmt.Sprintf("namespace %s name %s get failed,err%s", msNamespace, msName, err.Error()))
		return err
	}

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

		if v, ok := kcm.Data[ms_conf_name]; ok {
			viper.ReadConfig(bytes.NewBuffer([]byte(v)))
			mscvs = viper.AllSettings()
			break
		}
	}
	if v := mscvs[ms_http_token_key]; v != nil {
		token := v.(string)
		ddc.Status.MsToken = token
	}
	if v, ok := mscvs[resource.BRPC_LISTEN_PORT]; ok {
		msPort = v.(string)
	}

	return nil
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
	if ddc.Status.FEStatus.AvailableStatus != dv1.Available || ddc.Status.ClusterHealth.CCAvailableCount <= (ddc.Status.ClusterHealth.CCCount/2) {
		ddc.Status.ClusterHealth.Health = dv1.Red
	} else if ddc.Status.FEStatus.Phase != dv1.Ready || ddc.Status.ClusterHealth.CCAvailableCount < ddc.Status.ClusterHealth.CCCount {
		ddc.Status.ClusterHealth.Health = dv1.Yellow
	}
	return ctrl.Result{}, nil
}

func (dc *DisaggregatedClusterReconciler) reconcileSub(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster) (ctrl.Result, error) {
	for _, subC := range dc.Scs {
		if err := subC.Sync(ctx, ddc); err != nil {
			klog.Errorf("disaggreatedClusterReconciler sub reconciler %s sync err=%s.", subC.GetControllerName(), err.Error())
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// when spec revert by operator should update cr or directly update status.
func (dc *DisaggregatedClusterReconciler) updateObjectORStatus(ctx context.Context, ddc *dv1.DorisDisaggregatedCluster, preHv string) (ctrl.Result, error) {
	postHv := hash.HashObject(ddc.Spec)
	if preHv != postHv {
		var eddc dv1.DorisDisaggregatedCluster
		if err := dc.Get(ctx, types.NamespacedName{Namespace: ddc.Namespace, Name: ddc.Name}, &eddc); err == nil || !apierrors.IsNotFound(err) {
			if eddc.ResourceVersion != "" {
				ddc.ResourceVersion = eddc.ResourceVersion
			}
		}
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
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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
