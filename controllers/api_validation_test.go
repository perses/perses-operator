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

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	persescommon "github.com/perses/perses/pkg/model/api/v1/common"
	persesdashboard "github.com/perses/perses/pkg/model/api/v1/dashboard"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
)

var _ = Describe("API Validation", func() {
	Context("BasicAuth validation", func() {
		ctx := context.Background()

		It("should reject BasicAuth with empty username", func() {
			By("Creating a Perses resource with empty BasicAuth username")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-basicauth-no-user",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To("my-secret"),
								Namespace: ptr.To("default"),
							},
							Username:     "", // Invalid: required
							PasswordPath: "/path/to/password",
						},
					},
				},
			}

			By("Expecting the creation to fail with validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should reject BasicAuth with empty passwordPath", func() {
			By("Creating a Perses resource with empty BasicAuth passwordPath")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-basicauth-no-password",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To("my-secret"),
								Namespace: ptr.To("default"),
							},
							Username:     "admin",
							PasswordPath: "", // Invalid: required
						},
					},
				},
			}

			By("Expecting the creation to fail with validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should accept BasicAuth with valid configuration", func() {
			By("Creating a Perses resource with valid BasicAuth configuration")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-basicauth",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To("my-secret"),
								Namespace: ptr.To("default"),
							},
							Username:     "admin",
							PasswordPath: "password",
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("OAuth validation", func() {
		ctx := context.Background()

		It("should reject OAuth with empty tokenURL", func() {
			By("Creating a Perses resource with empty OAuth tokenURL")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-oauth-no-url",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						OAuth: &persesv1alpha2.OAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To("oauth-secret"),
								Namespace: ptr.To("default"),
							},
							TokenURL: "", // Invalid: required
						},
					},
				},
			}

			By("Expecting the creation to fail with validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should accept OAuth with valid tokenURL", func() {
			By("Creating a Perses resource with valid OAuth tokenURL")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-oauth",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						OAuth: &persesv1alpha2.OAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To("oauth-secret"),
								Namespace: ptr.To("default"),
							},
							TokenURL: "https://auth.example.com/token",
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("TLS Certificate validation", func() {
		ctx := context.Background()

		It("should reject Certificate with empty certPath", func() {
			By("Creating a Perses resource with empty Certificate certPath")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-cert-no-path",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						TLS: &persesv1alpha2.TLS{
							Enable: ptr.To(true),
							CaCert: &persesv1alpha2.Certificate{
								SecretSource: persesv1alpha2.SecretSource{
									Type:      persesv1alpha2.SecretSourceTypeSecret,
									Name:      ptr.To("tls-secret"),
									Namespace: ptr.To("default"),
								},
								CertPath: "", // Invalid: required
							},
						},
					},
				},
			}

			By("Expecting the creation to fail with validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should accept Certificate with valid certPath", func() {
			By("Creating a Perses resource with valid Certificate certPath")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-cert",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						TLS: &persesv1alpha2.TLS{
							Enable: ptr.To(true),
							CaCert: &persesv1alpha2.Certificate{
								SecretSource: persesv1alpha2.SecretSource{
									Type:      persesv1alpha2.SecretSourceTypeSecret,
									Name:      ptr.To("tls-secret"),
									Namespace: ptr.To("default"),
								},
								CertPath: "ca.crt",
							},
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})

		It("should reject Certificate with type=secret but missing name (CEL validation)", func() {
			By("Creating a Perses resource with Certificate type=secret but no name")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-cert-no-name",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						TLS: &persesv1alpha2.TLS{
							Enable: ptr.To(true),
							UserCert: &persesv1alpha2.Certificate{
								SecretSource: persesv1alpha2.SecretSource{
									Type:      persesv1alpha2.SecretSourceTypeSecret,
									Name:      nil, // Invalid: required when type is secret
									Namespace: ptr.To("default"),
								},
								CertPath:       "tls.crt",
								PrivateKeyPath: ptr.To("tls.key"),
							},
						},
					},
				},
			}

			By("Expecting the creation to fail with CEL validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("name is required when type is secret or configmap"))
		})

		It("should accept Certificate with type=file without name or namespace (CEL validation)", func() {
			By("Creating a Perses resource with Certificate type=file")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-cert-file",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						TLS: &persesv1alpha2.TLS{
							Enable: ptr.To(true),
							CaCert: &persesv1alpha2.Certificate{
								SecretSource: persesv1alpha2.SecretSource{
									Type:      persesv1alpha2.SecretSourceTypeFile,
									Name:      nil, // Valid: not required when type is file
									Namespace: nil, // Valid: not required when type is file
								},
								CertPath: "/etc/ssl/certs/ca.crt",
							},
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("SecretSource validation", func() {
		ctx := context.Background()

		It("should reject SecretSource with invalid type", func() {
			By("Creating a Perses resource with invalid SecretSource type")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-secretsource-type",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type: "invalid-type", // Invalid: must be secret, configmap, or file
								Name: ptr.To("my-secret"),
							},
							Username:     "admin",
							PasswordPath: "password",
						},
					},
				},
			}

			By("Expecting the creation to fail with validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should reject SecretSource with type=secret but missing name (CEL validation)", func() {
			By("Creating a Perses resource with type=secret but no name")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-secret-no-name",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      nil, // Invalid: required when type is secret
								Namespace: ptr.To("default"),
							},
							Username:     "admin",
							PasswordPath: "password",
						},
					},
				},
			}

			By("Expecting the creation to fail with CEL validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("name is required when type is secret or configmap"))
		})

		It("should reject SecretSource with type=secret but missing namespace (CEL validation)", func() {
			By("Creating a Perses resource with type=secret but no namespace")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-secret-no-namespace",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To("my-secret"),
								Namespace: nil, // Invalid: required when type is secret
							},
							Username:     "admin",
							PasswordPath: "password",
						},
					},
				},
			}

			By("Expecting the creation to fail with CEL validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("namespace is required when type is secret or configmap"))
		})

		It("should reject SecretSource with type=configmap but missing name (CEL validation)", func() {
			By("Creating a Perses resource with type=configmap but no name")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-configmap-no-name",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						OAuth: &persesv1alpha2.OAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeConfigMap,
								Name:      nil, // Invalid: required when type is configmap
								Namespace: ptr.To("default"),
							},
							TokenURL: "https://auth.example.com/token",
						},
					},
				},
			}

			By("Expecting the creation to fail with CEL validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("name is required when type is secret or configmap"))
		})

		It("should reject SecretSource with type=configmap but missing namespace (CEL validation)", func() {
			By("Creating a Perses resource with type=configmap but no namespace")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-configmap-no-namespace",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						OAuth: &persesv1alpha2.OAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeConfigMap,
								Name:      ptr.To("oauth-config"),
								Namespace: nil, // Invalid: required when type is configmap
							},
							TokenURL: "https://auth.example.com/token",
						},
					},
				},
			}

			By("Expecting the creation to fail with CEL validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("namespace is required when type is secret or configmap"))
		})

		It("should accept SecretSource with type=file without name or namespace (CEL validation)", func() {
			By("Creating a Perses resource with type=file and no name/namespace")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-file-source",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeFile,
								Name:      nil, // Valid: not required when type is file
								Namespace: nil, // Valid: not required when type is file
							},
							Username:     "admin",
							PasswordPath: "/var/secrets/password",
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})

		It("should accept SecretSource with type=secret and both name and namespace provided (CEL validation)", func() {
			By("Creating a Perses resource with type=secret and valid name/namespace")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-secret-source",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To("my-secret"),
								Namespace: ptr.To("default"),
							},
							Username:     "admin",
							PasswordPath: "password",
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})

		It("should accept SecretSource with type=configmap and both name and namespace provided (CEL validation)", func() {
			By("Creating a Perses resource with type=configmap and valid name/namespace")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-configmap-source",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						OAuth: &persesv1alpha2.OAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeConfigMap,
								Name:      ptr.To("oauth-config"),
								Namespace: ptr.To("default"),
							},
							TokenURL: "https://auth.example.com/token",
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})

		It("should reject SecretSource with type=secret and empty name string (MinLength validation)", func() {
			By("Creating a Perses resource with type=secret and empty name string")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-secret-empty-name",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To(""), // Invalid: MinLength=1
								Namespace: ptr.To("default"),
							},
							Username:     "admin",
							PasswordPath: "password",
						},
					},
				},
			}

			By("Expecting the creation to fail with MinLength validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should reject SecretSource with type=secret and empty namespace string (MinLength validation)", func() {
			By("Creating a Perses resource with type=secret and empty namespace string")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-secret-empty-namespace",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeSecret,
								Name:      ptr.To("my-secret"),
								Namespace: ptr.To(""), // Invalid: MinLength=1
							},
							Username:     "admin",
							PasswordPath: "password",
						},
					},
				},
			}

			By("Expecting the creation to fail with MinLength validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should reject SecretSource with type=configmap and empty name string (MinLength validation)", func() {
			By("Creating a Perses resource with type=configmap and empty name string")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-configmap-empty-name",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						OAuth: &persesv1alpha2.OAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeConfigMap,
								Name:      ptr.To(""), // Invalid: MinLength=1
								Namespace: ptr.To("default"),
							},
							TokenURL: "https://auth.example.com/token",
						},
					},
				},
			}

			By("Expecting the creation to fail with MinLength validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should reject SecretSource with type=configmap and empty namespace string (MinLength validation)", func() {
			By("Creating a Perses resource with type=configmap and empty namespace string")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-configmap-empty-namespace",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						OAuth: &persesv1alpha2.OAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeConfigMap,
								Name:      ptr.To("oauth-config"),
								Namespace: ptr.To(""), // Invalid: MinLength=1
							},
							TokenURL: "https://auth.example.com/token",
						},
					},
				},
			}

			By("Expecting the creation to fail with MinLength validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should accept SecretSource with type=file even with name and namespace provided (CEL validation)", func() {
			By("Creating a Perses resource with type=file and name/namespace (optional and ignored)")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-file-with-optional-fields",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						BasicAuth: &persesv1alpha2.BasicAuth{
							SecretSource: persesv1alpha2.SecretSource{
								Type:      persesv1alpha2.SecretSourceTypeFile,
								Name:      ptr.To("ignored-name"),      // Optional: ignored when type is file
								Namespace: ptr.To("ignored-namespace"), // Optional: ignored when type is file
							},
							Username:     "admin",
							PasswordPath: "/var/secrets/password",
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("ContainerPort validation", func() {
		ctx := context.Background()

		It("should reject ContainerPort exceeding 65535", func() {
			By("Creating a Perses resource with ContainerPort 65536")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-port-too-high",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(65536)), // Invalid: maximum is 65535
				},
			}

			By("Expecting the creation to fail with validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should accept ContainerPort with valid value", func() {
			By("Creating a Perses resource with valid ContainerPort 8500")
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-port",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8500)),
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("LogLevel validation", func() {
		ctx := context.Background()

		It("should reject LogLevel with invalid value", func() {
			By("Creating a Perses resource with invalid LogLevel")
			invalidLogLevel := "invalid"
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-loglevel",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					LogLevel:      &invalidLogLevel,
				},
			}

			By("Expecting the creation to fail with validation error")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("should accept LogLevel with valid value", func() {
			By("Creating a Perses resource with valid LogLevel debug")
			debugLogLevel := "debug"
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-loglevel",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					LogLevel:      &debugLogLevel,
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("LogMethodTrace validation", func() {
		ctx := context.Background()

		It("should accept LogMethodTrace with true value", func() {
			By("Creating a Perses resource with LogMethodTrace enabled")
			logMethodTrace := true
			perses := &persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-logmethodtrace",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort:  ptr.To(int32(8080)),
					LogMethodTrace: &logMethodTrace,
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, perses)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, perses)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})

var _ = Describe("PersesDashboard API Validation", func() {
	Context("PersesDashboard config validation", func() {
		ctx := context.Background()

		It("should accept PersesDashboard with valid config", func() {
			By("Creating a PersesDashboard resource with valid config")
			dashboard := &persesv1alpha2.PersesDashboard{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-dashboard",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.PersesDashboardSpec{
					Config: persesv1alpha2.Dashboard{
						DashboardSpec: persesv1.DashboardSpec{
							Display: &persescommon.Display{
								Name: "valid-dashboard",
							},
							Layouts: []persesdashboard.Layout{},
							Panels:  map[string]*persesv1.Panel{},
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, dashboard)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, dashboard)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})

var _ = Describe("PersesDatasource API Validation", func() {
	Context("PersesDatasource config validation", func() {
		ctx := context.Background()

		It("should accept PersesDatasource with valid config", func() {
			By("Creating a PersesDatasource resource with valid config")
			datasource := &persesv1alpha2.PersesDatasource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-datasource",
					Namespace: persesNamespace,
				},
				Spec: persesv1alpha2.DatasourceSpec{
					Config: persesv1alpha2.Datasource{
						DatasourceSpec: persesv1.DatasourceSpec{
							Display: &persescommon.Display{
								Name: "valid-datasource",
							},
							Plugin: persescommon.Plugin{
								Kind: "PrometheusDatasource",
								Spec: map[string]interface{}{},
							},
						},
					},
				},
			}

			By("Expecting the creation to succeed")
			err := k8sClient.Create(ctx, datasource)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleaning up the created resource")
			Eventually(func() error {
				return k8sClient.Delete(ctx, datasource)
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
