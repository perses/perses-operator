/*
Copyright 2026 The Perses Authors.

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
	"context"

	"github.com/perses/perses-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetTLSSecretReferences", func() {
	DescribeTable("when getting TLS secret references",
		func(perses *v1alpha2.Perses, expectedRefs []types.NamespacedName) {
			refs := GetTLSSecretReferences(perses)
			Expect(refs).To(Equal(expectedRefs))
		},
		Entry("Returns nil when TLS is not enabled",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses",
					Namespace: "default",
				},
				Spec: v1alpha2.PersesSpec{},
			},
			nil,
		),
		Entry("Returns CA cert reference when configured",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses",
					Namespace: "default",
				},
				Spec: v1alpha2.PersesSpec{
					TLS: &v1alpha2.TLS{
						Enable: true,
						CaCert: &v1alpha2.Certificate{
							SecretSource: v1alpha2.SecretSource{
								Type: v1alpha2.SecretSourceTypeSecret,
								Name: "ca-secret",
							},
							CertPath: "ca.crt",
						},
					},
				},
			},
			[]types.NamespacedName{
				{Namespace: "default", Name: "ca-secret"},
			},
		),
		Entry("Returns user cert reference when configured",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses",
					Namespace: "default",
				},
				Spec: v1alpha2.PersesSpec{
					TLS: &v1alpha2.TLS{
						Enable: true,
						UserCert: &v1alpha2.Certificate{
							SecretSource: v1alpha2.SecretSource{
								Type: v1alpha2.SecretSourceTypeSecret,
								Name: "tls-secret",
							},
							CertPath:       "tls.crt",
							PrivateKeyPath: "tls.key",
						},
					},
				},
			},
			[]types.NamespacedName{
				{Namespace: "default", Name: "tls-secret"},
			},
		),
		Entry("Returns both CA and user cert references",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses",
					Namespace: "default",
				},
				Spec: v1alpha2.PersesSpec{
					TLS: &v1alpha2.TLS{
						Enable: true,
						CaCert: &v1alpha2.Certificate{
							SecretSource: v1alpha2.SecretSource{
								Type: v1alpha2.SecretSourceTypeSecret,
								Name: "ca-secret",
							},
							CertPath: "ca.crt",
						},
						UserCert: &v1alpha2.Certificate{
							SecretSource: v1alpha2.SecretSource{
								Type: v1alpha2.SecretSourceTypeSecret,
								Name: "tls-secret",
							},
							CertPath:       "tls.crt",
							PrivateKeyPath: "tls.key",
						},
					},
				},
			},
			[]types.NamespacedName{
				{Namespace: "default", Name: "ca-secret"},
				{Namespace: "default", Name: "tls-secret"},
			},
		),
		Entry("Uses certificate namespace when specified",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses",
					Namespace: "default",
				},
				Spec: v1alpha2.PersesSpec{
					TLS: &v1alpha2.TLS{
						Enable: true,
						UserCert: &v1alpha2.Certificate{
							SecretSource: v1alpha2.SecretSource{
								Type:      v1alpha2.SecretSourceTypeSecret,
								Name:      "tls-secret",
								Namespace: "cert-manager",
							},
							CertPath:       "tls.crt",
							PrivateKeyPath: "tls.key",
						},
					},
				},
			},
			[]types.NamespacedName{
				{Namespace: "cert-manager", Name: "tls-secret"},
			},
		),
		Entry("Returns nil when TLS uses ConfigMap type (not Secret)",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses",
					Namespace: "default",
				},
				Spec: v1alpha2.PersesSpec{
					TLS: &v1alpha2.TLS{
						Enable: true,
						UserCert: &v1alpha2.Certificate{
							SecretSource: v1alpha2.SecretSource{
								Type: v1alpha2.SecretSourceTypeConfigMap,
								Name: "tls-configmap",
							},
							CertPath:       "tls.crt",
							PrivateKeyPath: "tls.key",
						},
					},
				},
			},
			nil,
		),
		Entry("Only returns Secret refs, ignores ConfigMap refs in mixed config",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-perses",
					Namespace: "default",
				},
				Spec: v1alpha2.PersesSpec{
					TLS: &v1alpha2.TLS{
						Enable: true,
						CaCert: &v1alpha2.Certificate{
							SecretSource: v1alpha2.SecretSource{
								Type: v1alpha2.SecretSourceTypeConfigMap,
								Name: "ca-configmap",
							},
							CertPath: "ca.crt",
						},
						UserCert: &v1alpha2.Certificate{
							SecretSource: v1alpha2.SecretSource{
								Type: v1alpha2.SecretSourceTypeSecret,
								Name: "tls-secret",
							},
							CertPath:       "tls.crt",
							PrivateKeyPath: "tls.key",
						},
					},
				},
			},
			[]types.NamespacedName{
				{Namespace: "default", Name: "tls-secret"},
			},
		),
	)
})

var _ = Describe("ComputeTLSCertificateChecksum", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha2.AddToScheme(scheme)).To(Succeed())
	})

	It("returns empty string when TLS is not enabled", func() {
		perses := &v1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-perses",
				Namespace: "default",
			},
			Spec: v1alpha2.PersesSpec{},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		checksum, err := ComputeTLSCertificateChecksum(ctx, fakeClient, perses)

		Expect(err).ToNot(HaveOccurred())
		Expect(checksum).To(BeEmpty())
	})

	// This test verifies the exact checksum value to catch regressions in the hashing implementation.
	// Changes to hash algorithm, concatenation order, sorting, or encoding will cause this test to fail,
	// preventing silent behavior changes that would cause all pods to restart unnecessarily.
	It("computes expected checksum for known input", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tls-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"tls.crt": []byte("test-cert"),
				"tls.key": []byte("test-key"),
			},
		}

		perses := &v1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-perses",
				Namespace: "default",
			},
			Spec: v1alpha2.PersesSpec{
				TLS: &v1alpha2.TLS{
					Enable: true,
					UserCert: &v1alpha2.Certificate{
						SecretSource: v1alpha2.SecretSource{
							Type: v1alpha2.SecretSourceTypeSecret,
							Name: "tls-secret",
						},
						CertPath:       "tls.crt",
						PrivateKeyPath: "tls.key",
					},
				},
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
		checksum, err := ComputeTLSCertificateChecksum(ctx, fakeClient, perses)

		Expect(err).ToNot(HaveOccurred())
		expectedChecksum := "90f8aed7780cec27b6e9f73d9d45ddc5809004d2a2b8f0cc8e423cba7be38d82"
		Expect(checksum).To(Equal(expectedChecksum))
	})

	It("returns error when secret is not found", func() {
		perses := &v1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-perses",
				Namespace: "default",
			},
			Spec: v1alpha2.PersesSpec{
				TLS: &v1alpha2.TLS{
					Enable: true,
					UserCert: &v1alpha2.Certificate{
						SecretSource: v1alpha2.SecretSource{
							Type: v1alpha2.SecretSourceTypeSecret,
							Name: "non-existent-secret",
						},
						CertPath:       "tls.crt",
						PrivateKeyPath: "tls.key",
					},
				},
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		_, err := ComputeTLSCertificateChecksum(ctx, fakeClient, perses)

		Expect(err).To(HaveOccurred())
	})

	It("includes both CA and user cert in checksum", func() {
		caSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ca-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"ca.crt": []byte("ca-cert-data"),
			},
		}

		tlsSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tls-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"tls.crt": []byte("cert-data"),
				"tls.key": []byte("key-data"),
			},
		}

		persesWithBoth := &v1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-perses",
				Namespace: "default",
			},
			Spec: v1alpha2.PersesSpec{
				TLS: &v1alpha2.TLS{
					Enable: true,
					CaCert: &v1alpha2.Certificate{
						SecretSource: v1alpha2.SecretSource{
							Type: v1alpha2.SecretSourceTypeSecret,
							Name: "ca-secret",
						},
						CertPath: "ca.crt",
					},
					UserCert: &v1alpha2.Certificate{
						SecretSource: v1alpha2.SecretSource{
							Type: v1alpha2.SecretSourceTypeSecret,
							Name: "tls-secret",
						},
						CertPath:       "tls.crt",
						PrivateKeyPath: "tls.key",
					},
				},
			},
		}

		persesUserOnly := &v1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-perses-user-only",
				Namespace: "default",
			},
			Spec: v1alpha2.PersesSpec{
				TLS: &v1alpha2.TLS{
					Enable: true,
					UserCert: &v1alpha2.Certificate{
						SecretSource: v1alpha2.SecretSource{
							Type: v1alpha2.SecretSourceTypeSecret,
							Name: "tls-secret",
						},
						CertPath:       "tls.crt",
						PrivateKeyPath: "tls.key",
					},
				},
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(caSecret, tlsSecret).Build()

		checksumBoth, err := ComputeTLSCertificateChecksum(ctx, fakeClient, persesWithBoth)
		Expect(err).ToNot(HaveOccurred())

		checksumUserOnly, err := ComputeTLSCertificateChecksum(ctx, fakeClient, persesUserOnly)
		Expect(err).ToNot(HaveOccurred())

		// Checksums should be different because one includes CA cert
		Expect(checksumBoth).ToNot(Equal(checksumUserOnly))
	})
})
