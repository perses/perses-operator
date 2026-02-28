package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/perses/perses/pkg/client/perseshttp"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	persescommon "github.com/perses/perses/pkg/model/api/v1/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	datasourcecontroller "github.com/perses/perses-operator/controllers/datasources"
	internal "github.com/perses/perses-operator/internal/perses"
	"github.com/perses/perses-operator/internal/perses/common"
)

var _ = Describe("Datasource controller", Ordered, func() {
	Context("Datasource controller test", func() {
		const PersesName = "perses-for-datasource"
		const DatasourceName = "my-custom-datasource"
		const PersesSecretName = DatasourceName + "-secret"

		ctx := context.Background()

		var namespace *corev1.Namespace
		var PersesNamespace string
		var persesNamespaceName types.NamespacedName
		var datasourceNamespaceName types.NamespacedName

		var newSecret *persesv1.Secret
		var newDatasource *persesv1.Datasource

		BeforeAll(func() {
			By("Creating the Namespace to perform the tests")
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "perses-datasource-test-",
				},
			}
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
			PersesNamespace = namespace.Name
			persesNamespaceName = types.NamespacedName{Name: PersesName, Namespace: PersesNamespace}
			datasourceNamespaceName = types.NamespacedName{Name: DatasourceName, Namespace: PersesNamespace}

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
						ContainerPort: ptr.To(int32(8080)),
					},
				}

				err = k8sClient.Create(ctx, perses)
				Expect(err).To(Not(HaveOccurred()))

				// Set the Perses instance status to Available so child controllers
				// consider it ready for syncing.
				// Use Eventually to handle potential resource version conflicts
				Eventually(func() error {
					// Fetch the latest version of the resource
					if err := k8sClient.Get(ctx, persesNamespaceName, perses); err != nil {
						return err
					}
					perses.Status.Conditions = []metav1.Condition{{
						Type:               common.TypeAvailablePerses,
						Status:             metav1.ConditionTrue,
						Reason:             "Testing",
						Message:            "Available for testing",
						LastTransitionTime: metav1.Now(),
					}}
					return k8sClient.Status().Update(ctx, perses)
				}, time.Second*10, time.Millisecond*250).Should(Succeed())
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

			By("Checking the Status Conditions added to the Perses datasource instance")
			Eventually(func() error {
				datasourceWithStatus := &persesv1alpha2.PersesDatasource{}
				err = k8sClient.Get(ctx, datasourceNamespaceName, datasourceWithStatus)

				if len(datasourceWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses datasource instance")
				}

				availableCond := apimeta.FindStatusCondition(datasourceWithStatus.Status.Conditions, common.TypeAvailablePerses)
				if availableCond == nil {
					return fmt.Errorf("Available condition not found on the perses datasource instance")
				}
				expectedAvailable := metav1.Condition{Type: common.TypeAvailablePerses,
					Status: metav1.ConditionTrue, Reason: "Reconciled",
					Message: fmt.Sprintf("Datasource (%s) created successfully", datasourceWithStatus.Name)}
				if availableCond.Message != expectedAvailable.Message || availableCond.Reason != expectedAvailable.Reason || availableCond.Status != expectedAvailable.Status || availableCond.Type != expectedAvailable.Type {
					return fmt.Errorf("The Available status condition is not as expected. Expected %v but received %v", expectedAvailable, *availableCond)
				}

				degradedCond := apimeta.FindStatusCondition(datasourceWithStatus.Status.Conditions, common.TypeDegradedPerses)
				if degradedCond == nil {
					return fmt.Errorf("Degraded condition not found on the perses datasource instance")
				}
				expectedDegraded := metav1.Condition{Type: common.TypeDegradedPerses,
					Status: metav1.ConditionFalse, Reason: "Reconciled",
					Message: fmt.Sprintf("Datasource (%s) reconciled successfully", datasourceWithStatus.Name)}
				if degradedCond.Message != expectedDegraded.Message || degradedCond.Reason != expectedDegraded.Reason || degradedCond.Status != expectedDegraded.Status || degradedCond.Type != expectedDegraded.Type {
					return fmt.Errorf("The Degraded status condition is not as expected. Expected %v but received %v", expectedDegraded, *degradedCond)
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

			By("Checking the Status Conditions added to the Perses datasource instance")
			Eventually(func() error {
				datasourceWithStatus := &persesv1alpha2.PersesDatasource{}
				err = k8sClient.Get(ctx, datasourceNamespaceName, datasourceWithStatus)

				if len(datasourceWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses datasource instance")
				}

				degradedCond := apimeta.FindStatusCondition(datasourceWithStatus.Status.Conditions, common.TypeDegradedPerses)
				if degradedCond == nil {
					return fmt.Errorf("Degraded condition not found on the perses datasource instance")
				}
				expectedDegraded := metav1.Condition{Type: common.TypeDegradedPerses,
					Status: metav1.ConditionTrue, Reason: string(common.ReasonBackendError),
					Message: "something wrong happened with the request to the API.  Message: internal server error StatusCode: 500"}
				if degradedCond.Message != expectedDegraded.Message || degradedCond.Reason != expectedDegraded.Reason || degradedCond.Status != expectedDegraded.Status || degradedCond.Type != expectedDegraded.Type {
					return fmt.Errorf("The Degraded status condition is not as expected. Expected %v but received %v", expectedDegraded, *degradedCond)
				}

				availableCond := apimeta.FindStatusCondition(datasourceWithStatus.Status.Conditions, common.TypeAvailablePerses)
				if availableCond == nil {
					return fmt.Errorf("Available condition not found on the perses datasource instance")
				}
				expectedAvailable := metav1.Condition{Type: common.TypeAvailablePerses,
					Status: metav1.ConditionFalse, Reason: string(common.ReasonBackendError),
					Message: "something wrong happened with the request to the API.  Message: internal server error StatusCode: 500"}
				if availableCond.Message != expectedAvailable.Message || availableCond.Reason != expectedAvailable.Reason || availableCond.Status != expectedAvailable.Status || availableCond.Type != expectedAvailable.Type {
					return fmt.Errorf("The Available status condition is not as expected. Expected %v but received %v", expectedAvailable, *availableCond)
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
