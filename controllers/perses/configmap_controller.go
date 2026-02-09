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

var cmlog = logger.WithField("module", "configmap_controller")

func (r *PersesReconciler) reconcileConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		cmlog.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	configName := common.GetConfigName(perses.Name)

	found := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: configName, Namespace: perses.Namespace}, found); err != nil {
		if !apierrors.IsNotFound(err) {
			cmlog.WithError(err).Error("Failed to get ConfigMap")
			return subreconciler.RequeueWithError(err)
		}

		cm, err2 := r.createPersesConfigMap(perses)
		if err2 != nil {
			cmlog.WithError(err2).Error("Failed to define new ConfigMap resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create ConfigMap for the custom resource (%s): (%s)", perses.Name, err2)})

			if err = r.Status().Update(ctx, perses); err != nil {
				cmlog.WithError(err).Error("Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err2)
		}

		cmlog.Infof("Creating a new ConfigMap: ConfigMap.Namespace %s ConfigMap.Name %s", cm.Namespace, cm.Name)
		if err = r.Create(ctx, cm); err != nil {
			cmlog.WithError(err).Errorf("Failed to create new ConfigMap: ConfigMap.Namespace %s ConfigMap.Name %s", cm.Namespace, cm.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.ContinueReconciling()
	}

	cm, err := r.createPersesConfigMap(perses)
	if err != nil {
		cmlog.WithError(err).Error("Failed to define new ConfigMap resource for perses")
		return subreconciler.RequeueWithError(err)
	}

	// call update with dry run to fill out fields that are also returned via the k8s api
	if err := r.Update(ctx, cm, client.DryRunAll); err != nil {
		cmlog.WithError(err).Error("Failed to update ConfigMap with dry run")
		return subreconciler.RequeueWithError(err)
	}

	if configMapNeedsUpdate(found, cm, configName, perses) {
		if err := r.Update(ctx, cm); err != nil {
			cmlog.WithError(err).Error("Failed to update ConfigMap")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func configMapNeedsUpdate(existing, updated *corev1.ConfigMap, name string, perses *v1alpha2.Perses) bool {
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

	// check for differences only in the labels that are set by the operator
	labels := common.LabelsForPerses(name, perses)
	for k := range labels {
		if existing.Labels[k] != updated.Labels[k] {
			return true
		}
	}

	return false
}

func (r *PersesReconciler) createPersesConfigMap(perses *v1alpha2.Perses) (*corev1.ConfigMap, error) {
	configName := common.GetConfigName(perses.Name)
	ls := common.LabelsForPerses(configName, perses)

	annotations := map[string]string{}
	if perses.Spec.Metadata != nil && perses.Spec.Metadata.Annotations != nil {
		annotations = perses.Spec.Metadata.Annotations
	}

	persesConfig, err := yaml.Marshal(perses.Spec.Config.Config)

	if err != nil {
		cmlog.WithError(err).Errorf("Failed to marshal configmap data: ConfigMap.Namespace %s ConfigMap.Name %s", perses.Namespace, configName)
		return nil, err
	}

	configData := map[string]string{
		"config.yaml": string(persesConfig),
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        configName,
			Namespace:   perses.Namespace,
			Annotations: annotations,
			Labels:      ls,
		},
		Data: configData,
	}

	// Set Perses instance as the owner and controller
	if err := ctrl.SetControllerReference(perses, cm, r.Scheme); err != nil {
		return nil, err
	}
	return cm, nil
}
