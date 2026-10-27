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

package common

import (
	"strings"
	"testing"

	"github.com/perses/perses-operator/api/v1alpha2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"
)

func TestLabels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Labels Suite")
}

var _ = Describe("ImageForPerses", func() {
	DescribeTable("resolves the correct image",
		func(specImage *string, flagImage string, expectedImage string, expectErr bool, errSubstring string) {
			perses := &v1alpha2.Perses{
				Spec: v1alpha2.PersesSpec{
					Image: specImage,
				},
			}
			image, err := ImageForPerses(perses, flagImage)
			if expectErr {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(errSubstring))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(expectedImage))
			}
		},
		Entry("spec.image takes priority over flag",
			ptr.To("custom/perses:v1.0.0"), "default/perses:v2.0.0",
			"custom/perses:v1.0.0", false, ""),
		Entry("falls back to flag when spec.image is nil",
			nil, "default/perses:v2.0.0",
			"default/perses:v2.0.0", false, ""),
		Entry("falls back to flag when spec.image is empty",
			ptr.To(""), "default/perses:v2.0.0",
			"default/perses:v2.0.0", false, ""),
		Entry("errors when neither spec.image nor flag is set",
			nil, "",
			"", true, "no image specified"),
		Entry("errors when image has no tag",
			ptr.To("perses/perses"), "",
			"", true, "must include a tag"),
	)
})

var _ = Describe("LabelsForPerses", func() {
	DescribeTable("when creating labels for Perses components",
		func(persesImageFromFlag string, componentName string, perses *v1alpha2.Perses, verifyFunc func(labels map[string]string)) {
			labels := LabelsForPerses(componentName, perses)
			verifyFunc(labels)
		},
		Entry("Long name is trimmed to 63 characters",
			"",
			strings.Repeat("a", 100),
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("a", 100)},
				Spec:       v1alpha2.PersesSpec{Image: ptr.To("perses/perses:latest")},
			},
			func(labels map[string]string) {
				nameLabel, exists := labels["app.kubernetes.io/name"]
				Expect(exists).To(BeTrue())
				Expect(nameLabel).To(HaveLen(63))
				Expect(nameLabel).To(Equal(strings.Repeat("a", 63)))
			},
		),
		Entry("Custom labels from metadata are preserved",
			"",
			"perses-server",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-perses",
				},
				Spec: v1alpha2.PersesSpec{
					Image: ptr.To("perses/perses:latest"),
					Metadata: &v1alpha2.Metadata{
						Labels: map[string]string{
							"custom-label": "custom-value",
						},
					},
				},
			},
			func(labels map[string]string) {
				Expect(labels).To(HaveKeyWithValue("custom-label", "custom-value"))
			},
		),
	)
})

var _ = Describe("SelectorLabelsForPerses", func() {
	It("returns only the operator-set labels", func() {
		labels := SelectorLabelsForPerses("perses-server", &v1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
		})

		Expect(labels).To(Equal(map[string]string{
			"app.kubernetes.io/name":       "perses-server",
			"app.kubernetes.io/instance":   "test-perses",
			"app.kubernetes.io/part-of":    "perses-operator",
			"app.kubernetes.io/created-by": "controller-manager",
			"app.kubernetes.io/managed-by": "perses-operator",
		}))
	})

	It("equals the full label set when no user labels are given", func() {
		perses := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "test-perses"}}

		// Workloads created by older operator versions used the full label set
		// as selector; without user labels it must stay identical so they are
		// not recreated on upgrade.
		Expect(SelectorLabelsForPerses("perses-server", perses)).To(Equal(LabelsForPerses("perses-server", perses)))
	})

	It("never includes user-supplied metadata labels", func() {
		perses := &v1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
			Spec: v1alpha2.PersesSpec{
				Metadata: &v1alpha2.Metadata{
					Labels: map[string]string{
						"custom-label": "custom-value",
					},
				},
			},
		}

		selector := SelectorLabelsForPerses("perses-server", perses)
		full := LabelsForPerses("perses-server", perses)

		// User labels can change over the lifetime of the instance, so they
		// must not leak into the immutable selector.
		Expect(selector).NotTo(HaveKey("custom-label"))

		// The full label set carries the user labels and remains a superset of
		// the selector, so the pod template still matches the selector.
		Expect(full).To(HaveKeyWithValue("custom-label", "custom-value"))
		for k, v := range selector {
			Expect(full).To(HaveKeyWithValue(k, v))
		}
	})
})
