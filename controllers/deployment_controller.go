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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var dlog = logger.WithField("module", "deployment_controller")

func (r *PersesReconciler) reconcileDeployment(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	perses := &observabilityv1alpha1.Perses{}

	if r, err := r.getLatestPerses(ctx, req, perses); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: perses.Name, Namespace: perses.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {

		dep, err := r.deploymentForPerses(perses)
		if err != nil {
			dlog.WithError(err).Error("Failed to define new Deployment resource for perses")

			meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", perses.Name, err)})

			if err := r.Status().Update(ctx, perses); err != nil {
				dlog.Error(err, "Failed to update perses status")
				return subreconciler.RequeueWithError(err)
			}

			return subreconciler.RequeueWithError(err)
		}

		dlog.Infof("Creating a new Deployment: Deployment.Namespace %s Deployment.Name %s", dep.Namespace, dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			dlog.WithError(err).Errorf("Failed to create new Deployment: Deployment.Namespace %s Deployment.Name %s", dep.Namespace, dep.Name)
			return subreconciler.RequeueWithError(err)
		}

		return subreconciler.RequeueWithDelay(time.Minute)
	} else if err != nil {
		dlog.WithError(err).Error("Failed to get Deployment")

		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesReconciler) deploymentForPerses(
	perses *observabilityv1alpha1.Perses) (*appsv1.Deployment, error) {
	configName := common.GetConfigName(perses.Name)

	ls := common.LabelsForPerses(perses.Name, perses.Name)

	// Get the Operand image
	image, err := common.ImageForPerses()
	if err != nil {
		return nil, err
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      perses.Name,
			Namespace: perses.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
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
						RunAsNonRoot: &[]bool{true}[0],
						RunAsUser:    &[]int64{65534}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Image:           image,
						Name:            "perses",
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &[]bool{true}[0],
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
									DefaultMode: &[]int32{420}[0],
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

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(perses, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}
