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

package perses

import (
	"context"
	"fmt"
	"sort"
	"time"

	logger "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/perses/perses-operator/api/v1alpha2"
	operatormetrics "github.com/perses/perses-operator/internal/metrics"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

type persesContextKey string

const contextKey persesContextKey = "perses"

func withPerses(ctx context.Context, p *v1alpha2.Perses) context.Context {
	return context.WithValue(ctx, contextKey, p)
}

func persesFromContext(ctx context.Context) (*v1alpha2.Perses, bool) {
	p, ok := ctx.Value(contextKey).(*v1alpha2.Perses)
	return p, ok
}

type Config struct {
	PersesImage          string
	TLSMinVersion        string
	TLSCipherSuites      string
	TLSConfigureOperands bool
}

// PersesReconciler reconciles a Perses object
type PersesReconciler struct {
	client.Client
	APIReader              client.Reader // uncached reader for Secret data (cached client strips Data via Transform)
	Scheme                 *runtime.Scheme
	Recorder               record.EventRecorder
	Config                 Config
	Metrics                *operatormetrics.Metrics
	ReconciliationTracker  *operatormetrics.ReconciliationTracker
	ClientCacheInvalidator common.PersesClientCacheInvalidator
}

var log = logger.WithField("module", "perses_controller")

// +kubebuilder:rbac:groups=perses.dev,resources=perses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=perses.dev,resources=perses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=perses.dev,resources=perses/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=config.openshift.io,resources=apiservers,verbs=get;list;watch
func (r *PersesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	objKey := req.String()

	if r.Metrics != nil {
		r.Metrics.ReconcileOperations("perses").Inc()
	}

	perses := &v1alpha2.Perses{}
	if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("perses resource not found. Ignoring since object must be deleted")
			if r.ReconciliationTracker != nil {
				r.ReconciliationTracker.ForgetObject(objKey)
			}
			if r.Metrics != nil {
				r.Metrics.ForgetObject(objKey)
				r.Metrics.DeletePersesInstance(req.Namespace, req.Name)
			}
			if r.ClientCacheInvalidator != nil {
				r.ClientCacheInvalidator.ForgetInstance(objKey)
			}
			return subreconciler.Evaluate(subreconciler.DoNotRequeue())
		}
		log.WithError(err).Error("Failed to get perses")
		if r.Metrics != nil {
			r.Metrics.ReconcileErrors("perses", "get_failed").Inc()
		}
		return subreconciler.Evaluate(subreconciler.RequeueWithError(err))
	}

	// Update Perses instance count (skip for objects being deleted)
	if r.Metrics != nil && perses.GetDeletionTimestamp() == nil {
		r.Metrics.PersesInstances(perses.Namespace, perses.Name).Set(1)
	}

	// Store perses in context for all sub-reconcilers
	ctx = withPerses(ctx, perses)

	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.handleDelete,
		r.setStatusToUnknown,
		r.removeFinalizer,
		r.validateVolumes,
		r.reconcileProvisioning,
		r.reconcileService,
		r.reconcileConfigMap,
		r.reconcileDeployment,
		r.reconcileStatefulSet,
		r.setStatusToComplete,
	}

	// Run all subreconcilers sequentially
	var reconcileErr error
	for _, f := range subreconcilersForPerses {
		if r, err := f(ctx, req); subreconciler.ShouldHaltOrRequeue(r, err) {
			reconcileErr = err
			break
		}
	}

	// Track reconciliation status
	if r.ReconciliationTracker != nil {
		r.ReconciliationTracker.SetStatus(objKey, reconcileErr)
		if reconcileErr == nil {
			r.ReconciliationTracker.SetReasonAndMessage(objKey, "ReconciliationSuccessful", "Perses instance reconciled successfully")
		}
	}

	// Track metrics
	if r.Metrics != nil {
		if reconcileErr != nil {
			r.Metrics.ReconcileErrors("perses", "reconciliation_failed").Inc()
			r.Metrics.SetFailedResources(objKey, "perses", 1)
		} else {
			r.Metrics.SetSyncedResources(objKey, "perses", 1)
		}
	}

	if reconcileErr != nil {
		return subreconciler.Evaluate(subreconciler.RequeueWithError(reconcileErr))
	}

	log.WithField("duration", time.Since(start)).Debug("reconciliation completed")
	return subreconciler.Evaluate(subreconciler.DoNotRequeue())
}

func (r *PersesReconciler) updatePersesStatus(
	ctx context.Context,
	req ctrl.Request,
	updateFn func(*v1alpha2.Perses),
) (*ctrl.Result, error) {
	_, ok := persesFromContext(ctx)
	if !ok {
		log.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fresh := &v1alpha2.Perses{}
		if err := r.Get(ctx, req.NamespacedName, fresh); err != nil {
			return err
		}
		before := fresh.Status.DeepCopy()
		updateFn(fresh)
		if equality.Semantic.DeepEqual(*before, fresh.Status) {
			return nil
		}
		return r.Status().Update(ctx, fresh)
	})

	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("perses resource not found. Ignoring since object must be deleted")
			return subreconciler.DoNotRequeue()
		}
		log.WithError(err).Error("Failed to update Perses status")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) setStatusToUnknown(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	_, ok := persesFromContext(ctx)
	if !ok {
		log.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	needsRequeue := false
	result, err := r.updatePersesStatus(ctx, req, func(p *v1alpha2.Perses) {
		// Only set status to Unknown when no status conditions exist yet
		if len(p.Status.Conditions) == 0 {
			meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{
				Type: common.TypeAvailablePerses, Status: metav1.ConditionUnknown,
				Reason: "Reconciling", Message: "Starting reconciliation"})
			needsRequeue = true
		}
	})
	if err != nil {
		return result, err
	}

	if needsRequeue {
		// requeue after adding unknown status to allow the initial reconciliation to proceed
		return subreconciler.RequeueWithDelay(time.Second * 5)
	}

	return subreconciler.ContinueReconciling()
}

// removeFinalizer strips the legacy finalizer from existing resources.
// Owner references handle child resource cleanup via cascading deletion,
// so the finalizer is unnecessary. This migration step ensures resources
// upgraded from earlier versions can be deleted cleanly.
func (r *PersesReconciler) removeFinalizer(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		log.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	if controllerutil.ContainsFinalizer(perses, common.PersesFinalizer) {
		log.Info("Removing legacy finalizer from Perses resource")

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
				return err
			}

			if ok := controllerutil.RemoveFinalizer(perses, common.PersesFinalizer); !ok {
				return fmt.Errorf("failed to remove finalizer from the custom resource")
			}

			return r.Update(ctx, perses)
		})

		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("perses resource not found. Ignoring since object must be deleted")
				return subreconciler.DoNotRequeue()
			}
			log.WithError(err).Error("Failed to remove legacy finalizer from custom resource")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) handleDelete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		log.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	if perses.GetDeletionTimestamp() != nil {
		// Strip the legacy finalizer if still present so the resource is not stuck.
		if controllerutil.ContainsFinalizer(perses, common.PersesFinalizer) {
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
					return err
				}
				controllerutil.RemoveFinalizer(perses, common.PersesFinalizer)
				return r.Update(ctx, perses)
			})
			if err != nil {
				log.WithError(err).Error("Failed to remove legacy finalizer during deletion")
				return subreconciler.RequeueWithError(err)
			}
		}

		log.Info("Perses resource is being deleted, skipping reconciliation")
		return subreconciler.DoNotRequeue()
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) validateVolumes(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		log.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	if len(perses.Spec.VolumeMounts) > 0 {
		volumeNames := make(map[string]struct{}, len(perses.Spec.Volumes))
		for _, v := range perses.Spec.Volumes {
			volumeNames[v.Name] = struct{}{}
		}
		for _, m := range perses.Spec.VolumeMounts {
			if _, exists := volumeNames[m.Name]; !exists {
				msg := fmt.Sprintf("volumeMount at %q references volume %q which is not defined in spec.volumes", m.MountPath, m.Name)
				log.WithField("volumeMount", m.Name).WithField("mountPath", m.MountPath).Error("volumeMount references undefined volume")
				_, err := r.updatePersesStatus(ctx, req, func(p *v1alpha2.Perses) {
					meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{
						Type:    common.TypeDegradedPerses,
						Status:  metav1.ConditionTrue,
						Reason:  "InvalidConfiguration",
						Message: msg,
					})
				})
				if err != nil {
					return subreconciler.RequeueWithError(err)
				}
				return subreconciler.RequeueWithError(fmt.Errorf("%s", msg))
			}
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) setStatusToComplete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	return r.updatePersesStatus(ctx, req, func(perses *v1alpha2.Perses) {
		meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{
			Type: common.TypeDegradedPerses, Status: metav1.ConditionFalse,
			Reason: "Reconciled", Message: fmt.Sprintf("Perses (%s) reconciled successfully", perses.Name)})
		meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{
			Type: common.TypeAvailablePerses, Status: metav1.ConditionTrue,
			Reason: "Reconciled", Message: fmt.Sprintf("Perses (%s) created successfully", perses.Name)})
	})
}

func (r *PersesReconciler) reconcileProvisioning(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		log.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	if perses.Spec.Provisioning == nil || len(perses.Spec.Provisioning.SecretRefs) == 0 {
		// If no provisioning secrets are defined, ensure the status is empty
		return r.updatePersesStatus(ctx, req, func(p *v1alpha2.Perses) {
			p.Status.Provisioning = []v1alpha2.SecretVersion{}
		})
	}

	// Use a map to deduplicate secrets and avoid redundant API calls
	// (provisioning secrets may be referenced more than once for different keys)
	secretVersionMap := make(map[string]string) // name -> resourceVersion
	for _, secretRef := range perses.Spec.Provisioning.SecretRefs {
		if _, seen := secretVersionMap[secretRef.Name]; seen {
			continue
		}

		secretName := types.NamespacedName{
			Namespace: perses.Namespace,
			Name:      secretRef.Name,
		}

		actualSecret := &corev1.Secret{}
		err := r.APIReader.Get(ctx, secretName, actualSecret)
		if err != nil {
			log.WithError(err).Errorf("Failed to get provisioning secret %s", secretName.String())
			return subreconciler.RequeueWithError(err)
		}
		secretVersionMap[secretRef.Name] = actualSecret.ResourceVersion
	}

	newSecretVersions := make([]v1alpha2.SecretVersion, 0, len(secretVersionMap))
	for name, version := range secretVersionMap {
		newSecretVersions = append(newSecretVersions, v1alpha2.SecretVersion{
			Name:    name,
			Version: version,
		})
	}

	sort.Slice(newSecretVersions, func(i, j int) bool {
		return newSecretVersions[i].Name < newSecretVersions[j].Name
	})

	if equality.Semantic.DeepEqual(newSecretVersions, perses.Status.Provisioning) {
		return subreconciler.ContinueReconciling()
	}

	return r.updatePersesStatus(ctx, req, func(p *v1alpha2.Perses) {
		p.Status.Provisioning = newSecretVersions
	})
}

func (r *PersesReconciler) findPersesForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	// List all Perses objects in the same namespace as the secret
	persesList := &v1alpha2.PersesList{}
	if err := r.List(ctx, persesList, client.InNamespace(obj.GetNamespace())); err != nil {
		log.WithError(err).Errorf("failed to list Perses instances for secret %s/%s", obj.GetNamespace(), obj.GetName())
		return nil
	}

	var requests []reconcile.Request
	for _, perses := range persesList.Items {
		if perses.Spec.Provisioning == nil || len(perses.Spec.Provisioning.SecretRefs) == 0 {
			continue
		}
		for _, ref := range perses.Spec.Provisioning.SecretRefs {
			if ref.Name == obj.GetName() {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      perses.Name,
						Namespace: perses.Namespace,
					},
				})
				break // found a match for this Perses instance, move to next
			}
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.Perses{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		// WatchesMetadata only caches metadata (not Data) to reduce memory.
		// Actual secret data is read via APIReader in reconcileProvisioning.
		WatchesMetadata(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findPersesForSecret),
		).
		Complete(r)
}
