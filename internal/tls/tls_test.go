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

package tls

import (
	"crypto/tls"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTLS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TLS Suite")
}

var _ = Describe("ParseTLSVersion", func() {
	DescribeTable("parses TLS version strings",
		func(input string, expectedVersion uint16, expectErr bool) {
			version, err := ParseTLSVersion(input)
			if expectErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal(expectedVersion))
			}
		},
		Entry("empty string returns zero (default)", "", uint16(0), false),
		Entry("VersionTLS10", "VersionTLS10", uint16(tls.VersionTLS10), false),
		Entry("VersionTLS11", "VersionTLS11", uint16(tls.VersionTLS11), false),
		Entry("VersionTLS12", "VersionTLS12", uint16(tls.VersionTLS12), false),
		Entry("VersionTLS13", "VersionTLS13", uint16(tls.VersionTLS13), false),
		Entry("invalid version", "VersionTLS14", uint16(0), true),
		Entry("numeric version", "1.2", uint16(0), true),
	)
})

var _ = Describe("ParseCipherSuites", func() {
	DescribeTable("parses cipher suite strings",
		func(input string, expectedLen int, expectErr bool) {
			ids, err := ParseCipherSuites(input)
			if expectErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(expectedLen))
			}
		},
		Entry("empty string returns nil", "", 0, false),
		Entry("single cipher suite", "TLS_AES_128_GCM_SHA256", 1, false),
		Entry("multiple cipher suites", "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384", 2, false),
		Entry("with whitespace", "TLS_AES_128_GCM_SHA256 , TLS_AES_256_GCM_SHA384", 2, false),
		Entry("unknown cipher suite", "INVALID_CIPHER", 0, true),
		Entry("one valid one invalid", "TLS_AES_128_GCM_SHA256,INVALID", 0, true),
		Entry("trailing comma ignored", "TLS_AES_128_GCM_SHA256,", 1, false),
	)
})

var _ = Describe("ConfigureTLS", func() {
	It("disables HTTP/2 when enableHTTP2 is false", func() {
		cfg := &tls.Config{}
		fn := ConfigureTLS(0, nil, false)
		fn(cfg)
		Expect(cfg.NextProtos).To(Equal([]string{"http/1.1"}))
	})

	It("does not disable HTTP/2 when enableHTTP2 is true", func() {
		cfg := &tls.Config{}
		fn := ConfigureTLS(0, nil, true)
		fn(cfg)
		Expect(cfg.NextProtos).To(BeNil())
	})

	It("sets min version", func() {
		cfg := &tls.Config{}
		fn := ConfigureTLS(tls.VersionTLS13, nil, true)
		fn(cfg)
		Expect(cfg.MinVersion).To(Equal(uint16(tls.VersionTLS13)))
	})

	It("sets cipher suites", func() {
		suites := []uint16{tls.TLS_AES_128_GCM_SHA256}
		cfg := &tls.Config{}
		fn := ConfigureTLS(0, suites, true)
		fn(cfg)
		Expect(cfg.CipherSuites).To(Equal(suites))
	})

	It("applies all settings together", func() {
		suites := []uint16{tls.TLS_AES_128_GCM_SHA256}
		cfg := &tls.Config{}
		fn := ConfigureTLS(tls.VersionTLS12, suites, false)
		fn(cfg)
		Expect(cfg.NextProtos).To(Equal([]string{"http/1.1"}))
		Expect(cfg.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
		Expect(cfg.CipherSuites).To(Equal(suites))
	})
})
