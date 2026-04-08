// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dashboards

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDashboardController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dashboard Controller Suite")
}

func newTestDashboardReconciler(objects ...runtime.Object) *PersesDashboardReconciler {
	scheme := runtime.NewScheme()
	Expect(persesv1alpha2.AddToScheme(scheme)).To(Succeed())

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
	for _, obj := range objects {
		clientBuilder = clientBuilder.WithRuntimeObjects(obj)
	}
	clientBuilder = clientBuilder.WithStatusSubresource(&persesv1alpha2.PersesDashboard{})

	return &PersesDashboardReconciler{
		Client: clientBuilder.Build(),
		Scheme: scheme,
	}
}

var _ = Describe("Dashboard controller", func() {
	Context("setStatusToDegraded", func() {
		const DashboardName = "test-dashboard"
		const DashboardNamespace = "default"

		It("should not panic when degradedError is nil", func() {
			dashboard := &persesv1alpha2.PersesDashboard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DashboardName,
					Namespace: DashboardNamespace,
				},
			}

			r := newTestDashboardReconciler(dashboard)
			ctx := withDashboard(context.Background(), dashboard)
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: DashboardName, Namespace: DashboardNamespace}}

			result, err := r.setStatusToDegraded(ctx, req, &ctrl.Result{RequeueAfter: time.Minute}, common.ReasonMissingPerses, nil)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.RequeueAfter).To(Equal(time.Minute))

			By("Checking the status conditions were set with 'unknown error' message")
			fresh := &persesv1alpha2.PersesDashboard{}
			Expect(r.Get(context.Background(), req.NamespacedName, fresh)).To(Succeed())

			availableCond := apimeta.FindStatusCondition(fresh.Status.Conditions, common.TypeAvailablePerses)
			Expect(availableCond).ToNot(BeNil())
			Expect(availableCond.Message).To(Equal("unknown error"))
			Expect(availableCond.Status).To(Equal(metav1.ConditionFalse))

			degradedCond := apimeta.FindStatusCondition(fresh.Status.Conditions, common.TypeDegradedPerses)
			Expect(degradedCond).ToNot(BeNil())
			Expect(degradedCond.Message).To(Equal("unknown error"))
			Expect(degradedCond.Status).To(Equal(metav1.ConditionTrue))
		})

		It("should set degraded status conditions with the error message", func() {
			dashboard := &persesv1alpha2.PersesDashboard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DashboardName,
					Namespace: DashboardNamespace,
				},
			}

			r := newTestDashboardReconciler(dashboard)
			ctx := withDashboard(context.Background(), dashboard)
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: DashboardName, Namespace: DashboardNamespace}}

			degradedErr := fmt.Errorf("no Perses instances found matching the label selector")
			result, err := r.setStatusToDegraded(ctx, req, &ctrl.Result{RequeueAfter: time.Minute}, common.ReasonMissingPerses, degradedErr)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(degradedErr))
			Expect(result).ToNot(BeNil())
			Expect(result.RequeueAfter).To(Equal(time.Minute))

			By("Checking the status conditions were set with the correct error message")
			fresh := &persesv1alpha2.PersesDashboard{}
			Expect(r.Get(context.Background(), req.NamespacedName, fresh)).To(Succeed())
			Expect(fresh.Status.Conditions).To(HaveLen(2))

			availableCond := apimeta.FindStatusCondition(fresh.Status.Conditions, common.TypeAvailablePerses)
			Expect(availableCond).ToNot(BeNil())
			Expect(availableCond.Message).To(Equal(degradedErr.Error()))
			Expect(availableCond.Reason).To(Equal(string(common.ReasonMissingPerses)))

			degradedCond := apimeta.FindStatusCondition(fresh.Status.Conditions, common.TypeDegradedPerses)
			Expect(degradedCond).ToNot(BeNil())
			Expect(degradedCond.Message).To(Equal(degradedErr.Error()))
			Expect(degradedCond.Reason).To(Equal(string(common.ReasonMissingPerses)))
		})
	})
})
