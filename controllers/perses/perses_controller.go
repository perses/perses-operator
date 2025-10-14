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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

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
	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.handleDelete,
		r.setStatusToUnknown,
		r.addFinalizer,
		r.reconcileService,
		r.reconcileConfigMap,
		r.reconcileDeployment,
		r.reconcileStatefulSet,
	}

	// Run all subreconcilers sequentially
	for _, f := range subreconcilersForPerses {
		if r, err := f(ctx, req); subreconciler.ShouldHaltOrRequeue(r, err) {
			return subreconciler.Evaluate(r, err)
		}
	}

	return subreconciler.Evaluate(subreconciler.DoNotRequeue())
}

func (r *PersesReconciler) getLatestPerses(ctx context.Context, req ctrl.Request, perses *v1alpha2.Perses) (*ctrl.Result, error) {
	// Fetch the latest version of the perses resource
	if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("perses resource not found. Ignoring since object must be deleted")
			return subreconciler.DoNotRequeue()
		}
		// Error reading the object - requeue the request.
		log.WithError(err).Error("Failed to get perses")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) setStatusToUnknown(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses := &v1alpha2.Perses{}

	// Fetch the latest Perses
	// If this fails, bubble up the reconcile results to the main reconciler
	if r, err := r.getLatestPerses(ctx, req, perses); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	// Let's just set the status as Unknown when no status are available
	if len(perses.Status.Conditions) == 0 {
		meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})

		if err := r.Status().Update(ctx, perses); err != nil {
			log.WithError(err).Error("Failed to update Perses status")
			return subreconciler.RequeueWithError(err)
		}

		// requeue after adding unknown status to allow adding the finalizer to succeed
		// see explanation on setting a status on creation here
		// https://github.com/kubernetes-sigs/controller-runtime/blob/1dce6213f6c078f3170921b3a774304d066d5bd4/pkg/controller/controllerutil/controllerutil.go#L378
		return subreconciler.RequeueWithDelay(time.Second * 5)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) addFinalizer(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses := &v1alpha2.Perses{}

	// Fetch the latest Perses
	// If this fails, bubble up the reconcile results to the main reconciler
	if r, err := r.getLatestPerses(ctx, req, perses); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(perses, common.PersesFinalizer) {
		log.Info("Adding Finalizer for Perses")

		// Re-fetch the perses Custom Resource before update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
			log.WithError(err).Error("Failed to re-fetch perses")
			return subreconciler.RequeueWithError(err)
		}

		if ok := controllerutil.AddFinalizer(perses, common.PersesFinalizer); !ok {
			log.Error("Failed to add finalizer into the custom resource")
			return subreconciler.RequeueWithDelay(time.Second * 10)
		}

		if err := r.Update(ctx, perses); err != nil {
			log.WithError(err).Error("Failed to update custom resource to add finalizer")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) handleDelete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses := &v1alpha2.Perses{}

	// Fetch the latest Perses
	// If this fails, bubble up the reconcile results to the main reconciler
	if r, err := r.getLatestPerses(ctx, req, perses); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	// Check if the Perses instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isPersesMarkedToBeDeleted := perses.GetDeletionTimestamp() != nil
	if isPersesMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(perses, common.PersesFinalizer) {
			log.Info("Performing Finalizer Operations for Perses before delete CR")

			// Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeDegradedPerses,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", perses.Name)})

			if err := r.Status().Update(ctx, perses); err != nil {
				log.WithError(err).Error("Failed to update Perses status")
				return subreconciler.RequeueWithError(err)
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperationsForPerses(perses)

			// TODO(user): If you add operations to the doFinalizerOperationsForPerses method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the perses Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, perses); err != nil {
				log.WithError(err).Error("Failed to re-fetch perses")
				return subreconciler.RequeueWithError(err)
			}

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeDegradedPerses,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", perses.Name)})

			if err := r.Status().Update(ctx, perses); err != nil {
				log.WithError(err).Error("Failed to update Perses status")
				return subreconciler.RequeueWithError(err)
			}

			log.Info("Removing Finalizer for Perses after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(perses, common.PersesFinalizer); !ok {
				log.Error(nil, "Failed to remove finalizer for Perses")
				return subreconciler.RequeueWithDelay(time.Second * 10)
			}

			if err := r.Update(ctx, perses); err != nil {
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
