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

	logger "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"maps"

	"github.com/perses/perses-operator/api/v1alpha1"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

var slog = logger.WithField("module", "service_controller")

func (r *PersesReconciler) reconcileService(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses := &v1alpha1.Perses{}

	if result, err := r.getLatestPerses(ctx, req, perses); subreconciler.ShouldHaltOrRequeue(result, err) {
		return result, err
	}

	serviceName := perses.Name
	if perses.Spec.Service != nil && len(perses.Spec.Service.Name) > 0 {
		serviceName = perses.Spec.Service.Name
	}

	found := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: perses.Namespace}, found); err != nil {
		if !apierrors.IsNotFound(err) {
			log.WithError(err).Error("Failed to get Service")

			return subreconciler.RequeueWithError(err)
		}

		ser, err2 := r.createPersesService(serviceName, perses)
		if err2 != nil {
			slog.WithError(err2).Error("Failed to define new Service resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Service for the custom resource (%s): (%s)", perses.Name, err2)})

			if err = r.Status().Update(ctx, perses); err != nil {
				slog.Error(err, "Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err2)
		}

		slog.Infof("Creating a new Service: Service.Namespace %s Service.Name %s", ser.Namespace, ser.Name)
		if err = r.Create(ctx, ser); err != nil {
			slog.WithError(err).Errorf("Failed to create new Service: Service.Namespace %s Service.Name %s", ser.Namespace, ser.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.ContinueReconciling()
	}

	svc, err := r.createPersesService(serviceName, perses)
	if err != nil {
		slog.WithError(err).Error("Failed to define new Service resource for perses")
		return subreconciler.RequeueWithError(err)
	}

	// call update with dry run to fill out fields that are also returned via the k8s api
	if err = r.Update(ctx, svc, client.DryRunAll); err != nil {
		slog.Error(err, "Failed to update Service with dry run")
		return subreconciler.RequeueWithError(err)
	}

	if serviceNeedsUpdate(found, svc, perses.Name, perses) {
		if err = r.Update(ctx, svc); err != nil {
			slog.Error(err, "Failed to update Service")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func serviceNeedsUpdate(existing, updated *corev1.Service, name string, perses *v1alpha1.Perses) bool {
	if existing == nil && updated == nil {
		return false
	}
	if existing == nil || updated == nil {
		return true
	}
	if existing.Name != updated.Name || existing.Namespace != updated.Namespace {
		return true
	}
	if !equality.Semantic.DeepEqual(existing.Spec.Type, updated.Spec.Type) ||
		!equality.Semantic.DeepEqual(existing.Spec.Ports, updated.Spec.Ports) ||
		!equality.Semantic.DeepEqual(existing.Annotations, updated.Annotations) {
		return true
	}

	// check for differences only in the labels that are set by the operator
	labels := common.LabelsForPerses(name, perses)

	for k := range labels {
		if existing.Labels[k] != updated.Labels[k] {
			return true
		}
		if existing.Spec.Selector[k] != updated.Spec.Selector[k] {
			return true
		}
	}

	return false
}

func (r *PersesReconciler) createPersesService(
	serviceName string,
	perses *v1alpha1.Perses) (*corev1.Service, error) {
	ls := common.LabelsForPerses(perses.Name, perses)

	annotations := map[string]string{}
	if perses.Spec.Metadata != nil && perses.Spec.Metadata.Annotations != nil {
		annotations = perses.Spec.Metadata.Annotations
	}

	if perses.Spec.Service != nil && perses.Spec.Service.Annotations != nil {
		maps.Copy(annotations, perses.Spec.Service.Annotations)
	}

	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   perses.Namespace,
			Annotations: annotations,
			Labels:      ls,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8080,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(8080),
			}},
			Selector: ls,
		},
	}

	// Set the ownerRef for the Service
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(perses, ser, r.Scheme); err != nil {
		return nil, err
	}
	return ser, nil
}
