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

package controllers

import (
	"context"
	"fmt"
	"time"

	observabilityv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	common "github.com/perses/perses-operator/internal/perses/common"
	subreconciler "github.com/perses/perses-operator/internal/subreconciler"
	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var cmlog = logger.WithField("module", "configmap_controller")

func (r *PersesReconciler) reconcileConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses := &observabilityv1alpha1.Perses{}

	if r, err := r.getLatestPerses(ctx, req, perses); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	configName := common.GetConfigName(perses.Name)

	found := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configName, Namespace: perses.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {

		cm, err := configMapForPerses(r, perses)
		if err != nil {
			cmlog.Error(err, "Failed to define new ConfigMap resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create ConfigMap for the custom resource (%s): (%s)", perses.Name, err)})

			if err := r.Status().Update(ctx, perses); err != nil {
				cmlog.Error(err, "Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err)
		}

		cmlog.Info("Creating a new ConfigMap",
			"ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		if err = r.Create(ctx, cm); err != nil {
			cmlog.Error(err, "Failed to create new ConfigMap",
				"ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.RequeueWithDelay(time.Minute)
	} else if err != nil {
		cmlog.Error(err, "Failed to get Deployment")
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func configMapForPerses(r *PersesReconciler, perses *observabilityv1alpha1.Perses) (*corev1.ConfigMap, error) {
	configName := common.GetConfigName(perses.Name)
	ls := common.LabelsForPerses(configName, perses.Name)

	persesConfig, err := yaml.Marshal(perses.Spec.Config)

	if err != nil {
		cmlog.Error(err, "Failed to marshal configmap data",
			"ConfigMap.Namespace", perses.Namespace, "ConfigMap.Name", configName)
		return nil, err
	}

	configData := map[string]string{
		"config.yaml": string(persesConfig),
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configName,
			Namespace: perses.Namespace,
			Labels:    ls,
		},
		Data: configData,
	}

	// Set Perses instance as the owner and controller
	if err := ctrl.SetControllerReference(perses, cm, r.Scheme); err != nil {
		return nil, err
	}
	return cm, nil
}
