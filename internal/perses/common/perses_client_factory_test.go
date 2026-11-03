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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

func TestConfigFingerprint(t *testing.T) {
	base := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "perses-1",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: persesv1alpha2.PersesSpec{
			ContainerPort: ptr.To(int32(8080)),
		},
	}

	fp1 := configFingerprint(base)

	// Same config produces same fingerprint
	fp2 := configFingerprint(base)
	assert.Equal(t, fp1, fp2, "Same config should produce same fingerprint")

	// Different Generation changes fingerprint
	modified := base.DeepCopy()
	modified.Generation = 2
	fp3 := configFingerprint(*modified)
	assert.NotEqual(t, fp1, fp3, "Different Generation should change fingerprint")

	// Different port changes fingerprint
	modified = base.DeepCopy()
	modified.Spec.ContainerPort = ptr.To(int32(9090))
	fp4 := configFingerprint(*modified)
	assert.NotEqual(t, fp1, fp4, "Different port should change fingerprint")

	// Different namespace changes fingerprint
	modified = base.DeepCopy()
	modified.Namespace = "production"
	fp5 := configFingerprint(*modified)
	assert.NotEqual(t, fp1, fp5, "Different namespace should change fingerprint")

	// Enabling client TLS changes fingerprint
	modified = base.DeepCopy()
	modified.Spec.Client = &persesv1alpha2.Client{
		TLS: &persesv1alpha2.TLS{
			Enable: ptr.To(true),
		},
	}
	fp6 := configFingerprint(*modified)
	assert.NotEqual(t, fp1, fp6, "Enabling client TLS should change fingerprint")

	// Same ResourceVersion but different Generation produces different fingerprint
	modified = base.DeepCopy()
	modified.ResourceVersion = "999"
	fpSameGen := configFingerprint(*modified)
	assert.Equal(t, fp1, fpSameGen, "ResourceVersion changes should not affect fingerprint")
}

func TestForgetInstance(t *testing.T) {
	factory := NewWithConfig(AllowedSecretFileDirs{})

	// Manually populate cache
	factory.cache["default/perses-1"] = clientCacheEntry{
		client:      nil,
		fingerprint: "test-fp",
	}
	factory.cache["prod/perses-2"] = clientCacheEntry{
		client:      nil,
		fingerprint: "test-fp-2",
	}

	assert.Len(t, factory.cache, 2)

	factory.ForgetInstance("default/perses-1")
	assert.Len(t, factory.cache, 1)

	_, exists := factory.cache["default/perses-1"]
	assert.False(t, exists, "Entry should be removed")

	_, exists = factory.cache["prod/perses-2"]
	assert.True(t, exists, "Other entry should remain")

	// Forgetting a non-existent key should not panic
	factory.ForgetInstance("nonexistent/key")
	assert.Len(t, factory.cache, 1)
}

func TestCacheHitTTLExpiry(t *testing.T) {
	factory := NewWithConfig(AllowedSecretFileDirs{})
	factory.ttl = 100 * time.Millisecond

	factory.cache["default/perses-1"] = clientCacheEntry{
		client:      nil,
		fingerprint: "fp-1",
		createdAt:   time.Now(),
	}

	// Fresh entry should be a hit
	cached, ok := factory.cacheHit("default/perses-1", "fp-1")
	assert.True(t, ok, "Fresh entry should be a cache hit")
	assert.Nil(t, cached)

	// Wrong fingerprint should miss
	_, ok = factory.cacheHit("default/perses-1", "fp-different")
	assert.False(t, ok, "Wrong fingerprint should be a cache miss")

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)
	_, ok = factory.cacheHit("default/perses-1", "fp-1")
	assert.False(t, ok, "Expired entry should be a cache miss")
}

func TestSweepExpiredEntries(t *testing.T) {
	factory := NewWithConfig(AllowedSecretFileDirs{})
	factory.ttl = 50 * time.Millisecond

	factory.cache["default/perses-1"] = clientCacheEntry{
		client:      nil,
		fingerprint: "fp-1",
		createdAt:   time.Now(),
	}
	factory.cache["default/perses-2"] = clientCacheEntry{
		client:      nil,
		fingerprint: "fp-2",
		createdAt:   time.Now(),
	}

	assert.Len(t, factory.cache, 2)

	// Wait for entries to expire
	time.Sleep(100 * time.Millisecond)

	// Add a fresh entry that should survive the sweep
	factory.cache["default/perses-3"] = clientCacheEntry{
		client:      nil,
		fingerprint: "fp-3",
		createdAt:   time.Now(),
	}

	factory.mtx.Lock()
	factory.sweepExpiredLocked()
	factory.mtx.Unlock()

	assert.Len(t, factory.cache, 1, "Only the fresh entry should remain")
	_, exists := factory.cache["default/perses-3"]
	assert.True(t, exists, "Fresh entry should survive sweep")
}

func TestSweepSkipsBeforeInterval(t *testing.T) {
	factory := NewWithConfig(AllowedSecretFileDirs{})
	factory.ttl = 50 * time.Millisecond
	factory.lastSweep = time.Now()

	// Add an expired entry
	factory.cache["default/perses-1"] = clientCacheEntry{
		client:      nil,
		fingerprint: "fp-1",
		createdAt:   time.Now().Add(-time.Minute),
	}

	// Sweep should be skipped because lastSweep is recent
	factory.mtx.Lock()
	factory.sweepExpiredLocked()
	factory.mtx.Unlock()

	assert.Len(t, factory.cache, 1, "Expired entry should still exist because sweep was skipped")
}

func TestClientCacheConcurrency(t *testing.T) {
	factory := NewWithConfig(AllowedSecretFileDirs{})

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "default/perses-1"
			factory.mtx.Lock()
			factory.cache[key] = clientCacheEntry{
				client:      nil,
				fingerprint: "fp",
				createdAt:   time.Now(),
			}
			factory.mtx.Unlock()
			factory.ForgetInstance(key)
		}(i)
	}
	wg.Wait()

	assert.NotNil(t, factory.cache)
}

func TestConfigFingerprintOAuth(t *testing.T) {
	base := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "perses-1", Namespace: "default", Generation: 1},
		Spec:       persesv1alpha2.PersesSpec{},
	}

	fpNoOAuth := configFingerprint(base)

	withOAuth := base.DeepCopy()
	withOAuth.Spec.Client = &persesv1alpha2.Client{
		OAuth: &persesv1alpha2.OAuth{
			SecretSource: persesv1alpha2.SecretSource{
				Type: persesv1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("perses-config"),
			},
			ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
			ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
			TokenURL:         "http://perses.perses-system.svc:8080/api/auth/providers/oidc/oidc-provider/token",
		},
	}
	fpWithOAuth := configFingerprint(*withOAuth)
	assert.NotEqual(t, fpNoOAuth, fpWithOAuth, "Adding OAuth should change fingerprint")
	assert.Equal(t, fpWithOAuth, configFingerprint(*withOAuth), "Same OAuth config should produce same fingerprint")

	differentToken := withOAuth.DeepCopy()
	differentToken.Spec.Client.OAuth.TokenURL = "http://perses.perses-system.svc:8080/api/auth/providers/oidc/other/token"
	assert.NotEqual(t, fpWithOAuth, configFingerprint(*differentToken), "Different tokenURL should change fingerprint")

	differentIDPath := withOAuth.DeepCopy()
	differentIDPath.Spec.Client.OAuth.ClientIDPath = ptr.To("DIFFERENT_ID")
	assert.NotEqual(t, fpWithOAuth, configFingerprint(*differentIDPath), "Different clientIDPath should change fingerprint")

	withClientID := withOAuth.DeepCopy()
	withClientID.Spec.Client.OAuth.ClientIDPath = nil
	withClientID.Spec.Client.OAuth.ClientID = ptr.To("perses-operator")
	differentClientID := withClientID.DeepCopy()
	differentClientID.Spec.Client.OAuth.ClientID = ptr.To("other-client")
	assert.NotEqual(t, configFingerprint(*withClientID), configFingerprint(*differentClientID), "Different clientID should change fingerprint")
}

func TestBuildClientWithOAuth(t *testing.T) {
	ctx := context.Background()

	k8sSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "perses-config", Namespace: "perses-system"},
		Data: map[string][]byte{
			"OPERATOR_CLIENT_ID":     []byte("perses-operator"),
			"OPERATOR_CLIENT_SECRET": []byte("super-secret"),
		},
	}
	reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(k8sSecret).Build()

	perses := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
		Spec: persesv1alpha2.PersesSpec{
			ContainerPort: ptr.To(int32(8080)),
			Client: &persesv1alpha2.Client{
				OAuth: &persesv1alpha2.OAuth{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeSecret,
						Name:      ptr.To("perses-config"),
						Namespace: ptr.To("perses-system"),
					},
					ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
					ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
					TokenURL:         "http://perses.perses-system.svc.cluster.local:8080/api/auth/providers/oidc/oidc-provider/token",
				},
			},
		},
	}

	factory := NewWithConfig(AllowedSecretFileDirs{})
	client, err := factory.buildClient(ctx, reader, perses)
	require.NoError(t, err)
	require.NotNil(t, client)

	// NewRESTClient wires OAuth into an http client that fetches/refreshes the
	// Perses-issued token lazily, so the REST client and its http client exist.
	rest := client.RESTClient()
	require.NotNil(t, rest)
	assert.NotNil(t, rest.Client)
}

func TestBuildClientOAuthMissingSecret(t *testing.T) {
	ctx := context.Background()
	reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

	perses := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
		Spec: persesv1alpha2.PersesSpec{
			ContainerPort: ptr.To(int32(8080)),
			Client: &persesv1alpha2.Client{
				OAuth: &persesv1alpha2.OAuth{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeSecret,
						Name:      ptr.To("does-not-exist"),
						Namespace: ptr.To("perses-system"),
					},
					ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
					ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
					TokenURL:         "http://perses.perses-system.svc:8080/api/auth/providers/oidc/oidc-provider/token",
					EndpointParams: map[string][]string{
						"something": {"cool"},
					},
				},
			},
		},
	}

	factory := NewWithConfig(AllowedSecretFileDirs{})
	_, err := factory.buildClient(ctx, reader, perses)
	require.Error(t, err)
}

func TestBuildClientErrorPaths(t *testing.T) {
	ctx := context.Background()

	// Helper to create a Perses instance with OAuth configured.
	persesWithOAuth := func(oauth *persesv1alpha2.OAuth) persesv1alpha2.Perses {
		return persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "perses",
				Namespace:  "perses-system",
				Generation: 1,
			},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client: &persesv1alpha2.Client{
					OAuth: oauth,
				},
			},
		}
	}

	// Helper to create a Perses with client TLS enabled.
	persesWithClientTLS := func(tls *persesv1alpha2.TLS) persesv1alpha2.Perses {
		return persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "perses",
				Namespace:  "perses-system",
				Generation: 1,
			},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client: &persesv1alpha2.Client{
					TLS: tls,
				},
			},
		}
	}

	tests := []struct {
		name      string
		objects   []runtime.Object
		perses    persesv1alpha2.Perses
		errSubstr string
	}{
		// --- OAuth error paths ---
		{
			name:    "OAuth missing subject name",
			objects: nil,
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type: persesv1alpha2.SecretSourceTypeSecret,
				},
				ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
				ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "no name found for oauth",
		},
		{
			name:    "OAuth secret not found",
			objects: nil,
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type:      persesv1alpha2.SecretSourceTypeSecret,
					Name:      ptr.To("does-not-exist"),
					Namespace: ptr.To("perses-system"),
				},
				ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
				ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "not found",
		},
		{
			name: "OAuth missing client ID key in secret",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "oauth-secret", Namespace: "perses-system"},
					Data:       map[string][]byte{"other-key": []byte("value")},
				},
			},
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type:      persesv1alpha2.SecretSourceTypeSecret,
					Name:      ptr.To("oauth-secret"),
					Namespace: ptr.To("perses-system"),
				},
				ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
				ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "no client id data found",
		},
		{
			name: "OAuth missing client secret key in secret",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "oauth-secret", Namespace: "perses-system"},
					Data: map[string][]byte{
						"OPERATOR_CLIENT_ID": []byte("perses-operator"),
					},
				},
			},
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type:      persesv1alpha2.SecretSourceTypeSecret,
					Name:      ptr.To("oauth-secret"),
					Namespace: ptr.To("perses-system"),
				},
				ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
				ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "no client secret data found",
		},
		{
			name: "OAuth nil ClientIDPath",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "oauth-secret", Namespace: "perses-system"},
					Data: map[string][]byte{
						"OPERATOR_CLIENT_SECRET": []byte("super-secret"),
					},
				},
			},
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type:      persesv1alpha2.SecretSourceTypeSecret,
					Name:      ptr.To("oauth-secret"),
					Namespace: ptr.To("perses-system"),
				},
				ClientIDPath:     nil,
				ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "no client id data found",
		},
		{
			name: "OAuth nil ClientSecretPath",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "oauth-secret", Namespace: "perses-system"},
					Data: map[string][]byte{
						"OPERATOR_CLIENT_ID": []byte("perses-operator"),
					},
				},
			},
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type:      persesv1alpha2.SecretSourceTypeSecret,
					Name:      ptr.To("oauth-secret"),
					Namespace: ptr.To("perses-system"),
				},
				ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
				ClientSecretPath: nil,
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "no client secret data found",
		},
		{
			name:    "OAuth ConfigMap missing subject name",
			objects: nil,
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type: persesv1alpha2.SecretSourceTypeConfigMap,
				},
				ClientIDPath:     ptr.To("client-id"),
				ClientSecretPath: ptr.To("client-secret"),
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "no name found for oauth",
		},
		{
			name:    "OAuth ConfigMap not found",
			objects: nil,
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type:      persesv1alpha2.SecretSourceTypeConfigMap,
					Name:      ptr.To("missing-cm"),
					Namespace: ptr.To("perses-system"),
				},
				ClientIDPath:     ptr.To("client-id"),
				ClientSecretPath: ptr.To("client-secret"),
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "not found",
		},
		{
			name: "OAuth ConfigMap missing client ID key",
			objects: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "oauth-cm", Namespace: "perses-system"},
					Data:       map[string]string{"other-key": "value"},
				},
			},
			perses: persesWithOAuth(&persesv1alpha2.OAuth{
				SecretSource: persesv1alpha2.SecretSource{
					Type:      persesv1alpha2.SecretSourceTypeConfigMap,
					Name:      ptr.To("oauth-cm"),
					Namespace: ptr.To("perses-system"),
				},
				ClientIDPath:     ptr.To("client-id"),
				ClientSecretPath: ptr.To("client-secret"),
				TokenURL:         "http://perses.perses-system.svc:8080/token",
			}),
			errSubstr: "no client id data found",
		},

		// --- TLS CaCert error paths ---
		{
			name:    "TLS CaCert missing subject name",
			objects: nil,
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				CaCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type: persesv1alpha2.SecretSourceTypeSecret,
					},
					CertPath: "ca.crt",
				},
			}),
			errSubstr: "no name found for tls certificate",
		},
		{
			name:    "TLS CaCert secret not found",
			objects: nil,
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				CaCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeSecret,
						Name:      ptr.To("missing-ca"),
						Namespace: ptr.To("perses-system"),
					},
					CertPath: "ca.crt",
				},
			}),
			errSubstr: "not found",
		},
		{
			name: "TLS CaCert missing key in secret",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "ca-secret", Namespace: "perses-system"},
					Data:       map[string][]byte{"other-key": []byte("data")},
				},
			},
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				CaCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeSecret,
						Name:      ptr.To("ca-secret"),
						Namespace: ptr.To("perses-system"),
					},
					CertPath: "ca.crt",
				},
			}),
			errSubstr: "no data found for certificate",
		},
		{
			name:    "TLS CaCert ConfigMap missing subject name",
			objects: nil,
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				CaCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type: persesv1alpha2.SecretSourceTypeConfigMap,
					},
					CertPath: "ca.crt",
				},
			}),
			errSubstr: "no name found for tls certificate",
		},
		{
			name: "TLS CaCert ConfigMap missing key",
			objects: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "ca-cm", Namespace: "perses-system"},
					Data:       map[string]string{"other-key": "value"},
				},
			},
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				CaCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeConfigMap,
						Name:      ptr.To("ca-cm"),
						Namespace: ptr.To("perses-system"),
					},
					CertPath: "ca.crt",
				},
			}),
			errSubstr: "no data found for certificate",
		},

		// --- TLS UserCert error paths ---
		{
			name:    "TLS UserCert missing subject name",
			objects: nil,
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				UserCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type: persesv1alpha2.SecretSourceTypeSecret,
					},
					CertPath:       "tls.crt",
					PrivateKeyPath: ptr.To("tls.key"),
				},
			}),
			errSubstr: "no name found for tls certificate",
		},
		{
			name:    "TLS UserCert secret not found",
			objects: nil,
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				UserCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeSecret,
						Name:      ptr.To("missing-cert"),
						Namespace: ptr.To("perses-system"),
					},
					CertPath:       "tls.crt",
					PrivateKeyPath: ptr.To("tls.key"),
				},
			}),
			errSubstr: "not found",
		},
		{
			name: "TLS UserCert missing cert key in secret",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "cert-secret", Namespace: "perses-system"},
					Data:       map[string][]byte{"other-key": []byte("data")},
				},
			},
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				UserCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeSecret,
						Name:      ptr.To("cert-secret"),
						Namespace: ptr.To("perses-system"),
					},
					CertPath:       "tls.crt",
					PrivateKeyPath: ptr.To("tls.key"),
				},
			}),
			errSubstr: "no data found for certificate",
		},
		{
			name:    "TLS UserCert ConfigMap missing subject name",
			objects: nil,
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				UserCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type: persesv1alpha2.SecretSourceTypeConfigMap,
					},
					CertPath:       "tls.crt",
					PrivateKeyPath: ptr.To("tls.key"),
				},
			}),
			errSubstr: "no name found for tls certificate",
		},
		{
			name: "TLS UserCert ConfigMap missing cert key",
			objects: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "cert-cm", Namespace: "perses-system"},
					Data:       map[string]string{"other-key": "value"},
				},
			},
			perses: persesWithClientTLS(&persesv1alpha2.TLS{
				Enable: ptr.To(true),
				UserCert: &persesv1alpha2.Certificate{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeConfigMap,
						Name:      ptr.To("cert-cm"),
						Namespace: ptr.To("perses-system"),
					},
					CertPath:       "tls.crt",
					PrivateKeyPath: ptr.To("tls.key"),
				},
			}),
			errSubstr: "no data found for certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(newScheme())
			for _, obj := range tt.objects {
				builder = builder.WithRuntimeObjects(obj)
			}
			reader := builder.Build()
			factory := NewWithConfig(AllowedSecretFileDirs{})
			_, err := factory.buildClient(ctx, reader, tt.perses)
			require.Error(t, err)
			if tt.errSubstr != "" {
				assert.Contains(t, err.Error(), tt.errSubstr)
			}
		})
	}
}

func TestBuildClientKubernetesAuthError(t *testing.T) {
	ctx := context.Background()

	// KubernetesAuth reads the service-account token from a fixed filesystem path.
	// Outside a Kubernetes pod this file is absent and buildClient must surface
	// the error.  When running inside a pod the file exists, so we skip that case.
	if _, err := os.Stat(tokenPath); err == nil {
		t.Skipf("skipping: service account token file %q exists in this environment", tokenPath)
	}

	reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()
	perses := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "perses",
			Namespace:  "perses-system",
			Generation: 1,
		},
		Spec: persesv1alpha2.PersesSpec{
			ContainerPort: ptr.To(int32(8080)),
			Client: &persesv1alpha2.Client{
				KubernetesAuth: &persesv1alpha2.KubernetesAuth{
					Enable: ptr.To(true),
				},
			},
		},
	}

	factory := NewWithConfig(AllowedSecretFileDirs{})
	_, err := factory.buildClient(ctx, reader, perses)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read service account token")
}

func TestBuildClientOAuthConfigMap(t *testing.T) {
	ctx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "oauth-cm", Namespace: "perses-system"},
		Data: map[string]string{
			"client-id":     "cm-client-id",
			"client-secret": "cm-client-secret",
		},
	}
	reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(cm).Build()

	perses := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
		Spec: persesv1alpha2.PersesSpec{
			ContainerPort: ptr.To(int32(8080)),
			Client: &persesv1alpha2.Client{
				OAuth: &persesv1alpha2.OAuth{
					SecretSource: persesv1alpha2.SecretSource{
						Type:      persesv1alpha2.SecretSourceTypeConfigMap,
						Name:      ptr.To("oauth-cm"),
						Namespace: ptr.To("perses-system"),
					},
					ClientIDPath:     ptr.To("client-id"),
					ClientSecretPath: ptr.To("client-secret"),
					TokenURL:         "http://perses.perses-system.svc:8080/token",
				},
			},
		},
	}

	factory := NewWithConfig(AllowedSecretFileDirs{})
	cl, err := factory.buildClient(ctx, reader, perses)
	require.NoError(t, err)
	require.NotNil(t, cl)

	rest := cl.RESTClient()
	require.NotNil(t, rest)
	assert.NotNil(t, rest.Client)
}

func TestBuildClientOAuthFile(t *testing.T) {
	ctx := context.Background()

	allowedDir := t.TempDir()
	// Create temp files for the client ID and secret inside the allowed directory
	idFile, err := os.CreateTemp(allowedDir, "oauth-client-id-*")
	require.NoError(t, err)
	_, err = idFile.WriteString("perses-operator")
	require.NoError(t, err)
	require.NoError(t, idFile.Close())

	secretFile, err := os.CreateTemp(allowedDir, "oauth-client-secret-*")
	require.NoError(t, err)
	_, err = secretFile.WriteString("super-secret")
	require.NoError(t, err)
	require.NoError(t, secretFile.Close())

	reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

	t.Run("file type rejected when allowedDirs not configured", func(t *testing.T) {
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client: &persesv1alpha2.Client{
					OAuth: &persesv1alpha2.OAuth{
						SecretSource: persesv1alpha2.SecretSource{
							Type: persesv1alpha2.SecretSourceTypeFile,
						},
						ClientIDPath:     ptr.To(idFile.Name()),
						ClientSecretPath: ptr.To(secretFile.Name()),
						TokenURL:         "http://perses.perses-system.svc:8080/token",
					},
				},
			},
		}

		factory := NewWithConfig(AllowedSecretFileDirs{})
		_, err := factory.buildClient(ctx, reader, perses)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file references in secrets are disabled")
	})

	t.Run("file type with valid files inside allowedDirs", func(t *testing.T) {
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client: &persesv1alpha2.Client{
					OAuth: &persesv1alpha2.OAuth{
						SecretSource: persesv1alpha2.SecretSource{
							Type: persesv1alpha2.SecretSourceTypeFile,
						},
						ClientIDPath:     ptr.To(idFile.Name()),
						ClientSecretPath: ptr.To(secretFile.Name()),
						TokenURL:         "http://perses.perses-system.svc:8080/token",
					},
				},
			},
		}

		factory := NewWithConfig(testAllowedDirs(allowedDir))
		cl, err := factory.buildClient(ctx, reader, perses)
		require.NoError(t, err)
		require.NotNil(t, cl)

		rest := cl.RESTClient()
		require.NotNil(t, rest)
		assert.NotNil(t, rest.Client)
	})

	t.Run("file type without ClientID or ClientIDPath", func(t *testing.T) {
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client: &persesv1alpha2.Client{
					OAuth: &persesv1alpha2.OAuth{
						SecretSource: persesv1alpha2.SecretSource{
							Type: persesv1alpha2.SecretSourceTypeFile,
						},
						ClientIDPath:     nil,
						ClientSecretPath: ptr.To(secretFile.Name()),
						TokenURL:         "http://perses.perses-system.svc:8080/token",
					},
				},
			},
		}

		factory := NewWithConfig(testAllowedDirs(allowedDir))
		_, err := factory.buildClient(ctx, reader, perses)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "clientID or clientIDPath is required when OAuth type is file")
	})

	t.Run("file type with literal ClientID reads only the secret file", func(t *testing.T) {
		secretOnlyDir := t.TempDir()
		secretPath := filepath.Join(secretOnlyDir, "client-secret")
		require.NoError(t, os.WriteFile(secretPath, []byte("super-secret"), 0600))
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client: &persesv1alpha2.Client{
					OAuth: &persesv1alpha2.OAuth{
						SecretSource:     persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeFile},
						ClientID:         ptr.To("perses-operator"),
						ClientSecretPath: ptr.To(secretPath),
						TokenURL:         "http://perses.perses-system.svc:8080/token",
					},
				},
			},
		}

		factory := NewWithConfig(testAllowedDirs(secretOnlyDir))
		cl, err := factory.buildClient(ctx, reader, perses)
		require.NoError(t, err)
		require.NotNil(t, cl)
	})

	t.Run("file type nil ClientSecretPath", func(t *testing.T) {
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client: &persesv1alpha2.Client{
					OAuth: &persesv1alpha2.OAuth{
						SecretSource: persesv1alpha2.SecretSource{
							Type: persesv1alpha2.SecretSourceTypeFile,
						},
						ClientIDPath:     ptr.To(idFile.Name()),
						ClientSecretPath: nil,
						TokenURL:         "http://perses.perses-system.svc:8080/token",
					},
				},
			},
		}

		factory := NewWithConfig(testAllowedDirs(allowedDir))
		_, err := factory.buildClient(ctx, reader, perses)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "clientSecretPath is required when OAuth type is file")
	})

	t.Run("file type non-existent ClientIDPath", func(t *testing.T) {
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client: &persesv1alpha2.Client{
					OAuth: &persesv1alpha2.OAuth{
						SecretSource: persesv1alpha2.SecretSource{
							Type: persesv1alpha2.SecretSourceTypeFile,
						},
						ClientIDPath:     ptr.To("/nonexistent/path/to/client-id"),
						ClientSecretPath: ptr.To(secretFile.Name()),
						TokenURL:         "http://perses.perses-system.svc:8080/token",
					},
				},
			},
		}

		factory := NewWithConfig(testAllowedDirs(allowedDir))
		_, err := factory.buildClient(ctx, reader, perses)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid OAuth client ID file")
	})

	outsideSecret := filepath.Join(t.TempDir(), "client-secret")
	require.NoError(t, os.WriteFile(outsideSecret, []byte("super-secret"), 0600))
	for name, secretPath := range map[string]string{
		"outside allowedDirs": outsideSecret,
		"missing":             filepath.Join(allowedDir, "missing-secret"),
	} {
		t.Run("file type ClientSecretPath "+name, func(t *testing.T) {
			perses := persesv1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
				Spec: persesv1alpha2.PersesSpec{
					ContainerPort: ptr.To(int32(8080)),
					Client: &persesv1alpha2.Client{
						OAuth: &persesv1alpha2.OAuth{
							SecretSource:     persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeFile},
							ClientIDPath:     ptr.To(idFile.Name()),
							ClientSecretPath: ptr.To(secretPath),
							TokenURL:         "http://perses.perses-system.svc:8080/token",
						},
					},
				},
			}

			factory := NewWithConfig(testAllowedDirs(allowedDir))
			_, err := factory.buildClient(ctx, reader, perses)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid OAuth client secret file")
		})
	}
}

// writeSelfSignedCert writes a PEM cert and key into dir and returns their paths.
func writeSelfSignedCert(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "perses-operator-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPath = filepath.Join(dir, "tls.crt")
	keyPath = filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600))
	return certPath, keyPath
}

func TestBuildClientTLSFile(t *testing.T) {
	ctx := context.Background()
	reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

	allowedDir := t.TempDir()
	certPath, keyPath := writeSelfSignedCert(t, allowedDir)
	outsideCertPath, outsideKeyPath := writeSelfSignedCert(t, t.TempDir())

	fileCert := func(certPath string, keyPath *string) *persesv1alpha2.Certificate {
		return &persesv1alpha2.Certificate{
			SecretSource:   persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeFile},
			CertPath:       certPath,
			PrivateKeyPath: keyPath,
		}
	}
	persesWithTLS := func(tls *persesv1alpha2.TLS) persesv1alpha2.Perses {
		tls.Enable = ptr.To(true)
		return persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
			Spec: persesv1alpha2.PersesSpec{
				ContainerPort: ptr.To(int32(8080)),
				Client:        &persesv1alpha2.Client{TLS: tls},
			},
		}
	}

	tests := []struct {
		name        string
		allowedDirs []string
		tls         *persesv1alpha2.TLS
		errSubstr   string
	}{
		{
			name:      "CA cert rejected when allowedDirs not configured",
			tls:       &persesv1alpha2.TLS{CaCert: fileCert(certPath, nil)},
			errSubstr: "file references in secrets are disabled",
		},
		{
			name:        "CA cert outside allowedDirs is rejected",
			allowedDirs: []string{allowedDir},
			tls:         &persesv1alpha2.TLS{CaCert: fileCert(outsideCertPath, nil)},
			errSubstr:   "invalid CA cert file",
		},
		{
			name:        "CA cert inside allowedDirs",
			allowedDirs: []string{allowedDir},
			tls:         &persesv1alpha2.TLS{CaCert: fileCert(certPath, nil)},
		},
		{
			name:      "user cert rejected when allowedDirs not configured",
			tls:       &persesv1alpha2.TLS{UserCert: fileCert(certPath, ptr.To(keyPath))},
			errSubstr: "file references in secrets are disabled",
		},
		{
			name:        "user cert outside allowedDirs is rejected",
			allowedDirs: []string{allowedDir},
			tls:         &persesv1alpha2.TLS{UserCert: fileCert(outsideCertPath, ptr.To(keyPath))},
			errSubstr:   "invalid user cert file",
		},
		{
			name:        "user cert private key outside allowedDirs is rejected",
			allowedDirs: []string{allowedDir},
			tls:         &persesv1alpha2.TLS{UserCert: fileCert(certPath, ptr.To(outsideKeyPath))},
			errSubstr:   "invalid user cert private key file",
		},
		{
			name:        "user cert without private key path is rejected",
			allowedDirs: []string{allowedDir},
			tls:         &persesv1alpha2.TLS{UserCert: fileCert(certPath, nil)},
			errSubstr:   "privateKeyPath is required",
		},
		{
			name:        "user cert and private key inside allowedDirs",
			allowedDirs: []string{allowedDir},
			tls: &persesv1alpha2.TLS{
				CaCert:   fileCert(certPath, nil),
				UserCert: fileCert(certPath, ptr.To(keyPath)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewWithConfig(testAllowedDirs(tt.allowedDirs...))
			cl, err := factory.buildClient(ctx, reader, persesWithTLS(tt.tls))
			if tt.errSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cl)
		})
	}
}

func TestBuildClientFileCredentialsReachTokenEndpoint(t *testing.T) {
	dir := t.TempDir()
	idPath := filepath.Join(dir, "client-id")
	secretPath := filepath.Join(dir, "client-secret")
	require.NoError(t, os.WriteFile(idPath, []byte("perses-operator"), 0600))
	require.NoError(t, os.WriteFile(secretPath, []byte("super-secret"), 0600))

	var mu sync.Mutex
	var gotID, gotSecret, gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/token" {
			gotAuthorization = r.Header.Get("Authorization")
			return
		}
		id, secret, ok := r.BasicAuth()
		if !ok {
			_ = r.ParseForm()
			id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
		}
		gotID, gotSecret = id, secret
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"bearer"}`))
	}))
	defer server.Close()

	perses := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
		Spec: persesv1alpha2.PersesSpec{
			Client: &persesv1alpha2.Client{
				OAuth: &persesv1alpha2.OAuth{
					SecretSource:     persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeFile},
					ClientIDPath:     ptr.To(idPath),
					ClientSecretPath: ptr.To(secretPath),
					TokenURL:         server.URL + "/token",
				},
			},
		},
	}
	reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()
	cl, err := NewWithConfig(testAllowedDirs(dir)).buildClient(context.Background(), reader, perses)
	require.NoError(t, err)

	resp, err := cl.RESTClient().Client.Get(server.URL + "/api")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "perses-operator", gotID)
	assert.Equal(t, "super-secret", gotSecret)
	assert.Equal(t, "Bearer tok", gotAuthorization)
}

func TestBuildClientFileCredentialsReachTLSServer(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeSelfSignedCert(t, dir)
	clientCertPEM, err := os.ReadFile(certPath)
	require.NoError(t, err)
	clientCAs := x509.NewCertPool()
	require.True(t, clientCAs.AppendCertsFromPEM(clientCertPEM))

	var mu sync.Mutex
	var peerCN string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		peerCN = r.TLS.PeerCertificates[0].Subject.CommonName
	}))
	server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	server.StartTLS()
	defer server.Close()

	// The client can only trust the server through this CA file.
	caPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600))

	fileCert := func(certPath string, keyPath *string) *persesv1alpha2.Certificate {
		return &persesv1alpha2.Certificate{
			SecretSource:   persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeFile},
			CertPath:       certPath,
			PrivateKeyPath: keyPath,
		}
	}
	get := func(userCert *persesv1alpha2.Certificate) error {
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
			Spec: persesv1alpha2.PersesSpec{
				Client: &persesv1alpha2.Client{TLS: &persesv1alpha2.TLS{
					Enable:   ptr.To(true),
					CaCert:   fileCert(caPath, nil),
					UserCert: userCert,
				}},
			},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()
		cl, err := NewWithConfig(testAllowedDirs(dir)).buildClient(context.Background(), reader, perses)
		if err != nil {
			return err
		}
		resp, err := cl.RESTClient().Client.Get(server.URL)
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}

	t.Run("cert and key reach the server", func(t *testing.T) {
		require.NoError(t, get(fileCert(certPath, ptr.To(keyPath))))
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, "perses-operator-test", peerCN)
	})

	t.Run("swapped cert and key fail", func(t *testing.T) {
		require.Error(t, get(fileCert(keyPath, ptr.To(certPath))))
	})
}

func TestBuildClientKubernetesAuthAndOAuthConflict(t *testing.T) {
	ctx := context.Background()
	reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

	perses := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system", Generation: 1},
		Spec: persesv1alpha2.PersesSpec{
			ContainerPort: ptr.To(int32(8080)),
			Client: &persesv1alpha2.Client{
				KubernetesAuth: &persesv1alpha2.KubernetesAuth{
					Enable: ptr.To(true),
				},
				OAuth: &persesv1alpha2.OAuth{
					SecretSource: persesv1alpha2.SecretSource{
						Type: persesv1alpha2.SecretSourceTypeSecret,
					},
					ClientIDPath:     ptr.To("OPERATOR_CLIENT_ID"),
					ClientSecretPath: ptr.To("OPERATOR_CLIENT_SECRET"),
					TokenURL:         "http://perses.perses-system.svc:8080/token",
				},
			},
		},
	}

	factory := NewWithConfig(AllowedSecretFileDirs{})
	_, err := factory.buildClient(ctx, reader, perses)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestConfigFingerprintOtherFields(t *testing.T) {
	base := persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "perses-1",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: persesv1alpha2.PersesSpec{
			ContainerPort: ptr.To(int32(8080)),
		},
	}
	fpBase := configFingerprint(base)

	// APIPrefix change alters fingerprint
	withPrefix := base.DeepCopy()
	withPrefix.Spec.Config.APIPrefix = "/api/v1"
	assert.NotEqual(t, fpBase, configFingerprint(*withPrefix), "APIPrefix change should alter fingerprint")

	// Same APIPrefix produces same fingerprint
	assert.Equal(t, configFingerprint(*withPrefix), configFingerprint(*withPrefix.DeepCopy()),
		"Same APIPrefix config should produce same fingerprint")

	// KubernetesAuth enabled alters fingerprint
	withK8sAuth := base.DeepCopy()
	withK8sAuth.Spec.Client = &persesv1alpha2.Client{
		KubernetesAuth: &persesv1alpha2.KubernetesAuth{
			Enable: ptr.To(true),
		},
	}
	assert.NotEqual(t, fpBase, configFingerprint(*withK8sAuth), "Enabling KubernetesAuth should alter fingerprint")

	// OAuth namespace change alters fingerprint
	oauthNS1 := base.DeepCopy()
	oauthNS1.Spec.Client = &persesv1alpha2.Client{
		OAuth: &persesv1alpha2.OAuth{
			SecretSource: persesv1alpha2.SecretSource{
				Type:      persesv1alpha2.SecretSourceTypeSecret,
				Name:      ptr.To("oauth-secret"),
				Namespace: ptr.To("ns1"),
			},
			ClientIDPath: ptr.To("client-id"),
			TokenURL:     "http://example.com/token",
		},
	}
	oauthNS2 := oauthNS1.DeepCopy()
	oauthNS2.Spec.Client.OAuth.Namespace = ptr.To("ns2")
	assert.NotEqual(t, configFingerprint(*oauthNS1), configFingerprint(*oauthNS2),
		"Different OAuth namespace should alter fingerprint")

	// OAuth type change (secret → configmap) alters fingerprint
	oauthSecret := base.DeepCopy()
	oauthSecret.Spec.Client = &persesv1alpha2.Client{
		OAuth: &persesv1alpha2.OAuth{
			SecretSource: persesv1alpha2.SecretSource{
				Type: persesv1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("oauth-secret"),
			},
			ClientIDPath: ptr.To("client-id"),
			TokenURL:     "http://example.com/token",
		},
	}
	oauthCM := oauthSecret.DeepCopy()
	oauthCM.Spec.Client.OAuth.Type = persesv1alpha2.SecretSourceTypeConfigMap
	assert.NotEqual(t, configFingerprint(*oauthSecret), configFingerprint(*oauthCM),
		"Different OAuth source type should alter fingerprint")

	// OAuth name change alters fingerprint
	oauthName1 := base.DeepCopy()
	oauthName1.Spec.Client = &persesv1alpha2.Client{
		OAuth: &persesv1alpha2.OAuth{
			SecretSource: persesv1alpha2.SecretSource{
				Type: persesv1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("name1"),
			},
			ClientIDPath: ptr.To("client-id"),
			TokenURL:     "http://example.com/token",
		},
	}
	oauthName2 := oauthName1.DeepCopy()
	oauthName2.Spec.Client.OAuth.Name = ptr.To("name2")
	assert.NotEqual(t, configFingerprint(*oauthName1), configFingerprint(*oauthName2),
		"Different OAuth name should alter fingerprint")

	// Client nil should not crash and produce stable fingerprint
	fpNilClient := configFingerprint(base)
	assert.Equal(t, fpBase, fpNilClient, "Nil/empty Client should not alter fingerprint")
}
