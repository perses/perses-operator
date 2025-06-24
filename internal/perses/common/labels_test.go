/*
Copyright 2023 The Perses Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"strings"
	"testing"

	"github.com/perses/perses-operator/api/v1alpha2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"
)

func TestLabels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Labels Suite")
}

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
				Spec:       v1alpha2.PersesSpec{Image: "perses/perses:latest"},
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
					Image: "perses/perses:latest",
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
