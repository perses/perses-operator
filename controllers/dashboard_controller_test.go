// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	persesdashboard "github.com/perses/perses/pkg/model/api/v1/dashboard"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	dashboardcontroller "github.com/perses/perses-operator/controllers/dashboards"
	internal "github.com/perses/perses-operator/internal/perses"
	"github.com/perses/perses-operator/internal/perses/common"
)

var _ = Describe("Dashboard controller", Ordered, func() {
	Context("Dashboard controller test", func() {
		const PersesName = "perses-for-dashboard"
		const DashboardName = "my-custom-dashboard"

		ctx := context.Background()

		var namespace *corev1.Namespace
		var persesNamespaceName types.NamespacedName
		var dashboardNamespaceName types.NamespacedName
		var PersesNamespace string

		var newDashboard *persesv1.Dashboard

		BeforeAll(func() {
			By("Creating the Namespace to perform the tests")
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "perses-dashboard-test-",
				},
			}
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
			PersesNamespace = namespace.Name
			persesNamespaceName = types.NamespacedName{Name: PersesName, Namespace: PersesNamespace}
			dashboardNamespaceName = types.NamespacedName{Name: DashboardName, Namespace: PersesNamespace}

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
					Duration: "5m",
					Layouts:  []persesdashboard.Layout{},
					Panels: map[string]*persesv1.Panel{
						"panel1": {
							Kind: "Panel",
							Spec: persesv1.PanelSpec{
								Display: &persesv1.PanelDisplay{
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

			By("Checking the Status Conditions added to the Perses dashboard instance")
			Eventually(func() error {
				dashboardWithStatus := &persesv1alpha2.PersesDashboard{}
				err = k8sClient.Get(ctx, dashboardNamespaceName, dashboardWithStatus)

				if len(dashboardWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses dashboard instance")
				}

				availableCond := apimeta.FindStatusCondition(dashboardWithStatus.Status.Conditions, common.TypeAvailablePerses)
				if availableCond == nil {
					return fmt.Errorf("Available condition not found on the perses dashboard instance")
				}
				expectedAvailable := metav1.Condition{Type: common.TypeAvailablePerses,
					Status: metav1.ConditionTrue, Reason: "Reconciled",
					Message: fmt.Sprintf("Dashboard (%s) created successfully", dashboardWithStatus.Name)}
				if availableCond.Message != expectedAvailable.Message || availableCond.Reason != expectedAvailable.Reason || availableCond.Status != expectedAvailable.Status || availableCond.Type != expectedAvailable.Type {
					return fmt.Errorf("The Available status condition is not as expected. Expected %v but received %v", expectedAvailable, *availableCond)
				}

				degradedCond := apimeta.FindStatusCondition(dashboardWithStatus.Status.Conditions, common.TypeDegradedPerses)
				if degradedCond == nil {
					return fmt.Errorf("Degraded condition not found on the perses dashboard instance")
				}
				expectedDegraded := metav1.Condition{Type: common.TypeDegradedPerses,
					Status: metav1.ConditionFalse, Reason: "Reconciled",
					Message: fmt.Sprintf("Dashboard (%s) reconciled successfully", dashboardWithStatus.Name)}
				if degradedCond.Message != expectedDegraded.Message || degradedCond.Reason != expectedDegraded.Reason || degradedCond.Status != expectedDegraded.Status || degradedCond.Type != expectedDegraded.Type {
					return fmt.Errorf("The Degraded status condition is not as expected. Expected %v but received %v", expectedDegraded, *degradedCond)
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

			By("Checking the Status Conditions added to the Perses dashboard instance")
			Eventually(func() error {
				dashboardWithStatus := &persesv1alpha2.PersesDashboard{}
				err = k8sClient.Get(ctx, dashboardNamespaceName, dashboardWithStatus)

				if len(dashboardWithStatus.Status.Conditions) == 0 {
					return fmt.Errorf("No status condition was added to the perses dashboard instance")
				}

				degradedCond := apimeta.FindStatusCondition(dashboardWithStatus.Status.Conditions, common.TypeDegradedPerses)
				if degradedCond == nil {
					return fmt.Errorf("Degraded condition not found on the perses dashboard instance")
				}
				expectedDegraded := metav1.Condition{Type: common.TypeDegradedPerses,
					Status: metav1.ConditionTrue, Reason: string(common.ReasonBackendError),
					Message: "something wrong happened with the request to the API.  Message: internal server error StatusCode: 500"}
				if degradedCond.Message != expectedDegraded.Message || degradedCond.Reason != expectedDegraded.Reason || degradedCond.Status != expectedDegraded.Status || degradedCond.Type != expectedDegraded.Type {
					return fmt.Errorf("The Degraded status condition is not as expected. Expected %v but received %v", expectedDegraded, *degradedCond)
				}

				availableCond := apimeta.FindStatusCondition(dashboardWithStatus.Status.Conditions, common.TypeAvailablePerses)
				if availableCond == nil {
					return fmt.Errorf("Available condition not found on the perses dashboard instance")
				}
				expectedAvailable := metav1.Condition{Type: common.TypeAvailablePerses,
					Status: metav1.ConditionFalse, Reason: string(common.ReasonBackendError),
					Message: "something wrong happened with the request to the API.  Message: internal server error StatusCode: 500"}
				if availableCond.Message != expectedAvailable.Message || availableCond.Reason != expectedAvailable.Reason || availableCond.Status != expectedAvailable.Status || availableCond.Type != expectedAvailable.Type {
					return fmt.Errorf("The Available status condition is not as expected. Expected %v but received %v", expectedAvailable, *availableCond)
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

	Context("Dashboard controller instance selector test", func() {
		const MatchingPersesName = "perses-matching"
		const NonMatchingPersesName = "perses-non-matching"
		const SelectorDashboardName = "selector-dashboard"

		ctx := context.Background()

		var namespace *corev1.Namespace
		var SelectorNamespace string
		var selectorDashboardNamespaceName types.NamespacedName

		var selectorDashboard *persesv1.Dashboard

		BeforeAll(func() {
			By("Creating the Namespace to perform the tests")
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "perses-selector-test-",
				},
			}
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
			SelectorNamespace = namespace.Name
			selectorDashboardNamespaceName = types.NamespacedName{Name: SelectorDashboardName, Namespace: SelectorNamespace}

			By("Creating a Perses instance with matching labels")
			matchingPerses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      MatchingPersesName,
					Namespace: SelectorNamespace,
					Labels: map[string]string{
						"app": "perses",
						"env": "production",
					},
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
				},
			}
			err = k8sClient.Create(ctx, matchingPerses)
			Expect(err).To(Not(HaveOccurred()))

			// Set the matching Perses instance status to Available
			Eventually(func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: MatchingPersesName, Namespace: SelectorNamespace}, matchingPerses); err != nil {
					return err
				}
				matchingPerses.Status.Conditions = []metav1.Condition{{
					Type:               common.TypeAvailablePerses,
					Status:             metav1.ConditionTrue,
					Reason:             "Testing",
					Message:            "Available for testing",
					LastTransitionTime: metav1.Now(),
				}}
				return k8sClient.Status().Update(ctx, matchingPerses)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Creating a Perses instance with non-matching labels")
			nonMatchingPerses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NonMatchingPersesName,
					Namespace: SelectorNamespace,
					Labels: map[string]string{
						"app": "perses",
						"env": "staging",
					},
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
				},
			}
			err = k8sClient.Create(ctx, nonMatchingPerses)
			Expect(err).To(Not(HaveOccurred()))

			// Set the non-matching Perses instance status to Available
			Eventually(func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: NonMatchingPersesName, Namespace: SelectorNamespace}, nonMatchingPerses); err != nil {
					return err
				}
				nonMatchingPerses.Status.Conditions = []metav1.Condition{{
					Type:               common.TypeAvailablePerses,
					Status:             metav1.ConditionTrue,
					Reason:             "Testing",
					Message:            "Available for testing",
					LastTransitionTime: metav1.Now(),
				}}
				return k8sClient.Status().Update(ctx, nonMatchingPerses)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			selectorDashboard = &persesv1.Dashboard{
				Kind: persesv1.KindDashboard,
				Metadata: persesv1.ProjectMetadata{
					Metadata: persesv1.Metadata{
						Name: SelectorDashboardName,
					},
				},
				Spec: persesv1.DashboardSpec{
					Display: &persescommon.Display{
						Name: SelectorDashboardName,
					},
					Duration: "5m",
					Layouts:  []persesdashboard.Layout{},
					Panels: map[string]*persesv1.Panel{
						"panel1": {
							Kind: "Panel",
							Spec: persesv1.PanelSpec{
								Display: &persesv1.PanelDisplay{
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
		})

		It("should only sync the dashboard with Perses instances matching the instance selector", func() {
			By("Creating the custom resource for the Kind PersesDashboard with instance selector")
			dashboard := &persesv1alpha2.PersesDashboard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SelectorDashboardName,
					Namespace: SelectorNamespace,
				},
				Spec: persesv1alpha2.PersesDashboardSpec{
					Config: persesv1alpha2.Dashboard{
						DashboardSpec: selectorDashboard.Spec,
					},
					InstanceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"env": "production",
						},
					},
				},
			}
			err := k8sClient.Create(ctx, dashboard)
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.PersesDashboard{}
				return k8sClient.Get(ctx, selectorDashboardNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			// Mock the Perses API - should only be called once for the matching instance
			mockPersesClient := new(internal.MockClient)
			mockDashboard := new(internal.MockDashboard)

			mockPersesClient.On("Dashboard", SelectorNamespace).Return(mockDashboard)
			mockDashboard.On("Get", SelectorDashboardName).Return(&persesv1.Dashboard{}, perseshttp.RequestNotFoundError)
			mockDashboard.On("Create", selectorDashboard).Return(&persesv1.Dashboard{}, nil)

			By("Reconciling the custom resource created")
			dashboardReconciler := &dashboardcontroller.PersesDashboardReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				ClientFactory: common.NewWithClient(mockPersesClient),
			}

			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: selectorDashboardNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			By("Checking that the Perses API was called exactly once (only for the matching instance)")
			mockDashboard.AssertNumberOfCalls(GinkgoT(), "Get", 1)
			mockDashboard.AssertNumberOfCalls(GinkgoT(), "Create", 1)

			By("Cleaning up the dashboard resource")
			mockDashboard.On("Delete", SelectorDashboardName).Return(nil)

			dashboardToDelete := &persesv1alpha2.PersesDashboard{}
			err = k8sClient.Get(ctx, selectorDashboardNamespaceName, dashboardToDelete)
			Expect(err).To(Not(HaveOccurred()))

			err = k8sClient.Delete(ctx, dashboardToDelete)
			Expect(err).To(Not(HaveOccurred()))

			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: selectorDashboardNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should sync the dashboard with all Perses instances when no instance selector is provided", func() {
			By("Counting the total number of available Perses instances in the cluster")
			allPerses := &persesv1alpha2.PersesList{}
			err := k8sClient.List(ctx, allPerses)
			Expect(err).To(Not(HaveOccurred()))
			availableInstances := 0
			for _, p := range allPerses.Items {
				if apimeta.IsStatusConditionTrue(p.Status.Conditions, common.TypeAvailablePerses) {
					availableInstances++
				}
			}
			Expect(availableInstances).To(BeNumerically(">", 1), "Expected more than 1 available Perses instance to validate no-selector behavior")

			By("Creating the custom resource for the Kind PersesDashboard without instance selector")
			dashboard := &persesv1alpha2.PersesDashboard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SelectorDashboardName,
					Namespace: SelectorNamespace,
				},
				Spec: persesv1alpha2.PersesDashboardSpec{
					Config: persesv1alpha2.Dashboard{
						DashboardSpec: selectorDashboard.Spec,
					},
					// No InstanceSelector - should match all instances
				},
			}
			err = k8sClient.Create(ctx, dashboard)
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &persesv1alpha2.PersesDashboard{}
				return k8sClient.Get(ctx, selectorDashboardNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			// Mock the Perses API - should be called for all instances
			mockPersesClient := new(internal.MockClient)
			mockDashboard := new(internal.MockDashboard)

			mockPersesClient.On("Dashboard", SelectorNamespace).Return(mockDashboard)
			mockDashboard.On("Get", SelectorDashboardName).Return(&persesv1.Dashboard{}, perseshttp.RequestNotFoundError)
			mockDashboard.On("Create", selectorDashboard).Return(&persesv1.Dashboard{}, nil)

			By("Reconciling the custom resource created")
			dashboardReconciler := &dashboardcontroller.PersesDashboardReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				ClientFactory: common.NewWithClient(mockPersesClient),
			}

			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: selectorDashboardNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			By("Checking that the Perses API was called for all available instances")
			mockDashboard.AssertNumberOfCalls(GinkgoT(), "Get", availableInstances)
			mockDashboard.AssertNumberOfCalls(GinkgoT(), "Create", availableInstances)

			By("Cleaning up the dashboard resource")
			mockDashboard.On("Delete", SelectorDashboardName).Return(nil)

			dashboardToDelete := &persesv1alpha2.PersesDashboard{}
			err = k8sClient.Get(ctx, selectorDashboardNamespaceName, dashboardToDelete)
			Expect(err).To(Not(HaveOccurred()))

			err = k8sClient.Delete(ctx, dashboardToDelete)
			Expect(err).To(Not(HaveOccurred()))

			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: selectorDashboardNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))
		})
	})
})
