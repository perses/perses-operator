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
	"flag"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/perses/perses/pkg/model/api/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
)

const factoryServiceName = "perses-1.default.svc.cluster.local"
const factoryTokenPath = "/api/auth/providers/oidc/native/token"

// A shared CA keeps hostname rejection distinct from unknown-authority failures.
type factoryTLSCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func newFactoryTLSCA(t *testing.T) *factoryTLSCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "factory test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	return &factoryTLSCA{cert: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

func (ca *factoryTLSCA) issue(t *testing.T, hostname string, client bool) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	cert := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: hostname},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(hostname); ip != nil {
		cert.IPAddresses = []net.IP{ip}
	} else {
		cert.DNSNames = []string{hostname}
	}
	if client {
		cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair, certPEM, keyPEM
}

func factoryTLSServer(t *testing.T, ca *factoryTLSCA, hostname string, handler http.HandlerFunc) (*httptest.Server, <-chan struct{}) {
	t.Helper()
	cert, _, _ := ca.issue(t, hostname, false)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca.pem))
	closed := make(chan struct{}, 10)
	server := httptest.NewUnstartedServer(handler)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			closed <- struct{}{}
		}
	}
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12,
		ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots,
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, closed
}

func factoryTLSPerses(t *testing.T, ca *factoryTLSCA, apiURL, tokenURL string, authStyle int32) (persesv1alpha2.Perses, *corev1.Secret) {
	t.Helper()
	_, certPEM, keyPEM := ca.issue(t, "operator", true)
	_, port, err := net.SplitHostPort(strings.TrimPrefix(apiURL, "https://"))
	require.NoError(t, err)
	portNumber, err := strconv.Atoi(port)
	require.NoError(t, err)
	source := persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeSecret, Name: ptr.To("client")}
	perses := newPersesForPods()
	perses.Spec.ContainerPort = ptr.To(int32(portNumber))
	perses.Spec.Config.APIPrefix = "/prefix"
	perses.Spec.TLS = &persesv1alpha2.TLS{Enable: ptr.To(true)}
	perses.Spec.Client = &persesv1alpha2.Client{
		TLS: &persesv1alpha2.TLS{
			Enable:   ptr.To(true),
			CaCert:   &persesv1alpha2.Certificate{SecretSource: source, CertPath: "ca"},
			UserCert: &persesv1alpha2.Certificate{SecretSource: source, CertPath: "cert", PrivateKeyPath: ptr.To("key")},
		},
		OAuth: &persesv1alpha2.OAuth{
			SecretSource: source, ClientIDPath: ptr.To("id"), ClientSecretPath: ptr.To("secret"),
			TokenURL: tokenURL + factoryTokenPath, Scopes: []string{"read", "write"}, AuthStyle: ptr.To(authStyle),
			// v0.55.0 NewRESTClient ignores these; the targeted fix must not change that.
			EndpointParams: map[string][]string{"audience": {"not-forwarded-by-upstream"}},
		},
	}
	return perses, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "client", Namespace: "default"},
		Data:       map[string][]byte{"ca": ca.pem, "cert": certPEM, "key": keyPEM, "id": []byte("operator"), "secret": []byte("secret")},
	}
}

func factoryTokenHandler(t *testing.T, authStyle int32, calls *atomic.Int32) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, factoryTokenPath, r.URL.Path)
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		assert.Equal(t, "read write", r.Form.Get("scope"))
		assert.Empty(t, r.Form.Get("audience"), "preserve v0.55.0 EndpointParams behavior")
		if authStyle == 1 {
			assert.Empty(t, r.Header.Get("Authorization"))
			assert.Equal(t, "operator", r.Form.Get("client_id"))
			assert.Equal(t, "secret", r.Form.Get("client_secret"))
		} else {
			id, secret, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "operator", id)
			assert.Equal(t, "secret", secret)
			assert.Empty(t, r.Form.Get("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"perses-native-token","token_type":"Bearer","expires_in":3600}`)
	}
}

func factoryAPIHandler(t *testing.T, oauth bool, calls *atomic.Int32) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/prefix/api/v1/projects", r.URL.Path)
		if oauth {
			assert.Equal(t, "Bearer perses-native-token", r.Header.Get("Authorization"))
		} else {
			assert.Empty(t, r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, "[]")
	}
}

func TestCreateClientsForAllPodsOAuthTLS(t *testing.T) {
	for _, tc := range []struct {
		name, apiName, tokenName, wantHostname string
		authStyle                              int32
	}{
		{name: "separate_native_token_hostname", apiName: factoryServiceName, tokenName: "127.0.0.1", authStyle: 2},
		{name: "credentials_in_params", apiName: factoryServiceName, tokenName: "127.0.0.1", authStyle: 1},
		{name: "auto_auth_style", apiName: factoryServiceName, tokenName: "127.0.0.1"},
		{name: "wrong_API_certificate", apiName: "wrong.example", tokenName: "127.0.0.1", authStyle: 2, wantHostname: factoryServiceName},
		// A service certificate on the token endpoint must NOT pass verification of its IP URL.
		{name: "wrong_token_certificate", apiName: factoryServiceName, tokenName: factoryServiceName, authStyle: 2, wantHostname: "127.0.0.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ca := newFactoryTLSCA(t)
			var apiCalls, tokenCalls atomic.Int32
			api, apiClosed := factoryTLSServer(t, ca, tc.apiName, factoryAPIHandler(t, true, &apiCalls))
			token, tokenClosed := factoryTLSServer(t, ca, tc.tokenName, factoryTokenHandler(t, tc.authStyle, &tokenCalls))
			perses, credentials := factoryTLSPerses(t, ca, api.URL, token.URL, tc.authStyle)
			reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(credentials,
				readyPod("ready", "127.0.0.1", corev1.PodRunning, true)).Build()
			factory := NewWithConfig()
			t.Cleanup(func() { factory.ForgetInstance("default/perses-1") })
			clients, err := factory.CreateClientsForAllPods(t.Context(), reader, perses)
			require.NoError(t, err)
			require.Len(t, clients, 1)
			_, err = clients[0].Project().List("")
			if tc.wantHostname != "" {
				require.Error(t, err)
				var hostnameError x509.HostnameError
				require.ErrorAs(t, err, &hostnameError)
				assert.Equal(t, tc.wantHostname, hostnameError.Host)
				assert.Zero(t, apiCalls.Load())
				if tc.name == "wrong_token_certificate" {
					assert.Zero(t, tokenCalls.Load())
				} else {
					assert.EqualValues(t, 1, tokenCalls.Load())
				}
				return
			}
			require.NoError(t, err)
			cached, err := factory.CreateClientsForAllPods(t.Context(), reader, perses)
			require.NoError(t, err)
			require.Same(t, clients[0].RESTClient(), cached[0].RESTClient())
			_, err = cached[0].Project().List("")
			require.NoError(t, err)
			assert.EqualValues(t, 2, apiCalls.Load())
			assert.EqualValues(t, 1, tokenCalls.Load(), "cached clients reuse their token")

			factory.ForgetInstance("default/perses-1")
			for name, closed := range map[string]<-chan struct{}{"API": apiClosed, "token": tokenClosed} {
				select {
				case <-closed:
				case <-time.After(2 * time.Second):
					t.Errorf("cache eviction left %s idle connection open", name)
				}
			}
		})
	}
}

// Change only dialing, not URL hosts, SNI, roots, or certificate verification.
func routeFactoryTLS(t *testing.T, transport http.RoundTripper, address string) {
	t.Helper()
	switch rt := transport.(type) {
	case *podOAuthTransport:
		routeFactoryTLS(t, rt.Base, address)
		routeFactoryTLS(t, rt.tokenTransport, address)
	case *oauth2.Transport:
		routeFactoryTLS(t, rt.Base, address)
	case *http.Transport:
		rt.Proxy = nil
		rt.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, network, address)
		}
	default:
		t.Fatalf("unexpected factory transport %T", transport)
	}
}

func TestCreateClientsForAllPodsOAuthTLSServiceRoutes(t *testing.T) {
	for _, mode := range []string{"pod_service_native_token", "SQL", "server_URL_override"} {
		t.Run(mode, func(t *testing.T) {
			ca := newFactoryTLSCA(t)
			var apiCalls, tokenCalls atomic.Int32
			apiName := factoryServiceName
			if mode == "server_URL_override" {
				apiName = "127.0.0.1"
			}
			tokenHandler := factoryTokenHandler(t, 2, &tokenCalls)
			apiHandler := factoryAPIHandler(t, true, &apiCalls)
			api, _ := factoryTLSServer(t, ca, apiName, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == factoryTokenPath {
					tokenHandler(w, r)
				} else {
					apiHandler(w, r)
				}
			})
			serviceURL := strings.Replace(api.URL, "127.0.0.1", factoryServiceName, 1)
			tokenURL := serviceURL
			if mode == "server_URL_override" {
				tokenURL = api.URL
			}
			perses, credentials := factoryTLSPerses(t, ca, api.URL, tokenURL, 2)
			reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(credentials,
				readyPod("ready", "127.0.0.1", corev1.PodRunning, true)).Build()
			if mode == "SQL" {
				perses.Spec.Config.Database = config.Database{SQL: &config.SQL{}}
			}
			if mode == "server_URL_override" {
				if flag.Lookup(PersesServerURLFlag) == nil {
					flag.String(PersesServerURLFlag, "", "The Perses backend server URL")
				}
				previous := flag.Lookup(PersesServerURLFlag).Value.String()
				require.NoError(t, flag.Set(PersesServerURLFlag, api.URL+"/prefix"))
				t.Cleanup(func() { _ = flag.Set(PersesServerURLFlag, previous) })
			}
			factory := NewWithConfig()
			t.Cleanup(func() { factory.ForgetInstance("default/perses-1") })
			clients, err := factory.CreateClientsForAllPods(t.Context(), reader, perses)
			require.NoError(t, err)
			require.Len(t, clients, 1)
			rest := clients[0].RESTClient()
			if mode == "SQL" || mode == "server_URL_override" {
				serviceClient, err := factory.CreateClient(t.Context(), reader, perses)
				require.NoError(t, err)
				require.Same(t, serviceClient.RESTClient(), rest, "must retain the CreateClient path and cache")
			}
			_, port, err := net.SplitHostPort(api.Listener.Addr().String())
			require.NoError(t, err)
			routeFactoryTLS(t, rest.Client.Transport, net.JoinHostPort("127.0.0.1", port))
			_, err = clients[0].Project().List("")
			require.NoError(t, err)
			assert.EqualValues(t, 1, apiCalls.Load())
			assert.EqualValues(t, 1, tokenCalls.Load())
		})
	}
}

func TestCreateClientsForAllPodsTLSWithoutOAuth(t *testing.T) {
	ca := newFactoryTLSCA(t)
	var calls atomic.Int32
	api, _ := factoryTLSServer(t, ca, factoryServiceName, factoryAPIHandler(t, false, &calls))
	perses, credentials := factoryTLSPerses(t, ca, api.URL, "", 0)
	perses.Spec.Client.OAuth = nil
	reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(credentials,
		readyPod("ready", "127.0.0.1", corev1.PodRunning, true)).Build()
	factory := NewWithConfig()
	t.Cleanup(func() { factory.ForgetInstance("default/perses-1") })
	clients, err := factory.CreateClientsForAllPods(t.Context(), reader, perses)
	require.NoError(t, err)
	require.Len(t, clients, 1)
	_, err = clients[0].Project().List("")
	require.NoError(t, err)
	assert.EqualValues(t, 1, calls.Load())
}
