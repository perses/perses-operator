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

	logger "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

// PersesDatasourceReconciler reconciles a PersesDatasource object
type PersesDatasourceReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	ClientFactory common.PersesClientFactory
}

var log = logger.WithField("module", "perses_datasource_controller")

// +kubebuilder:rbac:groups=perses.dev,resources=persesdatasources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=perses.dev,resources=persesdatasources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=perses.dev,resources=persesdatasources/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=watch;get
func (r *PersesDatasourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Infof("Reconciling PersesDatasource: %s/%s", req.Namespace, req.Name)
	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.handleDelete,
		r.setStatusToUnknown,
		r.reconcileDatasourcesInAllInstances,
		r.updateStatus,
	}

	for _, f := range subreconcilersForPerses {
		if r, err := f(ctx, req); subreconciler.ShouldHaltOrRequeue(r, err) {
			return subreconciler.Evaluate(r, err)
		}
	}

	return subreconciler.Evaluate(subreconciler.DoNotRequeue())
}

func (r *PersesDatasourceReconciler) getLatestPersesDatasource(ctx context.Context, req ctrl.Request, datasource *persesv1alpha2.PersesDatasource) (*ctrl.Result, error) {
	if err := r.Get(ctx, req.NamespacedName, datasource); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("perses datasource resource not found. Ignoring since object must be deleted")
			return subreconciler.DoNotRequeue()
		}
		log.WithError(err).Error("Failed to get perses datasource")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDatasourceReconciler) handleDelete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	datasource := &persesv1alpha2.PersesDatasource{}

	if err := r.Get(ctx, req.NamespacedName, datasource); err != nil {
		if !apierrors.IsNotFound(err) {
			log.WithError(err).Error("Failed to get perses datasource")
			return subreconciler.RequeueWithError(err)
		}

		log.Infof("perses datasource resource not found. Deleting '%s' in '%s'", req.Name, req.Namespace)

		return r.deleteDatasourceInAllInstances(ctx, req.Namespace, req.Name)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDatasourceReconciler) setStatusToUnknown(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	datasource := &persesv1alpha2.PersesDatasource{}

	if r, err := r.getLatestPersesDatasource(ctx, req, datasource); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	if len(datasource.Status.Conditions) == 0 {
		meta.SetStatusCondition(&datasource.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, datasource); err != nil {
			log.WithError(err).Error("Failed to update Perses datasource status")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDatasourceReconciler) updateStatus(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	datasource := &persesv1alpha2.PersesDatasource{}

	if r, err := r.getLatestPersesDatasource(ctx, req, datasource); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	meta.SetStatusCondition(&datasource.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Datasource (%s) created successfully", datasource.Name)})

	if err := r.Status().Update(ctx, datasource); err != nil {
		log.Error(err, "Failed to update Perses datasource status")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDatasourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&persesv1alpha2.PersesDatasource{}).
		Complete(r)
}
