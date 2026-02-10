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

package datasources

import (
	"context"
	"fmt"
	"time"

	logger "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	operatormetrics "github.com/perses/perses-operator/internal/metrics"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

type datasourceContextKey string

const contextKey datasourceContextKey = "datasource"

func withDatasource(ctx context.Context, ds *persesv1alpha2.PersesDatasource) context.Context {
	return context.WithValue(ctx, contextKey, ds)
}

func datasourceFromContext(ctx context.Context) (*persesv1alpha2.PersesDatasource, bool) {
	ds, ok := ctx.Value(contextKey).(*persesv1alpha2.PersesDatasource)
	return ds, ok
}

// PersesDatasourceReconciler reconciles a PersesDatasource object
type PersesDatasourceReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	ClientFactory         common.PersesClientFactory
	Metrics               *operatormetrics.Metrics
	ReconciliationTracker *operatormetrics.ReconciliationTracker
}

var log = logger.WithField("module", "perses_datasource_controller")

// +kubebuilder:rbac:groups=perses.dev,resources=persesdatasources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=perses.dev,resources=persesdatasources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=perses.dev,resources=persesdatasources/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=watch;get
func (r *PersesDatasourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	objKey := req.String()

	log.Infof("Reconciling PersesDatasource: %s/%s", req.Namespace, req.Name)

	// Find once and store in context for all sub-reconcilers, handle deletion if not found
	datasource := &persesv1alpha2.PersesDatasource{}
	if err := r.Get(ctx, req.NamespacedName, datasource); err != nil {
		if apierrors.IsNotFound(err) {
			log.Infof("perses datasource resource not found. Deleting '%s' in '%s'", req.Name, req.Namespace)
			if r.ReconciliationTracker != nil {
				r.ReconciliationTracker.ForgetObject(objKey)
			}
			return subreconciler.Evaluate(r.deleteDatasourceInAllInstances(ctx, req.Namespace, req.Name))
		}
		log.WithError(err).Error("Failed to get perses datasource")
		if r.Metrics != nil {
			r.Metrics.ReconcileErrors("persesdatasource", "get_failed").Inc()
		}
		return subreconciler.Evaluate(subreconciler.RequeueWithError(err))
	}

	ctx = withDatasource(ctx, datasource)

	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.setStatusToUnknown,
		r.reconcileDatasourcesInAllInstances,
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
			r.ReconciliationTracker.SetReasonAndMessage(objKey, "ReconciliationSuccessful", "Datasource reconciled successfully")
		}
	}

	// Track metrics
	if r.Metrics != nil {
		if reconcileErr != nil {
			r.Metrics.ReconcileErrors("persesdatasource", "reconciliation_failed").Inc()
			r.Metrics.SetFailedResources(objKey, "datasource", 1)
		} else {
			r.Metrics.SetSyncedResources(objKey, "datasource", 1)
		}
	}

	if reconcileErr != nil {
		return subreconciler.Evaluate(subreconciler.RequeueWithError(reconcileErr))
	}

	log.WithField("duration", time.Since(start)).Debug("datasource reconciliation completed")
	return subreconciler.Evaluate(subreconciler.DoNotRequeue())
}

func (r *PersesDatasourceReconciler) updateDatasourceStatus(
	ctx context.Context,
	req ctrl.Request,
	updateFn func(*persesv1alpha2.PersesDatasource),
) (*ctrl.Result, error) {
	_, ok := datasourceFromContext(ctx)
	if !ok {
		log.Error("datasource not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("datasource not found in context"))
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fresh := &persesv1alpha2.PersesDatasource{}
		if err := r.Get(ctx, req.NamespacedName, fresh); err != nil {
			return err
		}
		updateFn(fresh)
		return r.Status().Update(ctx, fresh)
	})

	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("perses datasource resource not found. Ignoring since object must be deleted")
			return subreconciler.DoNotRequeue()
		}
		log.WithError(err).Error("Failed to update Perses datasource status")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDatasourceReconciler) setStatusToUnknown(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	return r.updateDatasourceStatus(ctx, req, func(datasource *persesv1alpha2.PersesDatasource) {
		if len(datasource.Status.Conditions) == 0 {
			meta.SetStatusCondition(&datasource.Status.Conditions, metav1.Condition{
				Type: common.TypeAvailablePerses, Status: metav1.ConditionUnknown,
				Reason: "Reconciling", Message: "Starting reconciliation"})
		}
	})
}

func (r *PersesDatasourceReconciler) setStatusToComplete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	return r.updateDatasourceStatus(ctx, req, func(datasource *persesv1alpha2.PersesDatasource) {
		meta.SetStatusCondition(&datasource.Status.Conditions, metav1.Condition{
			Type: common.TypeAvailablePerses, Status: metav1.ConditionTrue,
			Reason: "Reconciled", Message: fmt.Sprintf("Datasource (%s) created successfully", datasource.Name)})
	})
}

func (r *PersesDatasourceReconciler) setStatusToDegraded(
	ctx context.Context,
	req ctrl.Request,
	degradedResult *ctrl.Result,
	degradedReason common.ConditionStatusReason,
	degradedError error,
) (*ctrl.Result, error) {
	result, err := r.updateDatasourceStatus(ctx, req, func(datasource *persesv1alpha2.PersesDatasource) {
		meta.SetStatusCondition(&datasource.Status.Conditions, metav1.Condition{
			Type: common.TypeDegradedPerses, Status: metav1.ConditionTrue,
			Reason: string(degradedReason), Message: degradedError.Error()})
	})

	if err != nil {
		return result, err
	}

	return degradedResult, degradedError
}

func (r *PersesDatasourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&persesv1alpha2.PersesDatasource{}).
		Complete(r)
}
