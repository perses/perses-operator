package controllers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	dashboardcontroller "github.com/perses/perses-operator/controllers/dashboards"
	common "github.com/perses/perses-operator/internal/perses/common"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	persescommon "github.com/perses/perses/pkg/model/api/v1/common"
	persesdashboard "github.com/perses/perses/pkg/model/api/v1/dashboard"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Dashboard controller", func() {
	Context("Dashboard controller test", func() {
		const PersesName = "test-perses-dashboard"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PersesName,
				Namespace: PersesName,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: PersesName, Namespace: PersesName}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource dashboard for Perses", func() {
			By("Creating the custom resource for the Kind PersesDashboard")
			dashboard := &persesv1alpha1.PersesDashboard{}
			err := k8sClient.Get(ctx, typeNamespaceName, dashboard)
			if err != nil && errors.IsNotFound(err) {
				perses := &persesv1alpha1.PersesDashboard{
					ObjectMeta: metav1.ObjectMeta{
						Name:      PersesName,
						Namespace: namespace.Name,
					},
					Spec: persesv1alpha1.Dashboard{
						Dashboard: persesv1.Dashboard{
							Kind: "Dashboard",
							Spec: persesv1.DashboardSpec{
								Display: &persescommon.Display{
									Name: "test-dashboard",
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
												Spec: map[string]interface{}{},
											},
										},
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
				found := &persesv1alpha1.PersesDashboard{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "expected response")
			}))
			defer svr.Close()

			By("Reconciling the custom resource created")
			dashboardReconciler := &dashboardcontroller.PersesDashboardReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				ClientFactory: common.NewWithURL(svr.URL),
			}

			// Errors might arise during reconciliation, but we are checking the final state of the resources
			_, err = dashboardReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})

			Expect(err).To(Not(HaveOccurred()))

			// By("Checking if the Perses API was called to create a dashboard")
			// Eventually(func() error {
			// 	return err
			// }, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses instance")
			Eventually(func() error {
				if dashboard.Status.Conditions != nil && len(dashboard.Status.Conditions) != 0 {
					latestStatusCondition := dashboard.Status.Conditions[len(dashboard.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeAvailablePerses,
						Status: metav1.ConditionTrue, Reason: "Reconciling",
						Message: fmt.Sprintf("Dashboard (%s) created successfully", dashboard.Name)}
					if latestStatusCondition != expectedLatestStatusCondition {
						return fmt.Errorf("The latest status condition added to the perses dashboard instance is not as expected")
					}
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			persesToDelete := &persesv1alpha1.PersesDashboard{}
			err = k8sClient.Get(ctx, typeNamespaceName, persesToDelete)
			Expect(err).To(Not(HaveOccurred()))

			By("Deleting the custom resource")
			err = k8sClient.Delete(ctx, persesToDelete)
			Expect(err).To(Not(HaveOccurred()))

			// By("Checking if the Perses API was called to delete a dashboard")
			// Eventually(func() error {
			// 	return err
			// }, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the Perses instance")
			Eventually(func() error {
				if dashboard.Status.Conditions != nil && len(dashboard.Status.Conditions) != 0 {
					latestStatusCondition := dashboard.Status.Conditions[len(dashboard.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: common.TypeAvailablePerses,
						Status: metav1.ConditionTrue, Reason: "Finalizing",
						Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", dashboard.Name)}
					if latestStatusCondition != expectedLatestStatusCondition {
						return fmt.Errorf("The latest status condition added to the perses instance is not as expected")
					}
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
