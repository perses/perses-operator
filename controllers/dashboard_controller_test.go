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
	persesdashboard "github.com/perses/perses/pkg/model/api/v1/dashboard"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	dashboardcontroller "github.com/perses/perses-operator/controllers/dashboards"
	internal "github.com/perses/perses-operator/internal/perses"
	"github.com/perses/perses-operator/internal/perses/common"
)

var _ = Describe("Dashboard controller", Ordered, func() {
	Context("Dashboard controller test", func() {
		const PersesName = "perses-for-dashboard"
		const PersesNamespace = "perses-dashboard-test"
		const DashboardName = "my-custom-dashboard"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PersesNamespace,
				Namespace: PersesNamespace,
			},
		}

		persesNamespaceName := types.NamespacedName{Name: PersesName, Namespace: PersesNamespace}
		dashboardNamespaceName := types.NamespacedName{Name: DashboardName, Namespace: PersesNamespace}

		persesImage := "perses-dev.io/perses:test"

		var newDashboard *persesv1.Dashboard

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

			newDashboard = &persesv1.Dashboard{
				Kind: persesv1.KindDashboard,
				Metadata: persesv1.ProjectMetadata{
					Metadata: persesv1.Metadata{
						Name: DashboardName,
					},
				},
				Spec: persesv1.DashboardSpec{
					Display: &persescommon.Display{
						Name: DashboardName,
					},
					Layouts: []persesdashboard.Layout{},
					Panels: map[string]*persesv1.Panel{
						"panel1": {
							Kind: "Panel",
							Spec: persesv1.PanelSpec{
								Display: persesv1.PanelDisplay{
									Name: "test-panel",
								},
								Plugin: persescommon.Plugin{
									Kind: "PrometheusPlugin",
									Spec: map[string]any{},
								},
							},
						},
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

		It("should successfully reconcile a custom resource dashboard for Perses", func() {
			By("Creating the custom resource for the Kind PersesDashboard")
			dashboard := &persesv1alpha2.PersesDashboard{}
			err := k8sClient.Get(ctx, dashboardNamespaceName, dashboard)
			if err != nil && errors.IsNotFound(err) {
				perses := &persesv1alpha2.PersesDashboard{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DashboardName,
						Namespace: PersesNamespace,
					},
					Spec: persesv1alpha2.PersesDashboardSpec{
						Config: persesv1alpha2.Dashboard{
							DashboardSpec: newDashboard.Spec,
						},
					},
				}

				err = k8sClient.Create(ctx, perses)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.PersesDashboard{}
				return k8sClient.Get(ctx, dashboardNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			// Mock the Perses API to assert that Is creating a new dashboard when reconciling
			mockPersesClient := new(internal.MockClient)
			mockDashboard := new(internal.MockDashboard)

			mockPersesClient.On("Dashboard", PersesNamespace).Return(mockDashboard)
			getDashboard := mockDashboard.On("Get", DashboardName).Return(&persesv1.Dashboard{}, perseshttp.RequestNotFoundError)
			mockDashboard.On("Create", newDashboard).Return(&persesv1.Dashboard{}, nil)

			By("Reconciling the custom resource created")
			dashboardReconciler := &dashboardcontroller.PersesDashboardReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				ClientFactory: common.NewWithClient(mockPersesClient),
			}

			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: dashboardNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			// The dashboard was created in the Perses API
			getDashboard.Unset()
			mockDashboard.On("Get", DashboardName).Return(&persesv1.Dashboard{}, nil)

			By("Checking if the Perses API was called to create a dashboard")
			Eventually(func() error {
				if !mockDashboard.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a dashboard")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses dashboard instance")
			Eventually(func() error {
				dashboardWithStatus := &persesv1alpha2.PersesDashboard{}
				err = k8sClient.Get(ctx, dashboardNamespaceName, dashboardWithStatus)

				if len(dashboardWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses dashboard instance")
				} else {
					latestStatusCondition := dashboardWithStatus.Status.Conditions[len(dashboardWithStatus.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeAvailablePerses,
						Status: metav1.ConditionTrue, Reason: "Reconciled",
						Message: fmt.Sprintf("Dashboard (%s) created successfully", dashboardWithStatus.Name)}
					if latestStatusCondition.Message != expectedLatestStatusCondition.Message || latestStatusCondition.Reason != expectedLatestStatusCondition.Reason || latestStatusCondition.Status != expectedLatestStatusCondition.Status || latestStatusCondition.Type != expectedLatestStatusCondition.Type {
						return fmt.Errorf("The latest status condition added to the perses dashboard instance is not as expected. Expected %v but recieved %v", expectedLatestStatusCondition, latestStatusCondition)
					}
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			mockDashboard.On("Delete", DashboardName).Return(nil)

			dashboardToDelete := &persesv1alpha2.PersesDashboard{}
			err = k8sClient.Get(ctx, dashboardNamespaceName, dashboardToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Deleting the custom resource")
			err = k8sClient.Delete(ctx, dashboardToDelete)
			Expect(err).To(Not(HaveOccurred()))

			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: dashboardNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the Perses API was called to delete a dashboard")
			Eventually(func() error {
				if !mockDashboard.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a dashboard")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})

		It("should show the error on CR dashboard status when the backend returns one", func() {
			By("Creating the custom resource for the Kind PersesDashboard")
			dashboard := &persesv1alpha2.PersesDashboard{}
			err := k8sClient.Get(ctx, dashboardNamespaceName, dashboard)
			if err != nil && errors.IsNotFound(err) {
				perses := &persesv1alpha2.PersesDashboard{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DashboardName,
						Namespace: PersesNamespace,
					},
					Spec: persesv1alpha2.PersesDashboardSpec{
						Config: persesv1alpha2.Dashboard{
							DashboardSpec: newDashboard.Spec,
						},
					},
				}

				err = k8sClient.Create(ctx, perses)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.PersesDashboard{}
				return k8sClient.Get(ctx, dashboardNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			// Mock the Perses API to assert that Is creating a new dashboard when reconciling
			mockPersesClient := new(internal.MockClient)
			mockDashboard := new(internal.MockDashboard)

			mockPersesClient.On("Dashboard", PersesNamespace).Return(mockDashboard)
			mockDashboard.On("Get", DashboardName).Return(&persesv1.Dashboard{}, perseshttp.RequestNotFoundError)
			mockDashboard.On("Create", newDashboard).Return(&persesv1.Dashboard{}, perseshttp.RequestInternalError)

			By("Reconciling the custom resource created")
			dashboardReconciler := &dashboardcontroller.PersesDashboardReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				ClientFactory: common.NewWithClient(mockPersesClient),
			}

			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: dashboardNamespaceName,
			})

			Expect(err).To(HaveOccurred())

			By("Checking if the Perses API was called to create a dashboard")
			Eventually(func() error {
				if !mockDashboard.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a dashboard")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses dashboard instance")
			Eventually(func() error {
				dashboardWithStatus := &persesv1alpha2.PersesDashboard{}
				err = k8sClient.Get(ctx, dashboardNamespaceName, dashboardWithStatus)

				if len(dashboardWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses dashboard instance")
				} else {
					latestStatusCondition := dashboardWithStatus.Status.Conditions[len(dashboardWithStatus.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeDegradedPerses,
						Status: metav1.ConditionTrue, Reason: string(common.ReasonBackendError),
						Message: "something wrong happened with the request to the API.  Message: internal server error StatusCode: 500"}
					if latestStatusCondition.Message != expectedLatestStatusCondition.Message || latestStatusCondition.Reason != expectedLatestStatusCondition.Reason || latestStatusCondition.Status != expectedLatestStatusCondition.Status || latestStatusCondition.Type != expectedLatestStatusCondition.Type {
						return fmt.Errorf("The latest status condition added to the perses dashboard instance is not as expected. Expected %v but recieved %v", expectedLatestStatusCondition, latestStatusCondition)
					}
				}

				return err
			}, time.Minute, time.Second).Should(Succeed())

			mockDashboard.On("Delete", DashboardName).Return(nil)

			dashboardToDelete := &persesv1alpha2.PersesDashboard{}
			err = k8sClient.Get(ctx, dashboardNamespaceName, dashboardToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Deleting the custom resource")
			err = k8sClient.Delete(ctx, dashboardToDelete)
			Expect(err).To(Not(HaveOccurred()))

			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: dashboardNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the Perses API was called to delete a dashboard")
			Eventually(func() error {
				if !mockDashboard.AssertExpectations(GinkgoT()) {
					return fmt.Errorf("The Perses API was not called to create a dashboard")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
