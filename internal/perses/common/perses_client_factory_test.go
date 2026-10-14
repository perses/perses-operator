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
	"os"
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

	persesv1alpha2 "github.com/rhobs/perses-operator/api/v1alpha2"
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
	factory := NewWithConfig()

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
	factory := NewWithConfig()
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
	factory := NewWithConfig()
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
	factory := NewWithConfig()
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
	factory := NewWithConfig()

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

	factory := NewWithConfig()
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

	factory := NewWithConfig()
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
			factory := NewWithConfig()
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

	factory := NewWithConfig()
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

	factory := NewWithConfig()
	cl, err := factory.buildClient(ctx, reader, perses)
	require.NoError(t, err)
	require.NotNil(t, cl)

	rest := cl.RESTClient()
	require.NotNil(t, rest)
	assert.NotNil(t, rest.Client)
}

func TestBuildClientOAuthFile(t *testing.T) {
	ctx := context.Background()

	// Create temp files for the client ID and secret
	idFile, err := os.CreateTemp("", "oauth-client-id-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(idFile.Name()) })
	_, err = idFile.WriteString("perses-operator")
	require.NoError(t, err)
	require.NoError(t, idFile.Close())

	secretFile, err := os.CreateTemp("", "oauth-client-secret-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(secretFile.Name()) })
	_, err = secretFile.WriteString("super-secret")
	require.NoError(t, err)
	require.NoError(t, secretFile.Close())

	reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

	t.Run("file type with valid files", func(t *testing.T) {
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

		factory := NewWithConfig()
		cl, err := factory.buildClient(ctx, reader, perses)
		require.NoError(t, err)
		require.NotNil(t, cl)

		rest := cl.RESTClient()
		require.NotNil(t, rest)
		assert.NotNil(t, rest.Client)
	})

	t.Run("file type nil ClientIDPath", func(t *testing.T) {
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

		factory := NewWithConfig()
		_, err := factory.buildClient(ctx, reader, perses)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "clientIDPath is required when OAuth type is file")
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

		factory := NewWithConfig()
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

		factory := NewWithConfig()
		_, err := factory.buildClient(ctx, reader, perses)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read OAuth client ID file")
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

	factory := NewWithConfig()
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
