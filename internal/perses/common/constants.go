// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"errors"
	"net/http"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses/pkg/client/perseshttp"
	logger "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var clog = logger.WithField("module", "constants")

const (
	PersesNamespaceDomain     = "perses.dev"
	PersesFinalizer           = PersesNamespaceDomain + "/finalizer"
	PersesProvisioningVersion = PersesNamespaceDomain + "/provisioning-version"
	PersesWatchLabel          = PersesNamespaceDomain + "/watch"
	PersesWatchLabelValue     = "true"
	PersesManagedByLabel      = "app.kubernetes.io/managed-by"
	PersesManagedByValue      = "perses-operator"
	TypeAvailablePerses       = "Available"
	TypeDegradedPerses        = "Degraded"

	// Flags
	PersesServerURLFlag      = "perses-server-url"
	WatchSecretLabelsFlag    = "watch-secret-labels"
	WatchAllSecretsFlag      = "watch-all-secrets"
	TLSMinVersionFlag        = "tls-min-version"
	TLSCipherSuitesFlag      = "tls-cipher-suites"
	TLSClusterProfileFlag    = "tls-cluster-profile"
	TLSConfigureOperandsFlag = "tls-configure-operands"
	ResourceSyncIntervalFlag = "resource-sync-interval"

	// Volume names
	configVolumeName  = "config"
	StorageVolumeName = "storage"
	pluginsVolumeName = "plugins"

	// TLS volume names
	caVolumeName     = "ca"
	caMountPath      = "/ca"
	tlsVolumeName    = "tls"
	tlsCertMountPath = "/tls"

	// Mount paths
	storageMountPath  = "/perses"
	secretsMountPath  = "/etc/perses/provisioning/secrets"
	configMountPath   = "/etc/perses/config"
	pluginsMountPath  = "/etc/perses/plugins"
	defaultConfigPath = configMountPath + "/config.yaml"

	defaultFileMode = 420

	// DefaultContainerPort is the default port on which the Perses server listens
	DefaultContainerPort = int32(8080)
)

type ConditionStatusReason string

// ReasonError wraps an error with a ConditionStatusReason so callers can
// extract the specific failure reason for metrics and logging.
type ReasonError struct {
	Err    error
	Reason ConditionStatusReason
}

func (e *ReasonError) Error() string { return e.Err.Error() }
func (e *ReasonError) Unwrap() error { return e.Err }

func NewReasonError(err error, reason ConditionStatusReason) *ReasonError {
	return &ReasonError{Err: err, Reason: reason}
}

// ExtractReason returns the ConditionStatusReason from an error if it wraps
// a ReasonError, otherwise returns the provided fallback.
func ExtractReason(err error, fallback ConditionStatusReason) ConditionStatusReason {
	var re *ReasonError
	if errors.As(err, &re) {
		return re.Reason
	}
	return fallback
}

const (
	// Failure to be used when unable to locate any perses backends
	ReasonMissingPerses        ConditionStatusReason = "PersesMissing"
	ReasonConnectionFailed     ConditionStatusReason = "PersesConnectionFailed"
	ReasonInvalidConfiguration ConditionStatusReason = "InvalidConfiguration"
	// Failure to be used when resource that started the reconciliation is unable to be found
	ReasonMissingResource ConditionStatusReason = "MissingResource"
	// Failure to be used when server-side validation of a resource fails
	ReasonValidationFailed ConditionStatusReason = "ValidationFailed"
	// Generic failure for when the reason is due to the backend returning an error
	ReasonBackendError ConditionStatusReason = "PersesBackendError"
)

// reasonSeverity ranks condition reasons so that, when a reconcile fans out to
// multiple pods and they fail for different reasons, the most actionable reason
// governs the resulting status condition. Higher wins. User-facing reasons
// (misconfiguration, validation) outrank connectivity, which outranks the
// generic backend error, so a "last write wins" loop cannot mask a config
// problem behind an unrelated backend error from another pod.
var reasonSeverity = map[ConditionStatusReason]int{
	ReasonBackendError:         1,
	ReasonConnectionFailed:     2,
	ReasonValidationFailed:     3,
	ReasonInvalidConfiguration: 3,
}

// MergeReason returns whichever of current and incoming should govern the
// status condition, breaking ties toward current. An empty current (no failure
// seen yet) always yields to incoming. Unknown reasons rank below all known
// ones so an explicit reason is always preferred over an unclassified one.
func MergeReason(current, incoming ConditionStatusReason) ConditionStatusReason {
	if current == "" {
		return incoming
	}
	if reasonSeverity[incoming] > reasonSeverity[current] {
		return incoming
	}
	return current
}

// IsClientError returns true if the error is an HTTP 4xx response from the
// Perses API, indicating a client-side error such as a validation failure.
// Returns false for 5xx errors, network errors, or non-HTTP errors.
func IsClientError(err error) bool {
	var reqErr *perseshttp.RequestError
	if errors.As(err, &reqErr) {
		return reqErr.StatusCode >= http.StatusBadRequest && reqErr.StatusCode < http.StatusInternalServerError
	}
	return false
}

// isTLSEnabled checks if TLS is enabled in the Perses configuration
func isTLSEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.TLS != nil &&
		perses.Spec.TLS.Enable != nil &&
		*perses.Spec.TLS.Enable
}

// hasTLSConfiguration checks if valid TLS configuration is present
func hasTLSConfiguration(perses *v1alpha2.Perses) bool {
	return isTLSEnabled(perses) &&
		perses.Spec.TLS.UserCert != nil &&
		perses.Spec.TLS.UserCert.CertPath != "" &&
		perses.Spec.TLS.UserCert.PrivateKeyPath != nil &&
		*perses.Spec.TLS.UserCert.PrivateKeyPath != ""
}

// isClientTLSEnabled checks if TLS is enabled in the Perses client configuration
func isClientTLSEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.Client != nil &&
		perses.Spec.Client.TLS != nil &&
		perses.Spec.Client.TLS.Enable != nil &&
		*perses.Spec.Client.TLS.Enable
}

func isKubernetesAuthEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.Client != nil &&
		perses.Spec.Client.KubernetesAuth != nil &&
		perses.Spec.Client.KubernetesAuth.Enable != nil &&
		*perses.Spec.Client.KubernetesAuth.Enable
}

// isClientOAuthEnabled checks if OAuth authentication is configured for the Perses client
func isClientOAuthEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.Client != nil &&
		perses.Spec.Client.OAuth != nil
}

// PersesBecameAvailable returns true when a Perses instance transitions
// to Available status, indicating its API is ready to accept requests.
func PersesBecameAvailable(oldObj, newObj client.Object) bool {
	oldPerses, ok := oldObj.(*v1alpha2.Perses)
	if !ok {
		return false
	}
	newPerses, ok := newObj.(*v1alpha2.Perses)
	if !ok {
		return false
	}
	wasAvailable := meta.IsStatusConditionTrue(oldPerses.Status.Conditions, TypeAvailablePerses)
	isAvailable := meta.IsStatusConditionTrue(newPerses.Status.Conditions, TypeAvailablePerses)
	return !wasAvailable && isAvailable
}

// MetadataListToRequests lists all objects of the given GVK from the metadata-only
// cache and returns a reconcile.Request for each one. It is intended for use in
// mapper functions passed to handler.EnqueueRequestsFromMapFunc.
func MetadataListToRequests(ctx context.Context, r client.Reader, gvk schema.GroupVersionKind) []reconcile.Request {
	list := &metav1.PartialObjectMetadataList{}
	list.SetGroupVersionKind(gvk)
	if err := r.List(ctx, list); err != nil {
		clog.WithError(err).Errorf("Failed to list %s for Perses instance change", gvk.Kind)
		return nil
	}

	requests := make([]reconcile.Request, len(list.Items))
	for i, obj := range list.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      obj.Name,
				Namespace: obj.Namespace,
			},
		}
	}
	return requests
}

// PersesInstanceForPod resolves the Perses instance that owns pod, or nil if
// the pod is not a Perses-operator-managed pod or no matching instance exists.
// The pod's app.kubernetes.io/instance label carries the sanitized instance
// name (see LabelsForPerses), so the owner is found by matching each candidate
// instance's computed instance label rather than assuming the raw name.
func PersesInstanceForPod(ctx context.Context, r client.Reader, pod *corev1.Pod) (*v1alpha2.Perses, error) {
	if pod.Labels["app.kubernetes.io/managed-by"] != "perses-operator" {
		return nil, nil
	}
	instanceLabel := pod.Labels["app.kubernetes.io/instance"]
	if instanceLabel == "" {
		return nil, nil
	}

	instances := &v1alpha2.PersesList{}
	if err := r.List(ctx, instances, client.InNamespace(pod.Namespace)); err != nil {
		return nil, err
	}
	for i := range instances.Items {
		instance := &instances.Items[i]
		if LabelsForPerses(instance.Name, instance)["app.kubernetes.io/instance"] == instanceLabel {
			return instance, nil
		}
	}
	return nil, nil
}

// ResourceTargetsInstance reports whether a resource with the given
// instanceSelector targets the Perses instance described by instanceLabels.
// A nil selector matches every instance (mirroring the reconcile path's use of
// labels.Everything() when no selector is set). It is the inverse of the
// forward match the controllers apply when listing instances for a resource,
// and is used by pod mappers to enqueue only the resources that target the
// instance owning the pod.
func ResourceTargetsInstance(instanceSelector *metav1.LabelSelector, instanceLabels map[string]string) (bool, error) {
	if instanceSelector == nil {
		return true, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(instanceSelector)
	if err != nil {
		return false, err
	}
	return selector.Matches(labels.Set(instanceLabels)), nil
}

// when a Perses instance becomes available or is deleted. This is used by
// child resource controllers (Dashboard, Datasource, GlobalDatasource) to
// reconcile their resources when the parent Perses instance becomes ready.
func PersesAvailabilityPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return PersesBecameAvailable(e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// PodBecameReady returns true when a pod transitions from non-Ready to Ready.
func PodBecameReady(oldObj, newObj client.Object) bool {
	oldPod, ok := oldObj.(*corev1.Pod)
	if !ok {
		return false
	}
	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		return false
	}
	return !isPodConditionReady(oldPod) && isPodConditionReady(newPod)
}

func isPodConditionReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// PodReadinessPredicate returns a predicate that triggers reconciliation
// when a pod transitions to Ready. This is used to re-sync resources to
// pods that restarted and lost their in-memory or emptyDir state.
//
// CreateFunc fires for pods that are already Ready when the event is observed.
// On operator startup the informer replays every existing pod as a Create
// event (event.CreateEvent, with IsInInitialList set), so returning false here
// would skip resyncing pods that were already running before the operator
// (re)started.
func PodReadinessPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			pod, ok := e.Object.(*corev1.Pod)
			if !ok {
				return false
			}
			return isPodConditionReady(pod)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return PodBecameReady(e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
