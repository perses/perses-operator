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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

var dlog = logger.WithField("module", "deployment_controller")

func (r *PersesReconciler) reconcileDeployment(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		dlog.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	if perses.Spec.Config.Database.SQL == nil {
		dlog.Debug("Database SQL configuration is not set, skipping Deployment creation")

		found := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Name: perses.Name, Namespace: perses.Namespace}, found)
		if err == nil {
			dlog.Info("Deleting Deployment since database configuration changed")
			if err := r.Delete(ctx, found); err != nil {
				dlog.WithError(err).Error("Failed to delete Deployment")
				return subreconciler.RequeueWithError(err)
			}
		}

		return subreconciler.ContinueReconciling()
	}

	found := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: perses.Name, Namespace: perses.Namespace}, found); err != nil {
		if !apierrors.IsNotFound(err) {
			dlog.WithError(err).Error("Failed to get Deployment")
			return subreconciler.RequeueWithError(err)
		}

		dep, err := r.createPersesDeployment(perses)
		if err != nil {
			dlog.WithError(err).Error("Failed to define new Deployment resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", perses.Name, err)})

			if err := r.Status().Update(ctx, perses); err != nil {
				dlog.WithError(err).Error("Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err)
		}

		dlog.Infof("Creating a new Deployment: Deployment.Namespace %s Deployment.Name %s", dep.Namespace, dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			dlog.WithError(err).Errorf("Failed to create new Deployment: Deployment.Namespace %s Deployment.Name %s", dep.Namespace, dep.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.ContinueReconciling()
	}

	dep, err := r.createPersesDeployment(perses)
	if err != nil {
		dlog.WithError(err).Error("Failed to define new Deployment resource for perses")
		return subreconciler.RequeueWithError(err)
	}

	// call update with dry run to fill out fields that are also returned via the k8s api
	if err = r.Update(ctx, dep, client.DryRunAll); err != nil {
		dlog.WithError(err).Error("Failed to update Deployment with dry run")
		return subreconciler.RequeueWithError(err)
	}

	if !equality.Semantic.DeepEqual(found.Spec, dep.Spec) {
		if err = r.Update(ctx, dep); err != nil {
			dlog.WithError(err).Error("Failed to update Deployment")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) createPersesDeployment(
	perses *v1alpha2.Perses) (*appsv1.Deployment, error) {

	ls := common.LabelsForPerses(perses.Name, perses)

	annotations := map[string]string{}
	if perses.Spec.Metadata != nil && perses.Spec.Metadata.Annotations != nil {
		annotations = perses.Spec.Metadata.Annotations
	}

	// Get the Operand image
	image, err := common.ImageForPerses(perses, r.Config.PersesImage)
	if err != nil {
		return nil, err
	}

	livenessProbe, readinessProbe := common.GetProbes(perses)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        perses.Name,
			Namespace:   perses.Namespace,
			Annotations: annotations,
			Labels:      ls,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Replicas: perses.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      ls,
				},
				Spec: corev1.PodSpec{
					NodeSelector:    perses.Spec.NodeSelector,
					Tolerations:     perses.Spec.Tolerations,
					Affinity:        perses.Spec.Affinity,
					SecurityContext: common.GetPodSecurityContext(perses),
					Containers: []corev1.Container{{
						Image:           image,
						Name:            "perses",
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: perses.Spec.ContainerPort,
							Name:          "perses",
						}},
						VolumeMounts:   common.GetVolumeMounts(perses),
						Args:           common.GetPersesArgs(perses),
						LivenessProbe:  livenessProbe,
						ReadinessProbe: readinessProbe,
					}},
					Volumes:       common.GetVolumes(perses),
					RestartPolicy: "Always",
					DNSPolicy:     "ClusterFirst",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Type(1),
						StrVal: "25%",
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Type(1),
						StrVal: "25%",
					},
				},
			},
		},
	}

	if perses.Spec.ServiceAccountName != "" {
		dep.Spec.Template.Spec.ServiceAccountName = perses.Spec.ServiceAccountName
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(perses, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}
