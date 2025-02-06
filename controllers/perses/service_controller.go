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

	"github.com/perses/perses-operator/api/v1alpha1"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
	logger "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var slog = logger.WithField("module", "service_controller")

func (r *PersesReconciler) reconcileService(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses := &v1alpha1.Perses{}

	if r, err := r.getLatestPerses(ctx, req, perses); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: perses.Name, Namespace: perses.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {

		ser, err := r.createPersesService(perses)
		if err != nil {
			slog.WithError(err).Error("Failed to define new Service resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Service for the custom resource (%s): (%s)", perses.Name, err)})

			if err := r.Status().Update(ctx, perses); err != nil {
				slog.Error(err, "Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err)
		}

		slog.Infof("Creating a new Service: Service.Namespace %s Service.Name %s", ser.Namespace, ser.Name)
		if err = r.Create(ctx, ser); err != nil {
			slog.WithError(err).Errorf("Failed to create new Service: Service.Namespace %s Service.Name %s", ser.Namespace, ser.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.RequeueWithDelay(time.Minute)
	} else if err != nil {
		slog.WithError(err).Error("Failed to get Service")

		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) createPersesService(
	perses *v1alpha1.Perses) (*corev1.Service, error) {
	ls, err := common.LabelsForPerses(r.Config.PersesImage, perses.Name, perses.Name, perses.Spec.Metadata)

	if err != nil {
		return nil, err
	}

	annotations := map[string]string{}
	if perses.Spec.Metadata != nil && perses.Spec.Metadata.Annotations != nil {
		annotations = perses.Spec.Metadata.Annotations
	}

	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        perses.Name,
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
				TargetPort: intstr.FromInt(8080),
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
