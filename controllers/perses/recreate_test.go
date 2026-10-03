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

package perses

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	persesconfig "github.com/perses/perses/pkg/model/api/config"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
)

var _ = Describe("Recreating workloads whose selector changed", func() {
	const (
		persesName = "test-perses"
		namespace  = "default"
		staleUID   = types.UID("stale-uid")
	)

	key := types.NamespacedName{Name: persesName, Namespace: namespace}

	// calls records what the reconciler asked the API server to do.
	type calls struct {
		updates       int
		dryRunCreates int
		deletes       []client.DeleteOptions
	}

	// newReconciler builds a reconciler over a fake client. updateErr is
	// returned from every Update, createErr from every dry-run Create.
	newReconciler := func(objects []client.Object, updateErr, createErr error, c *calls) *PersesReconciler {
		scheme := runtime.NewScheme()
		Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())
		Expect(appsv1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		cl := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objects...).
			WithStatusSubresource(&v1alpha2.Perses{}).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					co := client.CreateOptions{}
					co.ApplyOptions(opts)
					if len(co.DryRun) > 0 {
						c.dryRunCreates++
						if createErr != nil {
							return createErr
						}
					}
					return cl.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					c.updates++
					if updateErr != nil {
						return updateErr
					}
					return cl.Update(ctx, obj, opts...)
				},
				Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					do := client.DeleteOptions{}
					do.ApplyOptions(opts)
					c.deletes = append(c.deletes, do)
					return cl.Delete(ctx, obj, opts...)
				},
			}).
			Build()

		return &PersesReconciler{
			Client:    cl,
			APIReader: cl,
			Scheme:    scheme,
			Config:    Config{PersesImage: "perses/perses:latest"},
		}
	}

	// newPerses returns a Perses with a file database, which is served by a
	// StatefulSet, or by a Deployment when emptyDir is set. It carries a user
	// label, which older operator versions folded into the selector.
	newPerses := func(emptyDir bool) *v1alpha2.Perses {
		perses := &v1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: persesName, Namespace: namespace},
			Spec: v1alpha2.PersesSpec{
				Metadata: &v1alpha2.Metadata{
					Labels: map[string]string{"custom-label": "custom-value"},
				},
				Config: v1alpha2.PersesConfig{
					Config: persesconfig.Config{
						Database: persesconfig.Database{
							File: &persesconfig.File{Folder: "/etc/perses/storage"},
						},
					},
				},
			},
		}
		if emptyDir {
			perses.Spec.Storage = &v1alpha2.StorageConfiguration{EmptyDir: &corev1.EmptyDirVolumeSource{}}
		}
		return perses
	}

	// staleSelector stands in for the selector of a workload created by an
	// older operator version, which folded in all of LabelsForPerses,
	// including the user label. Guard against it silently matching the
	// desired selector, which would make the recreate tests vacuous.
	staleSelector := func(perses *v1alpha2.Perses) *metav1.LabelSelector {
		stale := common.LabelsForPerses(perses.Name, perses)
		Expect(stale).NotTo(Equal(common.SelectorLabelsForPerses(perses.Name, perses)))
		return &metav1.LabelSelector{MatchLabels: stale}
	}

	// existingDeployment returns the Deployment the operator would create, with
	// its selector replaced when selector is not nil.
	existingDeployment := func(r *PersesReconciler, perses *v1alpha2.Perses, selector *metav1.LabelSelector) *appsv1.Deployment {
		dep, err := r.createPersesDeployment(perses)
		Expect(err).NotTo(HaveOccurred())
		dep.UID = staleUID
		if selector != nil {
			dep.Spec.Selector = selector
		}
		return dep
	}

	existingStatefulSet := func(r *PersesReconciler, perses *v1alpha2.Perses, selector *metav1.LabelSelector) *appsv1.StatefulSet {
		sts, err := r.createPersesStatefulSet(perses)
		Expect(err).NotTo(HaveOccurred())
		sts.UID = staleUID
		if selector != nil {
			sts.Spec.Selector = selector
		}
		return sts
	}

	// template builds objects from the desired spec before the fake client
	// that is used for the actual test exists.
	template := newReconciler(nil, nil, nil, &calls{})

	expectOrphanedWithUID := func(c *calls) {
		Expect(c.deletes).To(HaveLen(1))
		Expect(c.deletes[0].PropagationPolicy).To(HaveValue(Equal(metav1.DeletePropagationOrphan)))
		Expect(c.deletes[0].Preconditions).NotTo(BeNil())
		Expect(c.deletes[0].Preconditions.UID).To(HaveValue(Equal(staleUID)))
	}

	It("recreates a Deployment whose selector differs from the desired one", func() {
		perses := newPerses(true)
		c := &calls{}
		r := newReconciler([]client.Object{perses, existingDeployment(template, perses, staleSelector(perses))}, nil, nil, c)
		ctx := withPerses(context.Background(), perses)

		result, err := r.reconcileDeployment(ctx, ctrl.Request{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))

		By("validating the desired Deployment before deleting")
		Expect(c.dryRunCreates).To(Equal(1))

		By("orphaning the pods instead of updating")
		expectOrphanedWithUID(c)
		Expect(c.updates).To(BeZero())
		Expect(apierrors.IsNotFound(r.Get(ctx, key, &appsv1.Deployment{}))).To(BeTrue())

		By("creating it again with the stable selector on the next reconcile")
		_, err = r.reconcileDeployment(ctx, ctrl.Request{})
		Expect(err).NotTo(HaveOccurred())
		recreated := &appsv1.Deployment{}
		Expect(r.Get(ctx, key, recreated)).To(Succeed())
		Expect(recreated.Spec.Selector.MatchLabels).To(Equal(common.SelectorLabelsForPerses(perses.Name, perses)))
	})

	It("recreates a StatefulSet whose selector differs from the desired one", func() {
		perses := newPerses(false)
		c := &calls{}
		r := newReconciler([]client.Object{perses, existingStatefulSet(template, perses, staleSelector(perses))}, nil, nil, c)
		ctx := withPerses(context.Background(), perses)

		result, err := r.reconcileStatefulSet(ctx, ctrl.Request{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))

		By("validating the desired StatefulSet before deleting")
		Expect(c.dryRunCreates).To(Equal(1))

		By("orphaning the pods instead of updating")
		expectOrphanedWithUID(c)
		Expect(c.updates).To(BeZero())
		Expect(apierrors.IsNotFound(r.Get(ctx, key, &appsv1.StatefulSet{}))).To(BeTrue())

		By("creating it again with the stable selector on the next reconcile")
		_, err = r.reconcileStatefulSet(ctx, ctrl.Request{})
		Expect(err).NotTo(HaveOccurred())
		recreated := &appsv1.StatefulSet{}
		Expect(r.Get(ctx, key, recreated)).To(Succeed())
		Expect(recreated.Spec.Selector.MatchLabels).To(Equal(common.SelectorLabelsForPerses(perses.Name, perses)))
	})

	It("waits for a Deployment that is being deleted instead of deleting it again", func() {
		perses := newPerses(true)
		terminating := existingDeployment(template, perses, staleSelector(perses))
		terminating.DeletionTimestamp = ptr.To(metav1.Now())
		terminating.Finalizers = []string{"orphan"}

		c := &calls{}
		r := newReconciler([]client.Object{perses, terminating}, nil, nil, c)

		result, err := r.reconcileDeployment(withPerses(context.Background(), perses), ctrl.Request{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		Expect(c.deletes).To(BeEmpty())
		Expect(c.updates).To(BeZero())
	})

	It("waits for a StatefulSet that is being deleted instead of deleting it again", func() {
		perses := newPerses(false)
		terminating := existingStatefulSet(template, perses, staleSelector(perses))
		terminating.DeletionTimestamp = ptr.To(metav1.Now())
		terminating.Finalizers = []string{"orphan"}

		c := &calls{}
		r := newReconciler([]client.Object{perses, terminating}, nil, nil, c)

		result, err := r.reconcileStatefulSet(withPerses(context.Background(), perses), ctrl.Request{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		Expect(c.deletes).To(BeEmpty())
		Expect(c.updates).To(BeZero())
	})

	// A rejected update that is not about the selector, such as an invalid
	// resources value in the Perses spec, must not take the running workload
	// down: recreating it would fail with the same error.
	DescribeTable("keeps the workload when an update is rejected for another reason",
		func(emptyDir bool, updateErr error) {
			perses := newPerses(emptyDir)
			var existing client.Object
			if emptyDir {
				existing = existingDeployment(template, perses, nil)
			} else {
				existing = existingStatefulSet(template, perses, nil)
			}

			c := &calls{}
			r := newReconciler([]client.Object{perses, existing}, updateErr, nil, c)
			ctx := withPerses(context.Background(), perses)

			var err error
			if emptyDir {
				_, err = r.reconcileDeployment(ctx, ctrl.Request{})
				Expect(r.Get(ctx, key, &appsv1.Deployment{})).To(Succeed())
			} else {
				_, err = r.reconcileStatefulSet(ctx, ctrl.Request{})
				Expect(r.Get(ctx, key, &appsv1.StatefulSet{})).To(Succeed())
			}

			Expect(err).To(MatchError(updateErr))
			Expect(c.updates).To(Equal(1))
			Expect(c.deletes).To(BeEmpty())
		},
		Entry("Deployment, invalid field", true, invalidResourcesErr("Deployment")),
		Entry("Deployment, conflict", true, conflictErr("deployments")),
		Entry("StatefulSet, invalid field", false, invalidResourcesErr("StatefulSet")),
		Entry("StatefulSet, conflict", false, conflictErr("statefulsets")),
	)

	// When the selector changed but the desired spec is itself invalid, the
	// existing workload must be kept: deleting it would leave the instance
	// without a workload until the spec is fixed.
	DescribeTable("keeps the workload when the desired spec is rejected during recreation",
		func(emptyDir bool) {
			perses := newPerses(emptyDir)
			var existing client.Object
			var createErr error
			if emptyDir {
				existing = existingDeployment(template, perses, staleSelector(perses))
				createErr = invalidResourcesErr("Deployment")
			} else {
				existing = existingStatefulSet(template, perses, staleSelector(perses))
				createErr = invalidResourcesErr("StatefulSet")
			}

			c := &calls{}
			r := newReconciler([]client.Object{perses, existing}, nil, createErr, c)
			ctx := withPerses(context.Background(), perses)

			var err error
			if emptyDir {
				_, err = r.reconcileDeployment(ctx, ctrl.Request{})
				Expect(r.Get(ctx, key, &appsv1.Deployment{})).To(Succeed())
			} else {
				_, err = r.reconcileStatefulSet(ctx, ctrl.Request{})
				Expect(r.Get(ctx, key, &appsv1.StatefulSet{})).To(Succeed())
			}

			// The API server's message names the dry-run probe, so the error
			// must be wrapped with the real object and still unwrap to the cause.
			Expect(err).To(MatchError(createErr))
			Expect(err.Error()).To(ContainSubstring(namespace + "/" + persesName))
			Expect(c.dryRunCreates).To(Equal(1))
			Expect(c.deletes).To(BeEmpty())
			Expect(c.updates).To(BeZero())
		},
		Entry("Deployment", true),
		Entry("StatefulSet", false),
	)

	// The delayed requeue asked for by the recreation must reach the
	// controller; Reconcile must not discard the halting subreconciler's result.
	It("requeues with a delay from the top-level Reconcile while recreating", func() {
		perses := newPerses(true)
		// An existing condition keeps setStatusToUnknown from halting first.
		meta.SetStatusCondition(&perses.Status.Conditions, metav1.Condition{
			Type: common.TypeAvailablePerses, Status: metav1.ConditionTrue, Reason: "Ready",
		})
		c := &calls{}
		r := newReconciler([]client.Object{perses, existingDeployment(template, perses, staleSelector(perses))}, nil, nil, c)
		ctx := context.Background()

		By("deleting the stale Deployment and asking for a timed retry")
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		expectOrphanedWithUID(c)

		By("creating it again on the next reconcile and settling")
		result, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeZero())
		recreated := &appsv1.Deployment{}
		Expect(r.Get(ctx, key, recreated)).To(Succeed())
		Expect(recreated.Spec.Selector.MatchLabels).To(Equal(common.SelectorLabelsForPerses(perses.Name, perses)))
	})

	It("updates a workload whose selector already matches without deleting it", func() {
		perses := newPerses(false)
		c := &calls{}
		r := newReconciler([]client.Object{perses, existingStatefulSet(template, perses, nil)}, nil, nil, c)
		ctx := withPerses(context.Background(), perses)

		_, err := r.reconcileStatefulSet(ctx, ctrl.Request{})
		Expect(err).NotTo(HaveOccurred())
		Expect(c.deletes).To(BeEmpty())
		Expect(r.Get(ctx, key, &appsv1.StatefulSet{})).To(Succeed())
	})

	// Older operator versions used the full label set as selector. Without
	// user labels that equals the current selector, so such workloads must be
	// left alone on upgrade.
	DescribeTable("does not recreate a workload without user labels created by an older operator version",
		func(emptyDir bool) {
			perses := newPerses(emptyDir)
			perses.Spec.Metadata = nil
			oldSelector := &metav1.LabelSelector{MatchLabels: common.LabelsForPerses(perses.Name, perses)}

			var existing client.Object
			if emptyDir {
				existing = existingDeployment(template, perses, oldSelector)
			} else {
				existing = existingStatefulSet(template, perses, oldSelector)
			}

			c := &calls{}
			r := newReconciler([]client.Object{perses, existing}, nil, nil, c)
			ctx := withPerses(context.Background(), perses)

			var err error
			if emptyDir {
				_, err = r.reconcileDeployment(ctx, ctrl.Request{})
			} else {
				_, err = r.reconcileStatefulSet(ctx, ctrl.Request{})
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(c.deletes).To(BeEmpty())
			Expect(c.dryRunCreates).To(BeZero())
		},
		Entry("Deployment", true),
		Entry("StatefulSet", false),
	)
})

// invalidResourcesErr mimics the API server rejecting an update because of an
// invalid value in the pod template, unrelated to the selector.
func invalidResourcesErr(kind string) error {
	path := field.NewPath("spec", "template", "spec", "containers").Index(0).Child("resources", "requests")
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "apps", Kind: kind},
		"test-perses",
		field.ErrorList{field.Invalid(path, "2", "must be less than or equal to cpu limit of 1")},
	)
}

func conflictErr(resource string) error {
	return apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: resource}, "test-perses", errors.New("the object has been modified"))
}
