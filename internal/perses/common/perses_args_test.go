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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const configArg = "--config=/etc/perses/config/config.yaml"

var _ = Describe("GetPersesArgs", func() {
	DescribeTable("when generating command line arguments",
		func(perses *v1alpha2.Perses, tlsMinVersion, tlsCipherSuites string, tlsConfigureOperands bool, expectedArgs []string) {
			args := GetPersesArgs(perses, tlsMinVersion, tlsCipherSuites, tlsConfigureOperands)
			Expect(args).To(Equal(expectedArgs))
		},
		Entry("Default args with no optional fields",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec:       v1alpha2.PersesSpec{},
			},
			"", "", false,
			[]string{configArg},
		),
		Entry("Args with LogLevel set to debug",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec: v1alpha2.PersesSpec{
					LogLevel: ptr.To("debug"),
				},
			},
			"", "", false,
			[]string{configArg, "--log.level=debug"},
		),
		Entry("Args with LogLevel set to trace",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec: v1alpha2.PersesSpec{
					LogLevel: ptr.To("trace"),
				},
			},
			"", "", false,
			[]string{configArg, "--log.level=trace"},
		),
		Entry("Args with LogMethodTrace enabled",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec: v1alpha2.PersesSpec{
					LogMethodTrace: ptr.To(true),
				},
			},
			"", "", false,
			[]string{configArg, "--log.method-trace"},
		),
		Entry("Args with LogMethodTrace disabled",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec: v1alpha2.PersesSpec{
					LogMethodTrace: ptr.To(false),
				},
			},
			"", "", false,
			[]string{configArg},
		),
		Entry("Args with both LogLevel and LogMethodTrace",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec: v1alpha2.PersesSpec{
					LogLevel:       ptr.To("debug"),
					LogMethodTrace: ptr.To(true),
				},
			},
			"", "", false,
			[]string{configArg, "--log.level=debug", "--log.method-trace"},
		),
		Entry("Args with user-provided extra args",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec: v1alpha2.PersesSpec{
					LogLevel: ptr.To("info"),
					Args:     []string{"--pprof"},
				},
			},
			"", "", false,
			[]string{configArg, "--log.level=info", "--pprof"},
		),
		Entry("Args with custom containerPort sets web.listen-address",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec: v1alpha2.PersesSpec{
					ContainerPort: ptr.To[int32](9000),
				},
			},
			"", "", false,
			[]string{configArg, "--web.listen-address=:9000"},
		),
		Entry("Args with TLS min version when configure operands is true",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec:       v1alpha2.PersesSpec{},
			},
			"VersionTLS13", "", true,
			[]string{configArg, "--web.tls-min-version=VersionTLS13"},
		),
		Entry("Args with TLS cipher suites when configure operands is true",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec:       v1alpha2.PersesSpec{},
			},
			"", "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384", true,
			[]string{configArg, "--web.tls-cipher-suites=TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"},
		),
		Entry("Args with both TLS min version and cipher suites when configure operands is true",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec:       v1alpha2.PersesSpec{},
			},
			"VersionTLS12", "TLS_AES_128_GCM_SHA256", true,
			[]string{configArg, "--web.tls-min-version=VersionTLS12", "--web.tls-cipher-suites=TLS_AES_128_GCM_SHA256"},
		),
		Entry("TLS values set but configure operands is false - no TLS args added",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test-perses"},
				Spec:       v1alpha2.PersesSpec{},
			},
			"VersionTLS12", "TLS_AES_128_GCM_SHA256", false,
			[]string{configArg},
		),
	)
})
