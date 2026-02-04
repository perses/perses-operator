/*
Copyright 2023 The Perses Authors.

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

package perses

import (
	"context"
	"fmt"
	"time"

	logger "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/perses/perses-operator/api/v1alpha2"
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
	PersesImage string
}

// PersesReconciler reconciles a Perses object
type PersesReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Config   Config
}

var log = logger.WithField("module", "perses_controller")

// +kubebuilder:rbac:groups=perses.dev,resources=perses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=perses.dev,resources=perses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=perses.dev,resources=perses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=perses.dev,resources=perses/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
func (r *PersesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	perses := &v1alpha2.Perses{}
	if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("perses resource not found. Ignoring since object must be deleted")
			return subreconciler.Evaluate(subreconciler.DoNotRequeue())
		}
		log.WithError(err).Error("Failed to get perses")
		return subreconciler.Evaluate(subreconciler.RequeueWithError(err))
	}

	// Store perses in context for all sub-reconcilers
	ctx = withPerses(ctx, perses)

	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.handleDelete,
		r.setStatusToUnknown,
		r.addFinalizer,
		r.reconcileService,
		r.reconcileConfigMap,
		r.reconcileDeployment,
		r.reconcileStatefulSet,
		r.setStatusToComplete,
	}

	// Run all subreconcilers sequentially
	for _, f := range subreconcilersForPerses {
		if r, err := f(ctx, req); subreconciler.ShouldHaltOrRequeue(r, err) {
			return subreconciler.Evaluate(r, err)
		}
	}

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
		updateFn(fresh)
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
		// requeue after adding unknown status to allow adding the finalizer to succeed
		// see explanation on setting a status on creation here
		// https://github.com/kubernetes-sigs/controller-runtime/blob/1dce6213f6c078f3170921b3a774304d066d5bd4/pkg/controller/controllerutil/controllerutil.go#L378
		return subreconciler.RequeueWithDelay(time.Second * 5)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) addFinalizer(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		log.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(perses, common.PersesFinalizer) {
		log.Info("Adding Finalizer for Perses")

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Re-fetch the perses Custom Resource before update
			// so that we have the latest state of the resource on the cluster
			if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
				return err
			}

			if ok := controllerutil.AddFinalizer(perses, common.PersesFinalizer); !ok {
				return fmt.Errorf("failed to add finalizer into the custom resource")
			}

			return r.Update(ctx, perses)
		})

		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("perses resource not found. Ignoring since object must be deleted")
				return subreconciler.DoNotRequeue()
			}
			log.WithError(err).Error("Failed to update custom resource to add finalizer")
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

	// Check if the Perses instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isPersesMarkedToBeDeleted := perses.GetDeletionTimestamp() != nil
	if isPersesMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(perses, common.PersesFinalizer) {
			log.Info("Performing Finalizer Operations for Perses before delete CR")

			// Set status to indicate finalization is in progress
			_, err := r.updatePersesStatus(ctx, req, func(p *v1alpha2.Perses) {
				meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{
					Type: common.TypeDegradedPerses, Status: metav1.ConditionUnknown,
					Reason: "Finalizing", Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", p.Name)})
			})
			if err != nil {
				return subreconciler.RequeueWithError(err)
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperationsForPerses(perses)

			// TODO(user): If you add operations to the doFinalizerOperationsForPerses method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Update status to indicate finalization is complete
			_, err = r.updatePersesStatus(ctx, req, func(p *v1alpha2.Perses) {
				meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{
					Type: common.TypeDegradedPerses, Status: metav1.ConditionTrue,
					Reason: "Finalizing", Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", p.Name)})
			})
			if err != nil {
				return subreconciler.RequeueWithError(err)
			}

			// Remove finalizer in a separate retry block
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
					return err
				}

				log.Info("Removing Finalizer for Perses after successfully perform the operations")
				if ok := controllerutil.RemoveFinalizer(perses, common.PersesFinalizer); !ok {
					return fmt.Errorf("failed to remove finalizer for Perses")
				}

				return r.Update(ctx, perses)
			})

			if err != nil {
				log.WithError(err).Error("Failed to remove finalizer for Perses")
				return subreconciler.RequeueWithError(err)
			}
		}

		return subreconciler.DoNotRequeue()
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) doFinalizerOperationsForPerses(perses *v1alpha2.Perses) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended of the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the Deployment will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	// The following implementation will raise an event
	if r.Recorder != nil {
		r.Recorder.Event(perses, "Warning", "Deleting",
			fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
				perses.Name,
				perses.Namespace))
	}
}

func (r *PersesReconciler) setStatusToComplete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	return r.updatePersesStatus(ctx, req, func(perses *v1alpha2.Perses) {
		meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{
			Type: common.TypeAvailablePerses, Status: metav1.ConditionTrue,
			Reason: "Reconciled", Message: fmt.Sprintf("Perses (%s) created successfully", perses.Name)})
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.Perses{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
