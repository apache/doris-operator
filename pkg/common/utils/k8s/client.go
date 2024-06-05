package k8s

import (
	"context"
	"errors"
	"fmt"
	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/common/utils"
	"github.com/selectdb/doris-operator/pkg/common/utils/resource"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/autoscaling/v1"
	v2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// judge two services equal or not in some fields. develoer can custom the function.
type ServiceEqual func(svc1 *corev1.Service, svc2 *corev1.Service) bool

// judge two statefulset equal or not in some fields. develoer can custom the function.
type StatefulSetEqual func(st1 *appv1.StatefulSet, st2 *appv1.StatefulSet) bool

func ApplyService(ctx context.Context, k8sclient client.Client, svc *corev1.Service, equal ServiceEqual) error {
	// As stated in the RetryOnConflict's documentation, the returned error shouldn't be wrapped.
	var esvc corev1.Service
	err := k8sclient.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &esvc)
	if err != nil && apierrors.IsNotFound(err) {
		return CreateClientObject(ctx, k8sclient, svc)
	} else if err != nil {
		return err
	}

	if equal(svc, &esvc) {
		klog.Info("CreateOrUpdateService service Name, Ports, Selector, ServiceType, Labels have not change ", "namespace ", svc.Namespace, " name ", svc.Name)
		return nil
	}

	//resolve the bug: metadata.resourceversion invalid value '' must be specified for an update
	svc.ResourceVersion = esvc.ResourceVersion
	return PatchClientObject(ctx, k8sclient, svc)
}

// ApplyStatefulSet when the object is not exist, create object. if exist and statefulset have been updated, patch the statefulset.
func ApplyStatefulSet(ctx context.Context, k8sclient client.Client, st *appv1.StatefulSet, equal StatefulSetEqual) error {
	var est appv1.StatefulSet
	err := k8sclient.Get(ctx, types.NamespacedName{Namespace: st.Namespace, Name: st.Name}, &est)
	if err != nil && apierrors.IsNotFound(err) {
		return CreateClientObject(ctx, k8sclient, st)
	} else if err != nil {
		return err
	}

	//if have restart annotation we should exclude it impacts on hash.
	if equal(st, &est) {
		klog.Infof("ApplyStatefulSet Sync exist statefulset name=%s, namespace=%s, equals to new statefulset.", est.Name, est.Namespace)
		return nil
	}

	st.ResourceVersion = est.ResourceVersion
	return PatchClientObject(ctx, k8sclient, st)
}

func CreateClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.Info("Creating resource service ", "namespace ", object.GetNamespace(), " name ", object.GetName(), " kind ", object.GetObjectKind().GroupVersionKind().Kind)
	if err := k8sclient.Create(ctx, object); err != nil {
		return err
	}
	return nil
}

func UpdateClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.Info("Updating resource service ", "namespace ", object.GetNamespace(), " name ", object.GetName(), " kind ", object.GetObjectKind())
	if err := k8sclient.Update(ctx, object); err != nil {
		return err
	}
	return nil
}

func CreateOrUpdateClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.V(4).Infof("create or update resource namespace=%s,name=%s,kind=%s.", object.GetNamespace(), object.GetName(), object.GetObjectKind())
	if err := k8sclient.Update(ctx, object); apierrors.IsNotFound(err) {
		return k8sclient.Create(ctx, object)
	} else if err != nil {
		return err
	}

	return nil
}

// PatchClientObject patch object when the object exist. if not return error.
func PatchClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.V(4).Infof("patch resource namespace=%s,name=%s,kind=%s.", object.GetNamespace(), object.GetName(), object.GetObjectKind())
	if err := k8sclient.Patch(ctx, object, client.Merge); err != nil {
		return err
	}

	return nil
}

// PatchOrCreate patch object if not exist create object.
func PatchOrCreate(ctx context.Context, k8sclient client.Client, object client.Object) error {
	klog.V(4).Infof("patch or create resource namespace=%s,name=%s,kind=%s.", object.GetNamespace(), object.GetName(), object.GetObjectKind())
	if err := k8sclient.Patch(ctx, object, client.Merge); apierrors.IsNotFound(err) {
		return k8sclient.Create(ctx, object)
	} else if err != nil {
		return err
	}

	return nil
}

func DeleteClientObject(ctx context.Context, k8sclient client.Client, object client.Object) error {
	if err := k8sclient.Delete(ctx, object); err != nil {
		return err
	}
	return nil
}

// DeleteStatefulset delete statefulset.
func DeleteStatefulset(ctx context.Context, k8sclient client.Client, namespace, name string) error {
	var st appv1.StatefulSet
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &st); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return k8sclient.Delete(ctx, &st)
}

// DeleteService delete service.
func DeleteService(ctx context.Context, k8sclient client.Client, namespace, name string) error {
	var svc corev1.Service
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &svc); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return k8sclient.Delete(ctx, &svc)
}

// DeleteAutoscaler as version type delete response autoscaler.
func DeleteAutoscaler(ctx context.Context, k8sclient client.Client, namespace, name string, autoscalerVersion dorisv1.AutoScalerVersion) error {
	var autoscaler client.Object
	switch autoscalerVersion {
	case dorisv1.AutoScalerV1:
		autoscaler = &v1.HorizontalPodAutoscaler{}
	case dorisv1.AutoSclaerV2:
		autoscaler = &v2.HorizontalPodAutoscaler{}

	default:
		return errors.New(fmt.Sprintf("the autoscaler type %s is not supported.", autoscalerVersion))
	}

	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, autoscaler); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return k8sclient.Delete(ctx, autoscaler)
}

func PodIsReady(status *corev1.PodStatus) bool {
	if status.ContainerStatuses == nil {
		return false
	}

	for _, cs := range status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
	}

	return true
}

// get the secret by namespace and name.
func GetSecret(ctx context.Context, k8sclient client.Client, namespace, name string) (*corev1.Secret, error) {
	var secret corev1.Secret
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func CreateSecret(ctx context.Context, k8sclient client.Client, secret *corev1.Secret) error {
	return k8sclient.Create(ctx, secret)
}

func UpdateSecret(ctx context.Context, k8sclient client.Client, secret *corev1.Secret) error {
	if err := k8sclient.Update(ctx, secret); err != nil {
		return err
	}
	return nil
}

// GetConfigMap get the configmap name=name, namespace=namespace.
func GetConfigMap(ctx context.Context, k8scient client.Client, namespace, name string) (*corev1.ConfigMap, error) {
	var configMap corev1.ConfigMap
	if err := k8scient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &configMap); err != nil {
		return nil, err
	}

	return &configMap, nil
}

// GetConfigMaps get the configmap by the array of MountConfigMapInfo and namespace.
func GetConfigMaps(ctx context.Context, k8scient client.Client, namespace string, cms []dorisv1.MountConfigMapInfo) ([]*corev1.ConfigMap, error) {
	var configMaps []*corev1.ConfigMap
	errMessage := ""
	for _, cm := range cms {
		var configMap corev1.ConfigMap
		if getErr := k8scient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cm.ConfigMapName}, &configMap); getErr != nil {
			errMessage = errMessage + fmt.Sprintf("(name: %s, namespace: %s, err: %s), ", cm.ConfigMapName, namespace, getErr.Error())
		}
		configMaps = append(configMaps, &configMap)
	}
	if errMessage != "" {
		return configMaps, errors.New("Failed to get configmap: " + errMessage)
	}
	return configMaps, nil
}

// get the Service by namespace and name.
func GetService(ctx context.Context, k8sclient client.Client, namespace, name string) (*corev1.Service, error) {
	var svc corev1.Service
	if err := k8sclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &svc); err != nil {
		return nil, err
	}
	return &svc, nil
}

func GetPods(ctx context.Context, k8sclient client.Client, targetDCR dorisv1.DorisCluster) (corev1.PodList, error) {
	pods := corev1.PodList{}

	err := k8sclient.List(ctx, &pods, client.InNamespace(targetDCR.Namespace), client.MatchingLabels(dorisv1.GetPodLabels(&targetDCR, dorisv1.Component_FE)))
	if err != nil {
		return pods, err
	}

	for _, pod := range pods.Items {
		fmt.Printf("pod --- Name: %s,  pod: %s \n", pod.GetName(), pod.Status.PodIP)
	}

	return pods, nil
}

// GetConfig get conf from configmap by componentType , if not use configmap get an empty map.
func GetConfig(ctx context.Context, k8sclient client.Client, configMapInfo *dorisv1.ConfigMapInfo, namespace string, componentType dorisv1.ComponentType) (map[string]interface{}, error) {
	cms := resource.GetMountConfigMapInfo(*configMapInfo)
	if len(cms) == 0 {
		return make(map[string]interface{}), nil
	}

	configMaps, err := GetConfigMaps(ctx, k8sclient, namespace, cms)
	if err != nil {
		klog.Errorf("GetConfig get configmap failed, namespace: %s,err: %s \n", namespace, err.Error())
	}
	res, resolveErr := resource.ResolveConfigMaps(configMaps, componentType)
	return res, utils.MergeError(err, resolveErr)
}

func GetDorisClusterPhase(ctx context.Context, k8sclient client.Client, dcrName, namespace string) (*dorisv1.ClusterPhase, error) {
	var edcr dorisv1.DorisCluster
	if err := k8sclient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: dcrName}, &edcr); err != nil {
		return nil, err
	}
	return &edcr.Status.ClusterPhase, nil
}

func SetDorisClusterPhase(
	ctx context.Context,
	k8sclient client.Client,
	dcrName, namespace string,
	phase dorisv1.ClusterPhase,
) error {
	var edcr dorisv1.DorisCluster
	if err := k8sclient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: dcrName}, &edcr); err != nil {
		return err
	}
	if edcr.Status.ClusterPhase.Phase == phase.Phase && edcr.Status.ClusterPhase.Retry == phase.Retry {
		klog.Infof("UpdateDorisClusterPhase will not change cluster Phase, it is already %s ,DCR name: %s, namespace: %s,", phase.Phase, dcrName, namespace)
		return nil
	}
	edcr.Status.ClusterPhase = phase
	return k8sclient.Status().Update(ctx, &edcr)
}

// DeletePVC delete pvc .
func DeletePVCs(ctx context.Context, k8sclient client.Client, namespace string, pvcs []corev1.PersistentVolumeClaim) error {

	// 60 seconds was picked arbitrarily
	gracePeriod := int64(60)
	propagationPolicy := metav1.DeletePropagationForeground

	for _, pvc := range pvcs {
		// Ensure that our context is still active. It will be canceled if a
		// change to sts.Spec.Replicas is detected.
		select {
		case <-ctx.Done():
			return errors.New("concurrent statefulset modification detected")
		default:
		}

		options := client.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
			// Wait for the underlying PV to be deleted before moving on to
			// the next volume.
			PropagationPolicy: &propagationPolicy,
			Preconditions: &metav1.Preconditions{
				// Ensure that this PVC is the same PVC that we slated for
				// deletion. If for some reason there are concurrent scale jobs
				// running, this will prevent us from re-deleting a PVC that
				// was removed and recreated.
				UID: &pvc.UID,
				// Ensure that this PVC has not changed since we fetched it.
				// This check doesn't help very much as a PVC is not actually
				// modified when it's mounted to a pod.
				ResourceVersion: &pvc.ResourceVersion,
			},
		}

		klog.Infof("DeletePVCs deleting PVC name: %s", pvc.Name)
		if err := k8sclient.Delete(ctx, &pvc, &options); err != nil {
			return nil
		}
	}

	return nil

}
