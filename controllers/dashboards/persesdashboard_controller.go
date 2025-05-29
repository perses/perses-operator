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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.handleDelete,
		r.setStatusToUnknown,
		r.reconcileDashboardInAllInstances,
		r.updateStatus,
	}

	for _, f := range subreconcilersForPerses {
		if r, err := f(ctx, req); subreconciler.ShouldHaltOrRequeue(r, err) {
			return subreconciler.Evaluate(r, err)
		}
	}

	return subreconciler.Evaluate(subreconciler.DoNotRequeue())
}

func (r *PersesDashboardReconciler) getLatestPersesDashboard(ctx context.Context, req ctrl.Request, dashboard *persesv1alpha1.PersesDashboard) (*ctrl.Result, error) {
	if err := r.Get(ctx, req.NamespacedName, dashboard); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("perses dashboard resource not found. Ignoring since object must be deleted")
			return subreconciler.DoNotRequeue()
		}
		log.WithError(err).Error("Failed to get perses dashboard")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDashboardReconciler) handleDelete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	dashboard := &persesv1alpha1.PersesDashboard{}

	if err := r.Get(ctx, req.NamespacedName, dashboard); err != nil {
		if !apierrors.IsNotFound(err) {
			dlog.Info("No Perses instances found, retrying in 1 minute")
			return subreconciler.RequeueWithDelay(time.Minute)
		}

		log.Infof("perses dashboard resource not found. Deleting '%s' in '%s'", req.Name, req.Namespace)

		return r.deleteDashboardInAllInstances(ctx, req, req.Namespace, req.Name)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDashboardReconciler) setStatusToUnknown(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	dashboard := &persesv1alpha1.PersesDashboard{}

	if r, err := r.getLatestPersesDashboard(ctx, req, dashboard); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	if len(dashboard.Status.Conditions) == 0 {
		meta.SetStatusCondition(&dashboard.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, dashboard); err != nil {
			log.WithError(err).Error("Failed to update Perses dashboard status")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDashboardReconciler) updateStatus(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	dashboard := &persesv1alpha1.PersesDashboard{}

	if r, err := r.getLatestPersesDashboard(ctx, req, dashboard); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	meta.SetStatusCondition(&dashboard.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Dashboard (%s) created successfully", dashboard.Name)})

	if err := r.Status().Update(ctx, dashboard); err != nil {
		log.Error(err, "Failed to update Perses dashboard status")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&persesv1alpha1.PersesDashboard{}).
		Complete(r)
}
