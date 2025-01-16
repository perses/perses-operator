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

	"github.com/perses/perses-operator/api/v1alpha1"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
	logger "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var stlog = logger.WithField("module", "statefulset_controller")

func (r *PersesReconciler) reconcileStatefulSet(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses := &v1alpha1.Perses{}

	if r, err := r.getLatestPerses(ctx, req, perses); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	if perses.Spec.Config.Database.File == nil {
		stlog.Info("Database file configuration is not set, skipping StatefulSet creation")

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
	err := r.Get(ctx, types.NamespacedName{Name: perses.Name, Namespace: perses.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {

		dep, err := r.createPersesStatefulSet(perses)
		if err != nil {
			stlog.WithError(err).Error("Failed to define new StatefulSet resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create StatefulSet for the custom resource (%s): (%s)", perses.Name, err)})

			if err := r.Status().Update(ctx, perses); err != nil {
				stlog.Error(err, "Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err)
		}

		stlog.Infof("Creating a new StatefulSet: StatefulSet.Namespace %s StatefulSet.Name %s", dep.Namespace, dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			stlog.WithError(err).Errorf("Failed to create new StatefulSet: StatefulSet.Namespace %s StatefulSet.Name %s", dep.Namespace, dep.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.RequeueWithDelay(time.Minute)
	} else if err != nil {
		stlog.WithError(err).Error("Failed to get StatefulSet")

		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) createPersesStatefulSet(
	perses *v1alpha1.Perses) (*appsv1.StatefulSet, error) {
	configName := common.GetConfigName(perses.Name)

	ls, err := common.LabelsForPerses(r.Config.PersesImage, perses.Name, perses.Name)
	if err != nil {
		return nil, err
	}

	// Get the Operand image
	image, err := common.ImageForPerses(r.Config.PersesImage)
	if err != nil {
		return nil, err
	}

	storageName := common.GetStorageName(perses.Name)

	dep := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      perses.Name,
			Namespace: perses.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					// TODO(user): Uncomment the following code to configure the nodeAffinity expression
					// according to the platforms which are supported by your solution. It is considered
					// best practice to support multiple architectures. build your manager image using the
					// makefile target docker-buildx. Also, you can use docker manifest inspect <image>
					// to check what are the platforms supported.
					// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity
					//Affinity: &corev1.Affinity{
					//	NodeAffinity: &corev1.NodeAffinity{
					//		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					//			NodeSelectorTerms: []corev1.NodeSelectorTerm{
					//				{
					//					MatchExpressions: []corev1.NodeSelectorRequirement{
					//						{
					//							Key:      "kubernetes.io/arch",
					//							Operator: "In",
					//							Values:   []string{"amd64", "arm64", "ppc64le", "s390x"},
					//						},
					//						{
					//							Key:      "kubernetes.io/os",
					//							Operator: "In",
					//							Values:   []string{"linux"},
					//						},
					//					},
					//				},
					//			},
					//		},
					//	},
					//},
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
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
						VolumeMounts: []corev1.VolumeMount{
							// TODO: check if perses supports passing certificates for TLS
							// {
							// 	Name:      "serving-cert",
							// 	ReadOnly:  true,
							// 	MountPath: "/var/serving-cert",
							// },
							{
								Name:      "config",
								ReadOnly:  true,
								MountPath: "/perses/config",
							},
							{
								Name:      storageName,
								ReadOnly:  false,
								MountPath: "/etc/perses/storage",
							},
						},
						Args: []string{"--config=/perses/config/config.yaml"},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configName,
									},
									DefaultMode: ptr.To[int32](420),
								},
							},
						},
						// {
						// 	Name: "serving-cert",
						// 	VolumeSource: corev1.VolumeSource{
						// 		Secret: &corev1.SecretVolumeSource{
						// 			SecretName:  "perses-serving-cert",
						// 			DefaultMode: &[]int32{420}[0],
						// 		},
						// 	},
						// },
					},
					RestartPolicy: "Always",
					DNSPolicy:     "ClusterFirst",
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: storageName,
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

	// Set the ownerRef for the StatefulSet
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(perses, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}
