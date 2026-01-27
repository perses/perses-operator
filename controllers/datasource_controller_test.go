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
	datasourcecontroller "github.com/perses/perses-operator/controllers/datasources"
	internal "github.com/perses/perses-operator/internal/perses"
	"github.com/perses/perses-operator/internal/perses/common"
)

var _ = Describe("Datasource controller", Ordered, func() {
	Context("Datasource controller test", func() {
		const PersesName = "perses-for-datasource"
		const PersesNamespace = "perses-datasource-test"
		const DatasourceName = "my-custom-datasource"
		const PersesSecretName = DatasourceName + "-secret"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PersesNamespace,
				Namespace: PersesNamespace,
			},
		}

		persesNamespaceName := types.NamespacedName{Name: PersesName, Namespace: PersesNamespace}
		datasourceNamespaceName := types.NamespacedName{Name: DatasourceName, Namespace: PersesNamespace}

		persesImage := "perses-dev.io/perses:test"

		var newSecret *persesv1.Secret
		var newDatasource *persesv1.Datasource

		BeforeAll(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))

			By("Setting the Image ENV VAR which stores the Operand image")
			err = os.Setenv("PERSES_IMAGE", persesImage)
			Expect(err).To(Not(HaveOccurred()))

			By("Creating the custom resource for the Kind Perses")
			perses := &persesv1alpha2.Perses{}
			err = k8sClient.Get(ctx, persesNamespaceName, perses)
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

			newSecret = &persesv1.Secret{
				Kind: persesv1.KindSecret,
				Metadata: persesv1.ProjectMetadata{
					Metadata: persesv1.Metadata{
						Name: PersesSecretName,
					},
				},
				Spec: persesv1.SecretSpec{},
			}
			newDatasource = &persesv1.Datasource{
				Kind: persesv1.KindDatasource,
				Metadata: persesv1.ProjectMetadata{
					Metadata: persesv1.Metadata{
						Name: DatasourceName,
					},
				},
				Spec: persesv1.DatasourceSpec{
					Display: &persescommon.Display{
						Name: DatasourceName,
					},
					Default: true,
					Plugin: persescommon.Plugin{
						Kind: "Prometheus",
						Spec: map[string]any{},
					},
				},
			}
		})

		AfterAll(func() {
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)

			By("Removing the Image ENV VAR which stores the Operand image")
			_ = os.Unsetenv("PERSES_IMAGE")
		})

		It("should successfully reconcile a custom resource datasource for Perses", func() {
			By("Creating the custom resource for the Kind PersesDatasource")

			datasource := &persesv1alpha2.PersesDatasource{}
			err := k8sClient.Get(ctx, datasourceNamespaceName, datasource)
			if err != nil && errors.IsNotFound(err) {
				datasource = &persesv1alpha2.PersesDatasource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DatasourceName,
						Namespace: PersesNamespace,
					},
					Spec: persesv1alpha2.DatasourceSpec{
						Config: persesv1alpha2.Datasource{
							DatasourceSpec: newDatasource.Spec,
						},
					},
				}

				err = k8sClient.Create(ctx, datasource)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.PersesDatasource{}
				return k8sClient.Get(ctx, datasourceNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			// Mock the Perses API to assert that Is creating a new datasource when reconciling
			mockPersesClient := new(internal.MockClient)
			mockDatasource := new(internal.MockDatasource)
			mockSecret := new(internal.MockSecret)

			mockPersesClient.On("Datasource", PersesNamespace).Return(mockDatasource)
			mockPersesClient.On("Secret", PersesNamespace).Return(mockSecret)
			getDatasource := mockDatasource.On("Get", DatasourceName).Return(&persesv1.Datasource{}, perseshttp.RequestNotFoundError)
			mockDatasource.On("Create", newDatasource).Return(&persesv1.Datasource{}, nil)
			mockSecret.On("Create", newSecret).Return(&persesv1.Secret{}, nil)

			By("Reconciling the custom resource created")
			datasourceReconciler := &datasourcecontroller.PersesDatasourceReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				ClientFactory: common.NewWithClient(mockPersesClient),
			}

			_, err = datasourceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: datasourceNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			// The datasource was created in the Perses API
			getDatasource.Unset()
			mockDatasource.On("Get", DatasourceName).Return(&persesv1.Datasource{}, nil)

			By("Checking if the Perses API was called to create a datasource")
			Eventually(func() error {
				if !mockDatasource.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a datasource")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses datasource instance")
			Eventually(func() error {
				datasourceWithStatus := &persesv1alpha2.PersesDatasource{}
				err = k8sClient.Get(ctx, datasourceNamespaceName, datasourceWithStatus)

				if len(datasourceWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses datasource instance")
				} else {
					latestStatusCondition := datasourceWithStatus.Status.Conditions[len(datasourceWithStatus.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeAvailablePerses,
						Status: metav1.ConditionTrue, Reason: "Reconciled",
						Message: fmt.Sprintf("Datasource (%s) created successfully", datasourceWithStatus.Name)}
					if latestStatusCondition.Message != expectedLatestStatusCondition.Message || latestStatusCondition.Reason != expectedLatestStatusCondition.Reason || latestStatusCondition.Status != expectedLatestStatusCondition.Status || latestStatusCondition.Type != expectedLatestStatusCondition.Type {
						return fmt.Errorf("The latest status condition added to the perses datasource instance is not as expected. Expected %v but recieved %v", expectedLatestStatusCondition, latestStatusCondition)
					}
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			mockDatasource.On("Delete", DatasourceName).Return(nil)
			mockSecret.On("Delete", PersesSecretName).Return(nil)

			datasourceToDelete := &persesv1alpha2.PersesDatasource{}
			err = k8sClient.Get(ctx, datasourceNamespaceName, datasourceToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Deleting the custom resource")
			err = k8sClient.Delete(ctx, datasourceToDelete)
			Expect(err).To(Not(HaveOccurred()))

			_, err = datasourceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: datasourceNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the Perses API was called to delete a datasource")
			Eventually(func() error {
				if !mockDatasource.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a datasource")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})

		It("should show the error on CR datasource status when the backend returns one", func() {
			By("Creating the custom resource for the Kind PersesDatasource")
			datasource := &persesv1alpha2.PersesDatasource{}
			err := k8sClient.Get(ctx, datasourceNamespaceName, datasource)
			if err != nil && errors.IsNotFound(err) {
				datasource = &persesv1alpha2.PersesDatasource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DatasourceName,
						Namespace: PersesNamespace,
					},
					Spec: persesv1alpha2.DatasourceSpec{
						Config: persesv1alpha2.Datasource{
							DatasourceSpec: newDatasource.Spec,
						},
					},
				}

				err = k8sClient.Create(ctx, datasource)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.PersesDatasource{}
				return k8sClient.Get(ctx, datasourceNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			// Mock the Perses API to assert that Is creating a new datasource when reconciling
			mockPersesClient := new(internal.MockClient)
			mockDatasource := new(internal.MockDatasource)
			mockSecret := new(internal.MockSecret)

			mockPersesClient.On("Datasource", PersesNamespace).Return(mockDatasource)
			mockPersesClient.On("Secret", PersesNamespace).Return(mockSecret)
			mockDatasource.On("Get", DatasourceName).Return(&persesv1.Datasource{}, perseshttp.RequestNotFoundError)
			mockDatasource.On("Create", newDatasource).Return(&persesv1.Datasource{}, perseshttp.RequestInternalError)
			mockSecret.On("Create", newSecret).Return(&persesv1.Secret{}, nil)

			By("Reconciling the custom resource created")
			datasourceReconciler := &datasourcecontroller.PersesDatasourceReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				ClientFactory: common.NewWithClient(mockPersesClient),
			}

			_, err = datasourceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: datasourceNamespaceName,
			})

			Expect(err).To(HaveOccurred())

			By("Checking if the Perses API was called to create a datasource")
			Eventually(func() error {
				if !mockDatasource.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a datasource")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses datasource instance")
			Eventually(func() error {
				datasourceWithStatus := &persesv1alpha2.PersesDatasource{}
				err = k8sClient.Get(ctx, datasourceNamespaceName, datasourceWithStatus)

				if len(datasourceWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses datasource instance")
				} else {
					latestStatusCondition := datasourceWithStatus.Status.Conditions[len(datasourceWithStatus.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeDegradedPerses,
						Status: metav1.ConditionTrue, Reason: string(common.ReasonBackendError),
						Message: "something wrong happened with the request to the API.  Message: internal server error StatusCode: 500"}
					if latestStatusCondition.Message != expectedLatestStatusCondition.Message || latestStatusCondition.Reason != expectedLatestStatusCondition.Reason || latestStatusCondition.Status != expectedLatestStatusCondition.Status || latestStatusCondition.Type != expectedLatestStatusCondition.Type {
						return fmt.Errorf("The latest status condition added to the perses datasource instance is not as expected. Expected %v but recieved %v", expectedLatestStatusCondition, latestStatusCondition)
					}
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			mockDatasource.On("Delete", DatasourceName).Return(nil)
			mockSecret.On("Delete", PersesSecretName).Return(nil)

			datasourceToDelete := &persesv1alpha2.PersesDatasource{}
			err = k8sClient.Get(ctx, datasourceNamespaceName, datasourceToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Deleting the custom resource")
			err = k8sClient.Delete(ctx, datasourceToDelete)
			Expect(err).To(Not(HaveOccurred()))

			_, err = datasourceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: datasourceNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the Perses API was called to delete a datasource")
			Eventually(func() error {
				if !mockDatasource.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a datasource")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
