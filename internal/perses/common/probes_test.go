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

package common

import (
	"github.com/perses/perses-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetProbes", func() {
	DescribeTable("when getting probes for a Perses instance",
		func(perses *v1alpha2.Perses, expectedLiveness, expectedReadiness *int32) {
			liveness, readiness := GetProbes(perses)
			if expectedLiveness == nil {
				Expect(liveness).To(BeNil())
			} else {
				Expect(liveness).NotTo(BeNil())
				Expect(liveness.HTTPGet.Port.IntVal).To(Equal(*expectedLiveness))
			}
			if expectedReadiness == nil {
				Expect(readiness).To(BeNil())
			} else {
				Expect(readiness).NotTo(BeNil())
				Expect(readiness.HTTPGet.Port.IntVal).To(Equal(*expectedReadiness))
			}
		},
		Entry("returns nil probes when none are configured",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec:       v1alpha2.PersesSpec{},
			},
			nil, nil,
		),
		Entry("uses DefaultContainerPort when containerPort is not set",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					LivenessProbe:  &corev1.Probe{},
					ReadinessProbe: &corev1.Probe{},
				},
			},
			ptr.To(DefaultContainerPort), ptr.To(DefaultContainerPort),
		),
		Entry("uses custom containerPort when set",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					ContainerPort:  ptr.To[int32](9000),
					LivenessProbe:  &corev1.Probe{},
					ReadinessProbe: &corev1.Probe{},
				},
			},
			ptr.To[int32](9000), ptr.To[int32](9000),
		),
		Entry("only liveness probe configured uses correct port",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					ContainerPort: ptr.To[int32](9000),
					LivenessProbe: &corev1.Probe{},
				},
			},
			ptr.To[int32](9000), nil,
		),
	)
})
