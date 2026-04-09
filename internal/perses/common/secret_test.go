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
	"testing"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

func TestGetBasicAuthData(t *testing.T) {
	ctx := context.Background()

	t.Run("reads password from Secret", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
			Data:       map[string][]byte{"password": []byte("s3cret")},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		basicAuth := &v1alpha2.BasicAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("my-secret"),
			},
			Username:     "admin",
			PasswordPath: "password",
		}

		password, err := GetBasicAuthData(ctx, reader, "default", "test-ds", basicAuth)
		require.NoError(t, err)
		assert.Equal(t, "s3cret", password)
	})

	t.Run("reads password from ConfigMap", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
			Data:       map[string]string{"password": "cm-password"},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(cm).Build()

		basicAuth := &v1alpha2.BasicAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeConfigMap,
				Name: ptr.To("my-cm"),
			},
			Username:     "admin",
			PasswordPath: "password",
		}

		password, err := GetBasicAuthData(ctx, reader, "default", "test-ds", basicAuth)
		require.NoError(t, err)
		assert.Equal(t, "cm-password", password)
	})

	t.Run("respects namespace override", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "other-ns"},
			Data:       map[string][]byte{"password": []byte("cross-ns")},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		basicAuth := &v1alpha2.BasicAuth{
			SecretSource: v1alpha2.SecretSource{
				Type:      v1alpha2.SecretSourceTypeSecret,
				Name:      ptr.To("my-secret"),
				Namespace: ptr.To("other-ns"),
			},
			Username:     "admin",
			PasswordPath: "password",
		}

		password, err := GetBasicAuthData(ctx, reader, "default", "test-ds", basicAuth)
		require.NoError(t, err)
		assert.Equal(t, "cross-ns", password)
	})

	t.Run("returns error when secret not found", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		basicAuth := &v1alpha2.BasicAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("missing-secret"),
			},
			Username:     "admin",
			PasswordPath: "password",
		}

		_, err := GetBasicAuthData(ctx, reader, "default", "test-ds", basicAuth)
		require.Error(t, err)
	})

	t.Run("returns error when key missing in secret", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
			Data:       map[string][]byte{"other-key": []byte("value")},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		basicAuth := &v1alpha2.BasicAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("my-secret"),
			},
			Username:     "admin",
			PasswordPath: "password",
		}

		_, err := GetBasicAuthData(ctx, reader, "default", "test-ds", basicAuth)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no password data found")
	})

	t.Run("returns empty string for non-secret/configmap type", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		basicAuth := &v1alpha2.BasicAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeFile,
			},
			Username:     "admin",
			PasswordPath: "/path/to/password",
		}

		password, err := GetBasicAuthData(ctx, reader, "default", "test-ds", basicAuth)
		require.NoError(t, err)
		assert.Equal(t, "", password)
	})

	t.Run("returns error when name is nil", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		basicAuth := &v1alpha2.BasicAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
			},
			Username:     "admin",
			PasswordPath: "password",
		}

		_, err := GetBasicAuthData(ctx, reader, "default", "test-ds", basicAuth)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no name found")
	})
}

func TestGetOAuthData(t *testing.T) {
	ctx := context.Background()

	t.Run("reads client ID and secret from Secret", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "oauth-secret", Namespace: "default"},
			Data: map[string][]byte{
				"client-id":     []byte("my-client-id"),
				"client-secret": []byte("my-client-secret"),
			},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		oauth := &v1alpha2.OAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("oauth-secret"),
			},
			ClientIDPath:     ptr.To("client-id"),
			ClientSecretPath: ptr.To("client-secret"),
			TokenURL:         "https://auth.example.com/token",
		}

		clientID, clientSecret, err := GetOAuthData(ctx, reader, "default", "test-ds", oauth)
		require.NoError(t, err)
		assert.Equal(t, "my-client-id", clientID)
		assert.Equal(t, "my-client-secret", clientSecret)
	})

	t.Run("reads from ConfigMap", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "oauth-cm", Namespace: "default"},
			Data: map[string]string{
				"client-id":     "cm-client-id",
				"client-secret": "cm-client-secret",
			},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(cm).Build()

		oauth := &v1alpha2.OAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeConfigMap,
				Name: ptr.To("oauth-cm"),
			},
			ClientIDPath:     ptr.To("client-id"),
			ClientSecretPath: ptr.To("client-secret"),
			TokenURL:         "https://auth.example.com/token",
		}

		clientID, clientSecret, err := GetOAuthData(ctx, reader, "default", "test-ds", oauth)
		require.NoError(t, err)
		assert.Equal(t, "cm-client-id", clientID)
		assert.Equal(t, "cm-client-secret", clientSecret)
	})

	t.Run("returns error when secret not found", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		oauth := &v1alpha2.OAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("missing"),
			},
			ClientIDPath:     ptr.To("client-id"),
			ClientSecretPath: ptr.To("client-secret"),
		}

		_, _, err := GetOAuthData(ctx, reader, "default", "test-ds", oauth)
		require.Error(t, err)
	})

	t.Run("returns error when client ID key missing", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "oauth-secret", Namespace: "default"},
			Data: map[string][]byte{
				"client-secret": []byte("my-client-secret"),
			},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		oauth := &v1alpha2.OAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("oauth-secret"),
			},
			ClientIDPath:     ptr.To("client-id"),
			ClientSecretPath: ptr.To("client-secret"),
		}

		_, _, err := GetOAuthData(ctx, reader, "default", "test-ds", oauth)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no client id data found")
	})

	t.Run("returns error when client secret key missing", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "oauth-secret", Namespace: "default"},
			Data: map[string][]byte{
				"client-id": []byte("my-client-id"),
			},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		oauth := &v1alpha2.OAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("oauth-secret"),
			},
			ClientIDPath:     ptr.To("client-id"),
			ClientSecretPath: ptr.To("client-secret"),
		}

		_, _, err := GetOAuthData(ctx, reader, "default", "test-ds", oauth)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no client secret data found")
	})

	t.Run("returns empty strings for file type", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		oauth := &v1alpha2.OAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeFile,
			},
			TokenURL: "https://auth.example.com/token",
		}

		clientID, clientSecret, err := GetOAuthData(ctx, reader, "default", "test-ds", oauth)
		require.NoError(t, err)
		assert.Equal(t, "", clientID)
		assert.Equal(t, "", clientSecret)
	})

	t.Run("returns error when name is nil", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		oauth := &v1alpha2.OAuth{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
			},
			ClientIDPath: ptr.To("client-id"),
		}

		_, _, err := GetOAuthData(ctx, reader, "default", "test-ds", oauth)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no name found")
	})
}

func TestGetTLSCertData(t *testing.T) {
	ctx := context.Background()

	t.Run("reads cert and key from Secret", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tls-secret", Namespace: "default"},
			Data: map[string][]byte{
				"tls.crt": []byte("cert-data"),
				"tls.key": []byte("key-data"),
			},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		cert := &v1alpha2.Certificate{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("tls-secret"),
			},
			CertPath:       "tls.crt",
			PrivateKeyPath: ptr.To("tls.key"),
		}

		certData, keyData, err := GetTLSCertData(ctx, reader, "default", "test-ds", cert)
		require.NoError(t, err)
		assert.Equal(t, "cert-data", certData)
		assert.Equal(t, "key-data", keyData)
	})

	t.Run("reads cert without key from Secret (CA cert)", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "ca-secret", Namespace: "default"},
			Data: map[string][]byte{
				"ca.crt": []byte("ca-cert-data"),
			},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		cert := &v1alpha2.Certificate{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("ca-secret"),
			},
			CertPath: "ca.crt",
		}

		certData, keyData, err := GetTLSCertData(ctx, reader, "default", "test-ds", cert)
		require.NoError(t, err)
		assert.Equal(t, "ca-cert-data", certData)
		assert.Equal(t, "", keyData)
	})

	t.Run("reads from ConfigMap", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "tls-cm", Namespace: "default"},
			Data: map[string]string{
				"tls.crt": "cm-cert-data",
				"tls.key": "cm-key-data",
			},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(cm).Build()

		cert := &v1alpha2.Certificate{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeConfigMap,
				Name: ptr.To("tls-cm"),
			},
			CertPath:       "tls.crt",
			PrivateKeyPath: ptr.To("tls.key"),
		}

		certData, keyData, err := GetTLSCertData(ctx, reader, "default", "test-ds", cert)
		require.NoError(t, err)
		assert.Equal(t, "cm-cert-data", certData)
		assert.Equal(t, "cm-key-data", keyData)
	})

	t.Run("returns error when secret not found", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		cert := &v1alpha2.Certificate{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("missing"),
			},
			CertPath: "tls.crt",
		}

		_, _, err := GetTLSCertData(ctx, reader, "default", "test-ds", cert)
		require.Error(t, err)
	})

	t.Run("returns error when cert key missing in secret", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tls-secret", Namespace: "default"},
			Data:       map[string][]byte{"other-key": []byte("value")},
		}
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(secret).Build()

		cert := &v1alpha2.Certificate{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
				Name: ptr.To("tls-secret"),
			},
			CertPath: "tls.crt",
		}

		_, _, err := GetTLSCertData(ctx, reader, "default", "test-ds", cert)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no data found for certificate")
	})

	t.Run("returns empty strings for file type", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		cert := &v1alpha2.Certificate{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeFile,
			},
			CertPath: "/path/to/cert",
		}

		certData, keyData, err := GetTLSCertData(ctx, reader, "default", "test-ds", cert)
		require.NoError(t, err)
		assert.Equal(t, "", certData)
		assert.Equal(t, "", keyData)
	})

	t.Run("returns error when name is nil", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		cert := &v1alpha2.Certificate{
			SecretSource: v1alpha2.SecretSource{
				Type: v1alpha2.SecretSourceTypeSecret,
			},
			CertPath: "tls.crt",
		}

		_, _, err := GetTLSCertData(ctx, reader, "default", "test-ds", cert)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no name found")
	})
}

func TestHasSecretConfig(t *testing.T) {
	t.Run("returns false for nil client", func(t *testing.T) {
		assert.False(t, HasSecretConfig(nil))
	})

	t.Run("returns false for empty client", func(t *testing.T) {
		assert.False(t, HasSecretConfig(&v1alpha2.Client{}))
	})

	t.Run("returns true for BasicAuth", func(t *testing.T) {
		assert.True(t, HasSecretConfig(&v1alpha2.Client{
			BasicAuth: &v1alpha2.BasicAuth{},
		}))
	})

	t.Run("returns true for OAuth", func(t *testing.T) {
		assert.True(t, HasSecretConfig(&v1alpha2.Client{
			OAuth: &v1alpha2.OAuth{},
		}))
	})

	t.Run("returns true for TLS enabled", func(t *testing.T) {
		assert.True(t, HasSecretConfig(&v1alpha2.Client{
			TLS: &v1alpha2.TLS{Enable: ptr.To(true)},
		}))
	})

	t.Run("returns false for TLS disabled", func(t *testing.T) {
		assert.False(t, HasSecretConfig(&v1alpha2.Client{
			TLS: &v1alpha2.TLS{Enable: ptr.To(false)},
		}))
	})
}
