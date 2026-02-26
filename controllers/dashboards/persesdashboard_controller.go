/*
Copyright 2024.

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

package dashboards

import (
	"context"
	"fmt"
	"time"

	logger "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	operatormetrics "github.com/perses/perses-operator/internal/metrics"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

type dashboardContextKey string

const contextKey dashboardContextKey = "dashboard"

func withDashboard(ctx context.Context, db *persesv1alpha2.PersesDashboard) context.Context {
	return context.WithValue(ctx, contextKey, db)
}

func dashboardFromContext(ctx context.Context) (*persesv1alpha2.PersesDashboard, bool) {
	db, ok := ctx.Value(contextKey).(*persesv1alpha2.PersesDashboard)
	return db, ok
}

// PersesDashboardReconciler reconciles a PersesDashboard object
type PersesDashboardReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	ClientFactory         common.PersesClientFactory
	Metrics               *operatormetrics.Metrics
	ReconciliationTracker *operatormetrics.ReconciliationTracker
}

var log = logger.WithField("module", "perses_dashboards_controller")

// +kubebuilder:rbac:groups=perses.dev,resources=persesdashboards,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=perses.dev,resources=persesdashboards/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=perses.dev,resources=persesdashboards/finalizers,verbs=update
func (r *PersesDashboardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	objKey := req.String()

	if r.Metrics != nil {
		r.Metrics.ReconcileOperations("persesdashboard").Inc()
	}

	log.Infof("Reconciling PersesDashboard: %s/%s", req.Namespace, req.Name)

	// Find once and store in context for all sub-reconcilers, handle deletion if not found
	dashboard := &persesv1alpha2.PersesDashboard{}
	if err := r.Get(ctx, req.NamespacedName, dashboard); err != nil {
		if apierrors.IsNotFound(err) {
			log.Infof("perses dashboard resource not found. Deleting '%s' in '%s'", req.Name, req.Namespace)
			if r.ReconciliationTracker != nil {
				r.ReconciliationTracker.ForgetObject(objKey)
			}
			return subreconciler.Evaluate(r.deleteDashboardInAllInstances(ctx, req, req.Namespace, req.Name))
		}
		log.WithError(err).Error("Failed to get perses dashboard")
		if r.Metrics != nil {
			r.Metrics.ReconcileErrors("persesdashboard", "get_failed").Inc()
		}
		return subreconciler.Evaluate(subreconciler.RequeueWithError(err))
	}

	ctx = withDashboard(ctx, dashboard)

	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.setStatusToUnknown,
		r.reconcileDashboardInAllInstances,
		r.setStatusToComplete,
	}

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
			r.ReconciliationTracker.SetReasonAndMessage(objKey, "ReconciliationSuccessful", "Dashboard reconciled successfully")
		}
	}

	// Track metrics
	if r.Metrics != nil {
		if reconcileErr != nil {
			r.Metrics.ReconcileErrors("persesdashboard", "reconciliation_failed").Inc()
			r.Metrics.SetFailedResources(objKey, "dashboard", 1)
		} else {
			r.Metrics.SetSyncedResources(objKey, "dashboard", 1)
		}
	}

	if reconcileErr != nil {
		return subreconciler.Evaluate(subreconciler.RequeueWithError(reconcileErr))
	}

	log.WithField("duration", time.Since(start)).Debug("dashboard reconciliation completed")
	return subreconciler.Evaluate(subreconciler.DoNotRequeue())
}

func (r *PersesDashboardReconciler) updateDashboardStatus(
	ctx context.Context,
	req ctrl.Request,
	updateFn func(*persesv1alpha2.PersesDashboard),
) (*ctrl.Result, error) {
	_, ok := dashboardFromContext(ctx)
	if !ok {
		log.Error("dashboard not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("dashboard not found in context"))
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fresh := &persesv1alpha2.PersesDashboard{}
		if err := r.Get(ctx, req.NamespacedName, fresh); err != nil {
			return err
		}
		updateFn(fresh)
		return r.Status().Update(ctx, fresh)
	})

	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("perses dashboard resource not found. Ignoring since object must be deleted")
			return subreconciler.DoNotRequeue()
		}
		log.WithError(err).Error("Failed to update Perses dashboard status")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDashboardReconciler) setStatusToUnknown(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	return r.updateDashboardStatus(ctx, req, func(dashboard *persesv1alpha2.PersesDashboard) {
		if len(dashboard.Status.Conditions) == 0 {
			meta.SetStatusCondition(&dashboard.Status.Conditions, metav1.Condition{
				Type: common.TypeAvailablePerses, Status: metav1.ConditionUnknown,
				Reason: "Reconciling", Message: "Starting reconciliation"})
		}
	})
}

func (r *PersesDashboardReconciler) setStatusToComplete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	return r.updateDashboardStatus(ctx, req, func(dashboard *persesv1alpha2.PersesDashboard) {
		meta.SetStatusCondition(&dashboard.Status.Conditions, metav1.Condition{
			Type: common.TypeDegradedPerses, Status: metav1.ConditionFalse,
			Reason: "Reconciled", Message: fmt.Sprintf("Dashboard (%s) reconciled successfully", dashboard.Name)})
		meta.SetStatusCondition(&dashboard.Status.Conditions, metav1.Condition{
			Type: common.TypeAvailablePerses, Status: metav1.ConditionTrue,
			Reason: "Reconciled", Message: fmt.Sprintf("Dashboard (%s) created successfully", dashboard.Name)})
	})
}

func (r *PersesDashboardReconciler) setStatusToDegraded(
	ctx context.Context,
	req ctrl.Request,
	degradedResult *ctrl.Result,
	degradedReason common.ConditionStatusReason,
	degradedError error,
) (*ctrl.Result, error) {
	result, err := r.updateDashboardStatus(ctx, req, func(dashboard *persesv1alpha2.PersesDashboard) {
		meta.SetStatusCondition(&dashboard.Status.Conditions, metav1.Condition{
			Type: common.TypeAvailablePerses, Status: metav1.ConditionFalse,
			Reason: string(degradedReason), Message: degradedError.Error()})
		meta.SetStatusCondition(&dashboard.Status.Conditions, metav1.Condition{
			Type: common.TypeDegradedPerses, Status: metav1.ConditionTrue,
			Reason: string(degradedReason), Message: degradedError.Error()})
	})

	if err != nil {
		return result, err
	}

	return degradedResult, degradedError
}

// findDashboardsForPerses returns reconcile requests for all PersesDashboards
// across all namespaces when a Perses instance becomes available.
// Each dashboard's instanceSelector labels determine which Perses instances it syncs to.
// If no instanceSelector is set, the dashboard syncs to all Perses instances.
func (r *PersesDashboardReconciler) findDashboardsForPerses(ctx context.Context, _ client.Object) []reconcile.Request {
	dashboards := &persesv1alpha2.PersesDashboardList{}
	if err := r.List(ctx, dashboards); err != nil {
		log.WithError(err).Error("Failed to list dashboards for Perses instance change")
		return nil
	}

	requests := make([]reconcile.Request, len(dashboards.Items))
	for i, d := range dashboards.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      d.Name,
				Namespace: d.Namespace,
			},
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
// It watches PersesDashboard resources and also watches Perses instances
// to trigger re-reconciliation of all dashboards when a Perses instance becomes
// available. Dashboards are matched to Perses instances via instanceSelector labels.
// Create and delete events for Perses instances are ignored because
// the instance is not yet ready at creation, and deletion is handled by the dashboard's
// own reconciliation loop.
func (r *PersesDashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&persesv1alpha2.PersesDashboard{}).
		Watches(
			&persesv1alpha2.Perses{},
			handler.EnqueueRequestsFromMapFunc(r.findDashboardsForPerses),
			builder.WithPredicates(common.PersesAvailabilityPredicate()),
		).
		Complete(r)
}
