package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/perses/perses/pkg/client/perseshttp"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	persescommon "github.com/perses/perses/pkg/model/api/v1/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	globaldatasourcecontroller "github.com/perses/perses-operator/controllers/globaldatasources"
	internal "github.com/perses/perses-operator/internal/perses"
	"github.com/perses/perses-operator/internal/perses/common"
)

var _ = Describe("GlobalDatasource controller", func() {
	Context("GlobalDatasource controller test", func() {
		const PersesName = "perses-for-globaldatasource"
		const PersesNamespace = "perses-globaldatasource-test"
		const GlobalDatasourceName = "my-custom-globaldatasource"
		const PersesSecretName = GlobalDatasourceName + "-secret"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PersesNamespace,
				Namespace: PersesNamespace,
			},
		}

		persesNamespaceName := types.NamespacedName{Name: PersesName, Namespace: PersesNamespace}
		globaldatasourceName := types.NamespacedName{Name: GlobalDatasourceName}

		persesImage := "perses-dev.io/perses:test"

		newGlobalDatasource := &persesv1.GlobalDatasource{
			Kind: persesv1.KindGlobalDatasource,
			Metadata: persesv1.Metadata{
				Name: GlobalDatasourceName,
			},
			Spec: persesv1.DatasourceSpec{
				Display: &persescommon.Display{
					Name: GlobalDatasourceName,
				},
				Default: true,
				Plugin: persescommon.Plugin{
					Kind: "Prometheus",
					Spec: map[string]any{},
				},
			},
		}

		newSecret := &persesv1.GlobalSecret{
			Kind: persesv1.KindGlobalSecret,
			Metadata: persesv1.Metadata{
				Name: PersesSecretName,
			},
			Spec: persesv1.SecretSpec{},
		}

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

		It("should successfully reconcile a custom resource globaldatasource for Perses", func() {
			By("Creating the custom resource for the Kind Perses")
			perses := &persesv1alpha2.Perses{}
			err := k8sClient.Get(ctx, persesNamespaceName, perses)
			if err != nil && errors.IsNotFound(err) {
				perses := &persesv1alpha2.Perses{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PersesName,
						Namespace: PersesNamespace,
					},
					Spec: persesv1alpha2.PersesSpec{
						ContainerPort: 8080,
					},
				}

				err = k8sClient.Create(ctx, perses)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Creating the custom resource for the Kind PersesGlobalDatasource")
			globaldatasource := &persesv1alpha2.PersesGlobalDatasource{}
			err = k8sClient.Get(ctx, globaldatasourceName, globaldatasource)
			if err != nil && errors.IsNotFound(err) {
				perses := &persesv1alpha2.PersesGlobalDatasource{
					ObjectMeta: metav1.ObjectMeta{
						Name: GlobalDatasourceName,
					},
					Spec: persesv1alpha2.DatasourceSpec{
						Config: persesv1alpha2.Datasource{
							DatasourceSpec: newGlobalDatasource.Spec,
						},
					},
				}

				err = k8sClient.Create(ctx, perses)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.PersesGlobalDatasource{}
				return k8sClient.Get(ctx, globaldatasourceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			// Mock the Perses API to assert that Is creating a new globaldatasource when reconciling
			mockPersesClient := new(internal.MockClient)
			mockGlobalDatasource := new(internal.MockGlobalDatasource)
			mockGlobalSecret := new(internal.MockGlobalSecret)

			mockPersesClient.On("GlobalDatasource").Return(mockGlobalDatasource)
			mockPersesClient.On("GlobalSecret").Return(mockGlobalSecret)
			getGlobalDatasource := mockGlobalDatasource.On("Get", GlobalDatasourceName).Return(&persesv1.GlobalDatasource{}, perseshttp.RequestNotFoundError)
			mockGlobalDatasource.On("Create", newGlobalDatasource).Return(&persesv1.GlobalDatasource{}, nil)
			mockGlobalSecret.On("Create", newSecret).Return(&persesv1.GlobalSecret{}, nil)

			By("Reconciling the custom resource created")
			globaldatasourceReconciler := &globaldatasourcecontroller.PersesGlobalDatasourceReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				ClientFactory: common.NewWithClient(mockPersesClient),
			}

			_, err = globaldatasourceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: globaldatasourceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			// The globaldatasource was created in the Perses API
			getGlobalDatasource.Unset()
			mockGlobalDatasource.On("Get", GlobalDatasourceName).Return(&persesv1.GlobalDatasource{}, nil)

			By("Checking if the Perses API was called to create a globaldatasource")
			Eventually(func() error {
				if !mockGlobalDatasource.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a globaldatasource")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses globaldatasource instance")
			Eventually(func() error {
				globaldatasourceWithStatus := &persesv1alpha2.PersesGlobalDatasource{}
				err = k8sClient.Get(ctx, globaldatasourceName, globaldatasourceWithStatus)

				if len(globaldatasourceWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses globaldatasource instance")
				} else {
					latestStatusCondition := globaldatasourceWithStatus.Status.Conditions[len(globaldatasourceWithStatus.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeAvailablePerses,
						Status: metav1.ConditionTrue, Reason: "Reconciling",
						Message: fmt.Sprintf("GlobalDatasource (%s) created successfully", globaldatasourceWithStatus.Name)}
					if latestStatusCondition.Message != expectedLatestStatusCondition.Message && latestStatusCondition.Reason != expectedLatestStatusCondition.Reason && latestStatusCondition.Status != expectedLatestStatusCondition.Status && latestStatusCondition.Type != expectedLatestStatusCondition.Type {
						return fmt.Errorf("The latest status condition added to the perses globaldatasource instance is not as expected, got: %v", expectedLatestStatusCondition)
					}
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			mockGlobalDatasource.On("Delete", GlobalDatasourceName).Return(nil)
			mockGlobalSecret.On("Delete", PersesSecretName).Return(nil)

			globaldatasourceToDelete := &persesv1alpha2.PersesGlobalDatasource{}
			err = k8sClient.Get(ctx, globaldatasourceName, globaldatasourceToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Deleting the custom resource")
			err = k8sClient.Delete(ctx, globaldatasourceToDelete)
			Expect(err).To(Not(HaveOccurred()))

			_, err = globaldatasourceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: globaldatasourceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the Perses API was called to delete a globaldatasource")
			Eventually(func() error {
				if !mockGlobalDatasource.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a globaldatasource")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
