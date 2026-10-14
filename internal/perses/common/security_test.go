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
	"github.com/rhobs/perses-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetPodSecurityContext", func() {
	DescribeTable("when getting pod security context",
		func(perses *v1alpha2.Perses, expected *corev1.PodSecurityContext) {
			result := GetPodSecurityContext(perses)
			Expect(result).To(Equal(expected))
		},
		Entry("returns default when no custom context is set",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec:       v1alpha2.PersesSpec{},
			},
			&corev1.PodSecurityContext{
				FSGroup: Int64Ptr(65534),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
		),
		Entry("returns custom context when specified",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					PodSecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  Int64Ptr(1000),
						RunAsGroup: Int64Ptr(1000),
					},
				},
			},
			&corev1.PodSecurityContext{
				RunAsUser:  Int64Ptr(1000),
				RunAsGroup: Int64Ptr(1000),
			},
		),
	)
})

var _ = Describe("GetContainerSecurityContext", func() {
	DescribeTable("when getting container security context",
		func(perses *v1alpha2.Perses, expected *corev1.SecurityContext) {
			result := GetContainerSecurityContext(perses)
			Expect(result).To(Equal(expected))
		},
		Entry("returns default when no custom context is set",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec:       v1alpha2.PersesSpec{},
			},
			&corev1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
		),
		Entry("returns custom context when specified",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					ContainerSecurityContext: &corev1.SecurityContext{
						RunAsUser:                Int64Ptr(1000),
						ReadOnlyRootFilesystem:   ptr.To(true),
						AllowPrivilegeEscalation: ptr.To(false),
					},
				},
			},
			&corev1.SecurityContext{
				RunAsUser:                Int64Ptr(1000),
				ReadOnlyRootFilesystem:   ptr.To(true),
				AllowPrivilegeEscalation: ptr.To(false),
			},
		),
		Entry("returns custom context with runAsNonRoot",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					ContainerSecurityContext: &corev1.SecurityContext{
						RunAsNonRoot: ptr.To(true),
					},
				},
			},
			&corev1.SecurityContext{
				RunAsNonRoot: ptr.To(true),
			},
		),
	)
})
