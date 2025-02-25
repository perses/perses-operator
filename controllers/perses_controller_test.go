package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	persescontroller "github.com/perses/perses-operator/controllers/perses"
	"github.com/perses/perses-operator/internal/perses/common"
	persesconfig "github.com/perses/perses/pkg/model/api/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Perses controller", func() {
	Context("Perses controller test", func() {
		const PersesName = "test-perses"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PersesName,
				Namespace: PersesName,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: PersesName, Namespace: PersesName}
		configMapNamespaceName := types.NamespacedName{Name: common.GetConfigName(PersesName), Namespace: PersesName}
		persesImage := "perses-dev.io/perses:test"
		persesServiceName := "perses-custom-service-name"

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))

			By("Setting the Image ENV VAR which stores the Operand image")
			err = os.Setenv("PERSES_IMAGE", persesImage)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)

			By("Removing the Image ENV VAR which stores the Operand image")
			_ = os.Unsetenv("PERSES_IMAGE")
		})

		It("should successfully reconcile a custom resource for Perses", func() {
			By("Creating the custom resource for the Kind Perses")
			perses := &persesv1alpha1.Perses{}
			err := k8sClient.Get(ctx, typeNamespaceName, perses)
			if err != nil && errors.IsNotFound(err) {
				replicas := int32(2)
				perses := &persesv1alpha1.Perses{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PersesName,
						Namespace: namespace.Name,
					},
					Spec: persesv1alpha1.PersesSpec{
						Metadata: &persesv1alpha1.Metadata{
							Annotations: map[string]string{
								"testing": "true",
							},
							Labels: map[string]string{
								"instance": PersesName,
							},
						},
						Replicas:      &replicas,
						ContainerPort: 8080,
						Image:         persesImage,
						ServiceName:   persesServiceName,
						Config: persesv1alpha1.PersesConfig{
							Config: persesconfig.Config{
								Database: persesconfig.Database{
									File: &persesconfig.File{
										Folder: "/etc/perses/storage",
									},
								},
							},
						},
					},
				}

				err = k8sClient.Create(ctx, perses)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha1.Perses{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			persesReconciler := &persescontroller.PersesReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Errors might arise during reconciliation, but we are checking the final state of the resources
			_, err = persesReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})

			By("Checking if Service was successfully created in the reconciliation")
			Eventually(func() error {
				found := &corev1.Service{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: persesServiceName, Namespace: PersesName}, found)

				if err == nil {
					if len(found.Spec.Ports) < 1 {
						return fmt.Errorf("The number of ports used in the service is not the one defined in the custom resource")
					}
					if found.Spec.Ports[0].Port != 8080 {
						return fmt.Errorf("The port used in the service is not the one defined in the custom resource")
					}
					if found.Spec.Ports[0].TargetPort.IntVal != 8080 {
						return fmt.Errorf("The target port used in the service is not the one defined in the custom resource")
					}
					if found.Spec.Selector["app.kubernetes.io/instance"] != PersesName {
						return fmt.Errorf("The selector used in the service is not the one defined in the custom resource")
					}
					if value, ok := found.ObjectMeta.Annotations["testing"]; ok {
						if value != "true" {
							return fmt.Errorf("The annotation in the service is not the one defined in the custom resource")
						}
					}
					if value, ok := found.ObjectMeta.Labels["instance"]; ok {
						if value != PersesName {
							return fmt.Errorf("The label in the service is not the one defined in the custom resource")
						}
					}
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if ConfigMap was successfully created in the reconciliation")
			Eventually(func() error {
				found := &corev1.ConfigMap{}
				return k8sClient.Get(ctx, configMapNamespaceName, found)
			}, time.Minute*3, time.Second).Should(Succeed())

			By("Checking if StatefulSet was successfully created in the reconciliation")
			Eventually(func() error {
				found := &appsv1.StatefulSet{}
				err = k8sClient.Get(ctx, typeNamespaceName, found)

				if err == nil {
					if len(found.Spec.Template.Spec.Containers) < 1 {
						return fmt.Errorf("The number of containers used in the StatefulSet is not the one expected")
					}
					if found.Spec.Template.Spec.Containers[0].Image != persesImage {
						return fmt.Errorf("The image used in the StatefulSet is not the one expected")
					}
					if len(found.Spec.Template.Spec.Containers[0].Ports) < 1 && found.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != 8080 {
						return fmt.Errorf("The port used in the StatefulSet is not the one defined in the custom resource")
					}
					if len(found.Spec.Template.Spec.Containers[0].Args) < 1 && found.Spec.Template.Spec.Containers[0].Args[0] != "--config=/etc/perses/config/config.yaml" {
						return fmt.Errorf("The config path used in the StatefulSet is not the one defined in the custom resource")
					}
					if value, ok := found.ObjectMeta.Annotations["testing"]; ok {
						if value != "true" {
							return fmt.Errorf("The annotation in the StatefulSet is not the one defined in the custom resource")
						}
					}
					if value, ok := found.ObjectMeta.Labels["instance"]; ok {
						if value != PersesName {
							return fmt.Errorf("The label in the StatefulSet is not the one defined in the custom resource")
						}
					}
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses instance")
			Eventually(func() error {
				if len(perses.Status.Conditions) != 0 {
					latestStatusCondition := perses.Status.Conditions[len(perses.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeAvailablePerses,
						Status: metav1.ConditionTrue, Reason: "Reconciling",
						Message: fmt.Sprintf("StatefulSet for custom resource (%s) created successfully", perses.Name)}
					if latestStatusCondition != expectedLatestStatusCondition {
						return fmt.Errorf("The latest status condition added to the perses instance is not as expected")
					}
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			persesToDelete := &persesv1alpha1.Perses{}
			err = k8sClient.Get(ctx, typeNamespaceName, persesToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Deleting the custom resource")
			err = k8sClient.Delete(ctx, persesToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if StatefulSet was successfully deleted in the reconciliation")
			Eventually(func() error {
				found := &appsv1.StatefulSet{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if Service was successfully deleted in the reconciliation")
			Eventually(func() error {
				found := &corev1.Service{}
				return k8sClient.Get(ctx, types.NamespacedName{Name: persesServiceName, Namespace: PersesName}, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if ConfigMap was successfully deleted in the reconciliation")
			Eventually(func() error {
				found := &corev1.ConfigMap{}
				return k8sClient.Get(ctx, configMapNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses instance")
			Eventually(func() error {
				if len(perses.Status.Conditions) != 0 {
					latestStatusCondition := perses.Status.Conditions[len(perses.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeAvailablePerses,
						Status: metav1.ConditionTrue, Reason: "Finalizing",
						Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", perses.Name)}
					if latestStatusCondition != expectedLatestStatusCondition {
						return fmt.Errorf("The latest status condition added to the perses instance is not as expected")
					}
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
