/*
Copyright 2025 The Perses Authors.

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

	logger "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

var stlog = logger.WithField("module", "statefulset_controller")

func (r *PersesReconciler) reconcileStatefulSet(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses, ok := persesFromContext(ctx)
	if !ok {
		stlog.Error("perses not found in context")
		return subreconciler.RequeueWithError(fmt.Errorf("perses not found in context"))
	}

	if perses.Spec.Config.Database.File == nil {
		stlog.Debug("Database file configuration is not set, skipping StatefulSet creation")

		found := &appsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{Name: perses.Name, Namespace: perses.Namespace}, found)
		if err == nil {
			stlog.Info("Deleting StatefulSet since database configuration changed")
			if err := r.Delete(ctx, found); err != nil {
				stlog.WithError(err).Error("Failed to delete StatefulSet")
				return subreconciler.RequeueWithError(err)
			}
		}

		return subreconciler.ContinueReconciling()
	}

	found := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: perses.Name, Namespace: perses.Namespace}, found); err != nil {
		if !apierrors.IsNotFound(err) {
			stlog.WithError(err).Error("Failed to get StatefulSet")

			return subreconciler.RequeueWithError(err)
		}

		sts, err := r.createPersesStatefulSet(perses)
		if err != nil {
			stlog.WithError(err).Error("Failed to define new StatefulSet resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create StatefulSet for the custom resource (%s): (%s)", perses.Name, err)})

			if err := r.Status().Update(ctx, perses); err != nil {
				stlog.WithError(err).Error("Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err)
		}

		stlog.Infof("Creating a new StatefulSet: StatefulSet.Namespace %s StatefulSet.Name %s", sts.Namespace, sts.Name)
		if err = r.Create(ctx, sts); err != nil {
			stlog.WithError(err).Errorf("Failed to create new StatefulSet: StatefulSet.Namespace %s StatefulSet.Name %s", sts.Namespace, sts.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.RequeueWithDelay(time.Minute)
	}

	sts, err := r.createPersesStatefulSet(perses)
	if err != nil {
		stlog.WithError(err).Error("Failed to define new StatefulSet resource for perses")
		return subreconciler.RequeueWithError(err)
	}

	// call update with dry run to fill out fields that are also returned via the k8s api
	if err = r.Update(ctx, sts, client.DryRunAll); err != nil {
		stlog.WithError(err).Error("Failed to update StatefulSet with dry run")
		return subreconciler.RequeueWithError(err)
	}

	if !equality.Semantic.DeepEqual(found.Spec, sts.Spec) {
		if err = r.Update(ctx, sts); err != nil {
			stlog.WithError(err).Error("Failed to update StatefulSet")
			return subreconciler.RequeueWithError(err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) createPersesStatefulSet(
	perses *v1alpha2.Perses) (*appsv1.StatefulSet, error) {

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

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        perses.Name,
			Namespace:   perses.Namespace,
			Annotations: annotations,
			Labels:      ls,
		},
		Spec: appsv1.StatefulSetSpec{
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
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: common.StorageVolumeName,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	}

	if perses.Spec.Storage != nil {
		if perses.Spec.Storage.StorageClass != nil && len(*perses.Spec.Storage.StorageClass) > 0 {
			sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName = perses.Spec.Storage.StorageClass
		}

		if !perses.Spec.Storage.Size.IsZero() {
			sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests = corev1.ResourceList{
				corev1.ResourceStorage: perses.Spec.Storage.Size,
			}
		}
	}

	if perses.Spec.ServiceAccountName != "" {
		sts.Spec.Template.Spec.ServiceAccountName = perses.Spec.ServiceAccountName
	}

	// Set the ownerRef for the StatefulSet
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(perses, sts, r.Scheme); err != nil {
		return nil, err
	}
	return sts, nil
}
