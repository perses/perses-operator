package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	persesconfig "github.com/perses/perses/pkg/model/api/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persescontroller "github.com/perses/perses-operator/controllers/perses"
	"github.com/perses/perses-operator/internal/perses/common"
)

var _ = Describe("Perses controller", func() {
	Context("Perses controller test", func() {
		const PersesName = "test-perses"

		ctx := context.Background()

		persesImage := "perses-dev.io/perses:test"
		persesServiceName := "perses-custom-service-name"

		BeforeEach(func() {
			By("Setting the Image ENV VAR which stores the Operand image")
			err := os.Setenv("PERSES_IMAGE", persesImage)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("Removing the Image ENV VAR which stores the Operand image")
			_ = os.Unsetenv("PERSES_IMAGE")
		})

		It("should successfully reconcile a custom resource for Perses", func() {
			typeNamespaceName := types.NamespacedName{Name: PersesName, Namespace: persesNamespace}
			configMapNamespaceName := types.NamespacedName{Name: common.GetConfigName(PersesName), Namespace: persesNamespace}

			By("Creating the custom resource for the Kind Perses")
			perses := &persesv1alpha2.Perses{}
			err := k8sClient.Get(ctx, typeNamespaceName, perses)
			if err != nil && errors.IsNotFound(err) {
				replicas := int32(2)
				perses := &persesv1alpha2.Perses{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PersesName,
						Namespace: persesNamespace,
					},
					Spec: persesv1alpha2.PersesSpec{
						Metadata: &persesv1alpha2.Metadata{
							Annotations: map[string]string{
								"testing": "true",
							},
							Labels: map[string]string{
								"instance": PersesName,
							},
						},
						ServiceAccountName: ptr.To("perses-service-account"),
						Replicas:           &replicas,
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
						ContainerPort: ptr.To(int32(8080)),
						Image:         ptr.To(persesImage),
						Service: &persesv1alpha2.PersesService{
							Name: ptr.To(persesServiceName),
							Annotations: map[string]string{
								"custom-annotation": "true",
							},
						},
						Config: persesv1alpha2.PersesConfig{
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
				found := &persesv1alpha2.Perses{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			persesReconciler := &persescontroller.PersesReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Errors might arise during reconciliation, but we are checking the final state of the resources
			//nolint:ineffassign
			_, err = persesReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})

			By("Checking if Service was successfully created in the reconciliation")
			Eventually(func() error {
				found := &corev1.Service{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: persesServiceName, Namespace: persesNamespace}, found)

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
					if value, ok := found.ObjectMeta.Annotations["custom-annotation"]; ok {
						if value != "true" {
							return fmt.Errorf("The custom annotation in the service is not the one defined in the custom resource")
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
					if found.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String() != "1Gi" {
						return fmt.Errorf("The size of the VolumeClaimTemplates is not the one expected")
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
					if found.Spec.Template.Spec.ServiceAccountName != "perses-service-account" {
						return fmt.Errorf("The service account used in the StatefulSet is not the one defined in the custom resource")
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
					if found.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String() != "128Mi" {
						return fmt.Errorf("The resources requests in the StatefulSet is not the one defined in the custom resource")

					}
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the Status Conditions added to the Perses instance")
			Eventually(func() error {
				persesWithStatus := &persesv1alpha2.Perses{}
				err = k8sClient.Get(ctx, typeNamespaceName, persesWithStatus)

				if len(persesWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses instance")
				}

				availableCond := apimeta.FindStatusCondition(persesWithStatus.Status.Conditions, common.TypeAvailablePerses)
				if availableCond == nil {
					return fmt.Errorf("Available condition not found on the perses instance")
				}
				expectedAvailable := metav1.Condition{Type: common.TypeAvailablePerses,
					Status: metav1.ConditionTrue, Reason: "Reconciled",
					Message: fmt.Sprintf("Perses (%s) created successfully", persesWithStatus.Name)}
				if availableCond.Message != expectedAvailable.Message || availableCond.Reason != expectedAvailable.Reason || availableCond.Status != expectedAvailable.Status || availableCond.Type != expectedAvailable.Type {
					return fmt.Errorf("The Available status condition is not as expected. Expected %v but received %v", expectedAvailable, *availableCond)
				}

				degradedCond := apimeta.FindStatusCondition(persesWithStatus.Status.Conditions, common.TypeDegradedPerses)
				if degradedCond == nil {
					return fmt.Errorf("Degraded condition not found on the perses instance")
				}
				expectedDegraded := metav1.Condition{Type: common.TypeDegradedPerses,
					Status: metav1.ConditionFalse, Reason: "Reconciled",
					Message: fmt.Sprintf("Perses (%s) reconciled successfully", persesWithStatus.Name)}
				if degradedCond.Message != expectedDegraded.Message || degradedCond.Reason != expectedDegraded.Reason || degradedCond.Status != expectedDegraded.Status || degradedCond.Type != expectedDegraded.Type {
					return fmt.Errorf("The Degraded status condition is not as expected. Expected %v but received %v", expectedDegraded, *degradedCond)
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			persesToDelete := &persesv1alpha2.Perses{}
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
				return k8sClient.Get(ctx, types.NamespacedName{Name: persesServiceName, Namespace: persesNamespace}, found)
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

		It("should successfully reconcile a custom resource for Perses with storage", func() {
			PersesStorageName := "perses-test-with-storage"
			typeNamespaceName := types.NamespacedName{Name: PersesStorageName, Namespace: persesNamespace}
			configMapNamespaceName := types.NamespacedName{Name: common.GetConfigName(PersesStorageName), Namespace: persesNamespace}

			By("Creating the custom resource for the Kind Perses with storage")
			perses := &persesv1alpha2.Perses{}
			err := k8sClient.Get(ctx, typeNamespaceName, perses)
			if err != nil && errors.IsNotFound(err) {
				perses := &persesv1alpha2.Perses{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PersesStorageName,
						Namespace: persesNamespace,
					},
					Spec: persesv1alpha2.PersesSpec{
						Metadata: &persesv1alpha2.Metadata{
							Labels: map[string]string{
								"instance": PersesStorageName,
							},
						},
						Image: ptr.To(persesImage),
						Config: persesv1alpha2.PersesConfig{
							Config: persesconfig.Config{
								Database: persesconfig.Database{
									File: &persesconfig.File{
										Folder: "/etc/perses/storage",
									},
								},
							},
						},
						Storage: &persesv1alpha2.StorageConfiguration{
							PersistentVolumeClaimTemplate: &corev1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("10Gi"),
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
				found := &persesv1alpha2.Perses{}
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

			By("Checking if StatefulSet was successfully created in the reconciliation")
			Eventually(func() error {
				found := &appsv1.StatefulSet{}
				err = k8sClient.Get(ctx, typeNamespaceName, found)

				if err == nil {
					if len(found.Spec.Template.Spec.Containers) != 1 {
						return fmt.Errorf("The number of containers used in the StatefulSet is not the one expected")
					}
					if found.Spec.Template.Spec.Containers[0].Image != persesImage {
						return fmt.Errorf("The image used in the StatefulSet is not the one expected")
					}
					if found.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String() != "10Gi" {
						return fmt.Errorf("The requested size of the VolumeClaimTemplates is not the one expected")
					}
					if value, ok := found.ObjectMeta.Labels["instance"]; ok {
						if value != PersesStorageName {
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

			persesToDelete := &persesv1alpha2.Perses{}
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
				return k8sClient.Get(ctx, typeNamespaceName, found)
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

		It("should successfully reconcile a custom resource for Perses with provisioning secrets", func() {
			PersesProvisioningName := "perses-test-with-provisioning"
			typeNamespaceName := types.NamespacedName{Name: PersesProvisioningName, Namespace: persesNamespace}
			secretName := "encrypted-key-secret"
			secretNamespaceName := types.NamespacedName{Name: secretName, Namespace: persesNamespace}
			var err error

			By("Creating the provisioning secret")
			secretKey := "encrypted-key"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: persesNamespace,
				},
				StringData: map[string]string{
					secretKey: "verysecret",
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("Creating the custom resource for the Kind Perses with provisioning")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PersesProvisioningName,
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					Image: ptr.To(persesImage),
					Config: persesv1alpha2.PersesConfig{
						Config: persesconfig.Config{
							Database: persesconfig.Database{
								File: &persesconfig.File{
									Folder: "/etc/perses/storage",
								},
							},
							Security: persesconfig.Security{
								EncryptionKeyFile: filepath.Join("/etc/perses/provisioning/secrets", secret.String()),
							},
						},
					},
					Provisioning: &persesv1alpha2.Provisioning{
						SecretRefs: []*persesv1alpha2.ProvisioningSecret{
							{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretName,
									},
									Key: secretKey,
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, perses)).To(Succeed())

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.Perses{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			persesReconciler := &persescontroller.PersesReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling the custom resource created")
			Eventually(func() error {
				_, err := persesReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespaceName,
				})
				return err
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			var initialHash string
			var initialResourceVersion string
			By("Checking if StatefulSet has the correct provisioning annotation and status is updated")
			Eventually(func() (string, error) {
				persesResource := &persesv1alpha2.Perses{}
				if err := k8sClient.Get(ctx, typeNamespaceName, persesResource); err != nil {
					return "", err
				}
				if len(persesResource.Status.Provisioning) != 1 {
					return "", fmt.Errorf("provisioning status not updated")
				}
				initialResourceVersion = persesResource.Status.Provisioning[0].Version

				sts := &appsv1.StatefulSet{}
				if err := k8sClient.Get(ctx, typeNamespaceName, sts); err != nil {
					return "", err
				}
				hash, ok := sts.Spec.Template.Annotations[common.PersesProvisioningVersion]
				if !ok {
					return "", fmt.Errorf("provisioning annotation not found")
				}
				return hash, nil
			}, time.Minute, time.Second).ShouldNot(BeEmpty(), initialHash)

			By("Updating the provisioning secret")
			updatedSecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, secretNamespaceName, updatedSecret)).To(Succeed())
			if updatedSecret.StringData == nil {
				updatedSecret.StringData = make(map[string]string)
			}
			updatedSecret.StringData[secretKey] = "newSecret"
			Expect(k8sClient.Update(ctx, updatedSecret)).To(Succeed())

			By("Reconciling the custom resource again")
			_, err = persesReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).NotTo(HaveOccurred())
			_, err = persesReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if StatefulSet has the updated provisioning annotation and status is updated")
			Eventually(func() (string, error) {
				persesResource := &persesv1alpha2.Perses{}
				if err := k8sClient.Get(ctx, typeNamespaceName, persesResource); err != nil {
					return "", err
				}
				if len(persesResource.Status.Provisioning) != 1 || persesResource.Status.Provisioning[0].Version == initialResourceVersion {
					return "", fmt.Errorf("provisioning status not updated")
				}

				sts := &appsv1.StatefulSet{}
				if err := k8sClient.Get(ctx, typeNamespaceName, sts); err != nil {
					return "", err
				}
				return sts.Spec.Template.Annotations["perses.dev/provisioning-version"], nil
			}, time.Minute, time.Second).Should(Not(Equal(initialHash)))

			By("Deleting the custom resource and secret")
			Expect(k8sClient.Delete(ctx, perses)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		})

		It("should successfully delete Perses and remove the finalizer", func() {
			PersesDeleteName := "perses-delete-test"
			typeNamespaceName := types.NamespacedName{Name: PersesDeleteName, Namespace: persesNamespace}

			By("Creating the custom resource for the Kind Perses")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PersesDeleteName,
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					Image: ptr.To(persesImage),
					Config: persesv1alpha2.PersesConfig{
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
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.Perses{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			persesReconciler := &persescontroller.PersesReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling to add the finalizer")
			Eventually(func() error {
				_, err := persesReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespaceName,
				})
				return err
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking if the finalizer was added")
			Eventually(func() bool {
				found := &persesv1alpha2.Perses{}
				err := k8sClient.Get(ctx, typeNamespaceName, found)
				if err != nil {
					return false
				}
				return slices.Contains(found.Finalizers, common.PersesFinalizer)
			}, time.Minute, time.Second).Should(BeTrue())

			By("Deleting the custom resource")
			persesToDelete := &persesv1alpha2.Perses{}
			err = k8sClient.Get(ctx, typeNamespaceName, persesToDelete)
			Expect(err).To(Not(HaveOccurred()))
			err = k8sClient.Delete(ctx, persesToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Reconciling after deletion to trigger finalizer removal")
			Eventually(func() error {
				_, err := persesReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespaceName,
				})
				return err
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking if the Perses resource was fully deleted (finalizer removed)")
			Eventually(func() bool {
				found := &persesv1alpha2.Perses{}
				err := k8sClient.Get(ctx, typeNamespaceName, found)
				return errors.IsNotFound(err)
			}, time.Minute, time.Second).Should(BeTrue(), "Perses resource should be deleted after finalizer removal")
		})

		It("should include user-defined volumes and volumeMounts in the workload", func() {
			const PersesVolumesName = "test-perses-volumes"
			typeNamespaceName := types.NamespacedName{Name: PersesVolumesName, Namespace: persesNamespace}

			By("Creating a Perses CR with user-defined volumes and volumeMounts")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PersesVolumesName,
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					Image: ptr.To(persesImage),
					Config: persesv1alpha2.PersesConfig{
						Config: persesconfig.Config{
							Database: persesconfig.Database{
								File: &persesconfig.File{
									Folder: "/etc/perses/storage",
								},
							},
						},
					},
					Storage: &persesv1alpha2.StorageConfiguration{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
					Volumes: []corev1.Volume{
						{
							Name: "extra-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "extra-config",
							MountPath: "/etc/perses/extra",
							ReadOnly:  true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, perses)).To(Succeed())

			persesReconciler := &persescontroller.PersesReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling the custom resource")
			Eventually(func() error {
				_, err := persesReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespaceName,
				})
				return err
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking that the Deployment has the user-defined volume and volumeMount")
			Eventually(func() error {
				deployment := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, typeNamespaceName, deployment); err != nil {
					return err
				}
				volumes := deployment.Spec.Template.Spec.Volumes
				if !slices.ContainsFunc(volumes, func(v corev1.Volume) bool {
					return v.Name == "extra-config" && v.VolumeSource.EmptyDir != nil
				}) {
					return fmt.Errorf("user-defined volume 'extra-config' not found in deployment")
				}
				mounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
				if !slices.ContainsFunc(mounts, func(m corev1.VolumeMount) bool {
					return m.Name == "extra-config" && m.MountPath == "/etc/perses/extra"
				}) {
					return fmt.Errorf("user-defined volumeMount 'extra-config' not found in deployment")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("Deleting the custom resource")
			persesToDelete := &persesv1alpha2.Perses{}
			Expect(k8sClient.Get(ctx, typeNamespaceName, persesToDelete)).To(Succeed())
			Expect(k8sClient.Delete(ctx, persesToDelete)).To(Succeed())
		})

		It("should reject a Perses CR with a volume using the provisioning- prefix", func() {
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses-provisioning-prefix",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					Image: ptr.To(persesImage),
					Volumes: []corev1.Volume{
						{
							Name: "provisioning-my-secret",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			}
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'provisioning-' prefix"))
		})

		DescribeTable("should reject a Perses CR with any reserved volume name",
			func(name string) {
				perses := &persesv1alpha2.Perses{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("test-reserved-vol-%s", name),
						Namespace: persesNamespace,
					},
					Spec: persesv1alpha2.PersesSpec{
						Image: ptr.To(persesImage),
						Volumes: []corev1.Volume{
							{
								Name: name,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				}
				err := k8sClient.Create(ctx, perses)
				Expect(err).To(HaveOccurred())
			},
			Entry("config", "config"),
			Entry("plugins", "plugins"),
			Entry("storage", "storage"),
			Entry("ca", "ca"),
			Entry("tls", "tls"),
		)

		DescribeTable("should reject a Perses CR with any reserved or shadowed volumeMount path",
			func(path string) {
				volName := fmt.Sprintf("vol-%s", path[1:3])
				perses := &persesv1alpha2.Perses{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("test-reserved-path-%s", volName),
						Namespace: persesNamespace,
					},
					Spec: persesv1alpha2.PersesSpec{
						Image: ptr.To(persesImage),
						Volumes: []corev1.Volume{
							{
								Name: volName,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volName,
								MountPath: path,
							},
						},
					},
				}
				err := k8sClient.Create(ctx, perses)
				Expect(err).To(HaveOccurred())
			},
			Entry("/etc/perses (root)", "/etc/perses"),
			Entry("/etc/perses/config", "/etc/perses/config"),
			Entry("/etc/perses/config/subdir", "/etc/perses/config/subdir"),
			Entry("/etc/perses/plugins", "/etc/perses/plugins"),
			Entry("/etc/perses/plugins/custom", "/etc/perses/plugins/custom"),
			Entry("/etc/perses/provisioning", "/etc/perses/provisioning"),
			Entry("/etc/perses/provisioning/secrets", "/etc/perses/provisioning/secrets"),
			Entry("/perses (storage)", "/perses"),
			Entry("/ca", "/ca"),
			Entry("/tls", "/tls"),
		)

		DescribeTable("should allow a Perses CR with a non-reserved volumeMount path",
			func(path string) {
				name := fmt.Sprintf("test-allowed-path-%s", strings.ReplaceAll(path[1:], "/", "-"))
				perses := &persesv1alpha2.Perses{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: persesNamespace,
					},
					Spec: persesv1alpha2.PersesSpec{
						Image: ptr.To(persesImage),
						Volumes: []corev1.Volume{
							{
								Name: "user-vol",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "user-vol",
								MountPath: path,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, perses)).To(Succeed())

				persesToDelete := &persesv1alpha2.Perses{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: persesNamespace}, persesToDelete)).To(Succeed())
				Expect(k8sClient.Delete(ctx, persesToDelete)).To(Succeed())
			},
			Entry("/etc/perses/extra", "/etc/perses/extra"),
			Entry("/data/custom", "/data/custom"),
		)

		It("should reject a Perses CR with duplicate volume names", func() {
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses-dup-vol",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					Image: ptr.To(persesImage),
					Volumes: []corev1.Volume{
						{
							Name: "my-vol",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "my-vol",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			}
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
		})

		It("should reject a Perses CR with duplicate volumeMount mountPaths", func() {
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses-dup-mount",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					Image: ptr.To(persesImage),
					Volumes: []corev1.Volume{
						{
							Name: "vol-a",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "vol-b",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "vol-a",
							MountPath: "/data/shared",
						},
						{
							Name:      "vol-b",
							MountPath: "/data/shared",
						},
					},
				},
			}
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
		})

		It("should set Degraded status when a volumeMount has no corresponding volume", func() {
			const orphanName = "test-perses-orphan-mount"
			typeNamespaceName := types.NamespacedName{Name: orphanName, Namespace: persesNamespace}

			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      orphanName,
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					Image: ptr.To(persesImage),
					Volumes: []corev1.Volume{
						{
							Name: "existing-vol",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "nonexistent-vol",
							MountPath: "/data/orphan",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, perses)).To(Succeed())

			persesReconciler := &persescontroller.PersesReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling until validateVolumes catches the orphan volumeMount")
			Eventually(func() string {
				_, err := persesReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespaceName,
				})
				if err != nil {
					return err.Error()
				}
				return ""
			}, time.Second*10, time.Millisecond*250).Should(ContainSubstring("not defined in spec.volumes"))

			By("Checking the Degraded status condition")
			Eventually(func() string {
				found := &persesv1alpha2.Perses{}
				if err := k8sClient.Get(ctx, typeNamespaceName, found); err != nil {
					return ""
				}
				cond := apimeta.FindStatusCondition(found.Status.Conditions, common.TypeDegradedPerses)
				if cond == nil {
					return ""
				}
				return cond.Message
			}, time.Minute, time.Second).Should(ContainSubstring("not defined in spec.volumes"))

			By("Deleting the custom resource")
			persesToDelete := &persesv1alpha2.Perses{}
			Expect(k8sClient.Get(ctx, typeNamespaceName, persesToDelete)).To(Succeed())
			Expect(k8sClient.Delete(ctx, persesToDelete)).To(Succeed())
		})

		It("should set Degraded status when volumeMounts exist but no volumes defined", func() {
			const noVolName = "test-perses-mount-no-vol"
			typeNamespaceName := types.NamespacedName{Name: noVolName, Namespace: persesNamespace}

			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      noVolName,
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					Image: ptr.To(persesImage),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "my-vol",
							MountPath: "/data/test",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, perses)).To(Succeed())

			persesReconciler := &persescontroller.PersesReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling until validateVolumes catches the missing volumes")
			Eventually(func() string {
				_, err := persesReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespaceName,
				})
				if err != nil {
					return err.Error()
				}
				return ""
			}, time.Second*10, time.Millisecond*250).Should(ContainSubstring("not defined in spec.volumes"))

			By("Deleting the custom resource")
			persesToDelete := &persesv1alpha2.Perses{}
			Expect(k8sClient.Get(ctx, typeNamespaceName, persesToDelete)).To(Succeed())
			Expect(k8sClient.Delete(ctx, persesToDelete)).To(Succeed())
		})
	})
})
