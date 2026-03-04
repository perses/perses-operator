// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package perses

import (
	"context"
	"fmt"

	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

var secretlog = logger.WithField("module", "secret_controller")

func (r *PersesReconciler) reconcileConfigSecret(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		secretlog.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	configName := common.GetConfigName(perses.Name)

	found := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: configName, Namespace: perses.Namespace}, found); err != nil {
		if !apierrors.IsNotFound(err) {
			secretlog.WithError(err).Error("Failed to get config Secret")
			return subreconciler.RequeueWithError(err)
		}

		sec, err2 := r.createPersesConfigSecret(perses)
		if err2 != nil {
			secretlog.WithError(err2).Error("Failed to define new config Secret resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create config Secret for the custom resource (%s): (%s)", perses.Name, err2)})

			if err = r.Status().Update(ctx, perses); err != nil {
				secretlog.WithError(err).Error("Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err2)
		}

		secretlog.Infof("Creating a new config Secret: Secret.Namespace %s Secret.Name %s", sec.Namespace, sec.Name)
		if err = r.Create(ctx, sec); err != nil {
			secretlog.WithError(err).Errorf("Failed to create new config Secret: Secret.Namespace %s Secret.Name %s", sec.Namespace, sec.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.ContinueReconciling()
	}

	sec, err := r.createPersesConfigSecret(perses)
	if err != nil {
		secretlog.WithError(err).Error("Failed to define new config Secret resource for perses")
		return subreconciler.RequeueWithError(err)
	}

	if err := r.Update(ctx, sec, client.DryRunAll); err != nil {
		secretlog.WithError(err).Error("Failed to update config Secret with dry run")
		return subreconciler.RequeueWithError(err)
	}

	if configSecretNeedsUpdate(found, sec, configName, perses) {
		if err := r.Update(ctx, sec); err != nil {
			secretlog.WithError(err).Error("Failed to update config Secret")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func configSecretNeedsUpdate(existing, updated *corev1.Secret, name string, perses *v1alpha2.Perses) bool {
	if existing == nil && updated == nil {
		return false
	}
	if existing == nil || updated == nil {
		return true
	}
	if existing.Name != updated.Name || existing.Namespace != updated.Namespace {
		return true
	}
	if !equality.Semantic.DeepEqual(existing.Data, updated.Data) ||
		!equality.Semantic.DeepEqual(existing.Annotations, updated.Annotations) {
		return true
	}

	labels := common.LabelsForPerses(name, perses)
	for k := range labels {
		if existing.Labels[k] != updated.Labels[k] {
			return true
		}
	}

	return false
}

func (r *PersesReconciler) createPersesConfigSecret(perses *v1alpha2.Perses) (*corev1.Secret, error) {
	configName := common.GetConfigName(perses.Name)
	ls := common.LabelsForPerses(configName, perses)

	annotations := map[string]string{}
	if perses.Spec.Metadata != nil && perses.Spec.Metadata.Annotations != nil {
		annotations = perses.Spec.Metadata.Annotations
	}

	persesConfig, err := yaml.Marshal(perses.Spec.Config.Config)
	if err != nil {
		secretlog.WithError(err).Errorf("Failed to marshal config data: Secret.Namespace %s Secret.Name %s", perses.Namespace, configName)
		return nil, err
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        configName,
			Namespace:   perses.Namespace,
			Annotations: annotations,
			Labels:      ls,
		},
		Data: map[string][]byte{
			"config.yaml": persesConfig,
		},
	}

	if err := ctrl.SetControllerReference(perses, sec, r.Scheme); err != nil {
		return nil, err
	}
	return sec, nil
}

// cleanupOldConfigMap deletes any leftover ConfigMap from before the migration
// to Secret-based config storage. This ensures a clean upgrade path.
func (r *PersesReconciler) cleanupOldConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		secretlog.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	configName := common.GetConfigName(perses.Name)
	oldCM := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: configName, Namespace: perses.Namespace}, oldCM); err != nil {
		if apierrors.IsNotFound(err) {
			return subreconciler.ContinueReconciling()
		}
		secretlog.WithError(err).Error("Failed to check for old ConfigMap")
		return subreconciler.RequeueWithError(err)
	}

	secretlog.Infof("Cleaning up old ConfigMap: %s/%s (migrated to Secret)", oldCM.Namespace, oldCM.Name)
	if err := r.Delete(ctx, oldCM); err != nil && !apierrors.IsNotFound(err) {
		secretlog.WithError(err).Error("Failed to delete old ConfigMap")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}
