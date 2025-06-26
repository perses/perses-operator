/*
Copyright 2025.

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

package globaldatasources

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

// PersesGlobalDatasourceReconciler reconciles a PersesDatasource object
type PersesGlobalDatasourceReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	ClientFactory common.PersesClientFactory
}

var log = logger.WithField("module", "perses_globaldatasource_controller")

// +kubebuilder:rbac:groups=perses.dev,resources=persesglobaldatasources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=perses.dev,resources=persesglobaldatasources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=perses.dev,resources=persesglobaldatasources/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=watch;get
func (r *PersesGlobalDatasourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Infof("Reconciling PersesGlobalDatasource: %s/%s", req.Namespace, req.Name)
	subreconcilersForPerses := []subreconciler.FnWithRequest{
		r.handleDelete,
		r.setStatusToUnknown,
		r.reconcileGlobalDatasourcesInAllInstances,
		r.updateStatus,
	}

	for _, f := range subreconcilersForPerses {
		if r, err := f(ctx, req); subreconciler.ShouldHaltOrRequeue(r, err) {
			return subreconciler.Evaluate(r, err)
		}
	}

	return subreconciler.Evaluate(subreconciler.DoNotRequeue())
}

func (r *PersesGlobalDatasourceReconciler) getLatestPersesGlobalDatasource(ctx context.Context, req ctrl.Request, globaldatasource *persesv1alpha2.PersesGlobalDatasource) (*ctrl.Result, error) {
	if err := r.Get(ctx, req.NamespacedName, globaldatasource); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("perses globaldatasource resource not found. Ignoring since object must be deleted")
			return subreconciler.DoNotRequeue()
		}
		log.WithError(err).Error("Failed to get perses globaldatasource")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesGlobalDatasourceReconciler) handleDelete(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	globaldatasource := &persesv1alpha2.PersesGlobalDatasource{}

	if err := r.Get(ctx, req.NamespacedName, globaldatasource); err != nil {
		if !apierrors.IsNotFound(err) {
			log.WithError(err).Error("Failed to get perses globaldatasource")
			return subreconciler.RequeueWithError(err)
		}

		log.Infof("perses globaldatasource resource not found. Deleting '%s' in '%s'", req.Name, req.Namespace)

		return r.deleteGlobalDatasourceInAllInstances(ctx, req.Namespace, req.Name)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesGlobalDatasourceReconciler) setStatusToUnknown(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	globaldatasource := &persesv1alpha2.PersesGlobalDatasource{}

	if r, err := r.getLatestPersesGlobalDatasource(ctx, req, globaldatasource); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	if len(globaldatasource.Status.Conditions) == 0 {
		meta.SetStatusCondition(&globaldatasource.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, globaldatasource); err != nil {
			log.WithError(err).Error("Failed to update Perses globaldatasource status")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesGlobalDatasourceReconciler) updateStatus(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	globaldatasource := &persesv1alpha2.PersesGlobalDatasource{}

	if r, err := r.getLatestPersesGlobalDatasource(ctx, req, globaldatasource); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	meta.SetStatusCondition(&globaldatasource.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("GlobalDatasource (%s) created successfully", globaldatasource.Name)})

	if err := r.Status().Update(ctx, globaldatasource); err != nil {
		log.Error(err, "Failed to update Perses globaldatasource status")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesGlobalDatasourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&persesv1alpha2.PersesGlobalDatasource{}).
		Complete(r)
}
