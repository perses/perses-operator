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

	logger "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
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
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	ClientFactory common.PersesClientFactory
}

var log = logger.WithField("module", "perses_dashboards_controller")

// +kubebuilder:rbac:groups=perses.dev,resources=persesdashboards,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=perses.dev,resources=persesdashboards/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=perses.dev,resources=persesdashboards/finalizers,verbs=update
func (r *PersesDashboardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Infof("Reconciling PersesDashboard: %s/%s", req.Namespace, req.Name)

	// Find once and store in context for all sub-reconcilers, handle deletion if not found
	dashboard := &persesv1alpha2.PersesDashboard{}
	if err := r.Get(ctx, req.NamespacedName, dashboard); err != nil {
		if apierrors.IsNotFound(err) {
			log.Infof("perses dashboard resource not found. Deleting '%s' in '%s'", req.Name, req.Namespace)
			return subreconciler.Evaluate(r.deleteDashboardInAllInstances(ctx, req, req.Namespace, req.Name))
		}
		log.WithError(err).Error("Failed to get perses dashboard")
		return subreconciler.Evaluate(subreconciler.RequeueWithError(err))
	}

	ctx = withDashboard(ctx, dashboard)

	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.setStatusToUnknown,
		r.reconcileDashboardInAllInstances,
		r.setStatusToComplete,
	}

	for _, f := range subreconcilersForPerses {
		if r, err := f(ctx, req); subreconciler.ShouldHaltOrRequeue(r, err) {
			return subreconciler.Evaluate(r, err)
		}
	}

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
			Type: common.TypeDegradedPerses, Status: metav1.ConditionTrue,
			Reason: string(degradedReason), Message: degradedError.Error()})
	})

	if err != nil {
		return result, err
	}

	return degradedResult, degradedError
}

func (r *PersesDashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&persesv1alpha2.PersesDashboard{}).
		Complete(r)
}
