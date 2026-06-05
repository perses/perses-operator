// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/rhobs/perses/pkg/client/api/v1"
	clientConfig "github.com/rhobs/perses/pkg/client/config"
	"github.com/rhobs/perses/pkg/model/api/v1/common"
	"github.com/rhobs/perses/pkg/model/api/v1/secret"

	persesv1alpha2 "github.com/rhobs/perses-operator/api/v1alpha2"
)

const tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

type PersesClientFactory interface {
	CreateClient(ctx context.Context, client client.Reader, perses persesv1alpha2.Perses) (v1.ClientInterface, error)
}

type PersesClientCacheInvalidator interface {
	ForgetInstance(key string)
}

const defaultClientCacheTTL = 5 * time.Minute

type clientCacheEntry struct {
	client      v1.ClientInterface
	fingerprint string
	createdAt   time.Time
}

type PersesClientFactoryWithConfig struct {
	mtx       sync.RWMutex
	cache     map[string]clientCacheEntry
	ttl       time.Duration
	lastSweep time.Time
}

func NewWithConfig() *PersesClientFactoryWithConfig {
	return &PersesClientFactoryWithConfig{
		cache: make(map[string]clientCacheEntry),
		ttl:   defaultClientCacheTTL,
	}
}

// ForgetInstance removes the cached client for the given Perses instance key.
// The key format is "namespace/name".
func (f *PersesClientFactoryWithConfig) ForgetInstance(key string) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	if entry, exists := f.cache[key]; exists {
		closeClientConnections(entry.client)
	}
	delete(f.cache, key)
}

// sweepExpiredLocked removes cache entries whose TTL has elapsed.
// It runs at most once per TTL interval to amortise the iteration cost.
// Caller must hold f.mtx write lock.
func (f *PersesClientFactoryWithConfig) sweepExpiredLocked() {
	now := time.Now()
	if now.Sub(f.lastSweep) < f.ttl {
		return
	}
	for key, entry := range f.cache {
		if now.Sub(entry.createdAt) >= f.ttl {
			closeClientConnections(entry.client)
			delete(f.cache, key)
		}
	}
	f.lastSweep = now
}

func closeClientConnections(c v1.ClientInterface) {
	if c == nil {
		return
	}
	if rest := c.RESTClient(); rest != nil && rest.Client != nil {
		rest.Client.CloseIdleConnections()
	}
}

func configFingerprint(perses persesv1alpha2.Perses) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ns=%s,name=%s,gen=%d,", perses.Namespace, perses.Name, perses.Generation)
	if perses.Spec.ContainerPort != nil {
		fmt.Fprintf(&b, "port=%d,", *perses.Spec.ContainerPort)
	}
	fmt.Fprintf(&b, "apiPrefix=%s,", perses.Spec.Config.APIPrefix)
	fmt.Fprintf(&b, "tls=%v,", isTLSEnabled(&perses))
	fmt.Fprintf(&b, "k8sAuth=%v,", isKubernetesAuthEnabled(&perses))
	fmt.Fprintf(&b, "clientTLS=%v,", isClientTLSEnabled(&perses))
	serverURLFlag := flag.Lookup(PersesServerURLFlag)
	if serverURLFlag != nil {
		fmt.Fprintf(&b, "serverURL=%s,", serverURLFlag.Value.String())
	}
	return b.String()
}

// cacheHit returns the cached client if it exists, has the matching fingerprint,
// and has not expired. Caller must hold at least RLock.
func (f *PersesClientFactoryWithConfig) cacheHit(instanceKey, fingerprint string) (v1.ClientInterface, bool) {
	entry, ok := f.cache[instanceKey]
	if !ok {
		return nil, false
	}
	if entry.fingerprint != fingerprint {
		return nil, false
	}
	if time.Since(entry.createdAt) >= f.ttl {
		return nil, false
	}
	return entry.client, true
}

func (f *PersesClientFactoryWithConfig) CreateClient(ctx context.Context, client client.Reader, perses persesv1alpha2.Perses) (v1.ClientInterface, error) {
	instanceKey := fmt.Sprintf("%s/%s", perses.Namespace, perses.Name)
	fingerprint := configFingerprint(perses)

	// Check cache under read lock — no file I/O needed.
	// The TTL bounds credential staleness (SA token, TLS certs) so the
	// fingerprint only tracks config shape (Generation, port, TLS flags).
	f.mtx.RLock()
	if cached, ok := f.cacheHit(instanceKey, fingerprint); ok {
		f.mtx.RUnlock()
		return cached, nil
	}
	f.mtx.RUnlock()

	// Build the client without holding any lock to avoid blocking other
	// controllers during slow operations (TLS cert fetches from API server).
	newClient, err := f.buildClient(ctx, client, perses)
	if err != nil {
		return nil, err
	}

	// Store in cache under write lock. Re-check in case another goroutine raced.
	f.mtx.Lock()
	if cached, ok := f.cacheHit(instanceKey, fingerprint); ok {
		f.mtx.Unlock()
		return cached, nil
	}
	if old, exists := f.cache[instanceKey]; exists {
		closeClientConnections(old.client)
	}
	f.cache[instanceKey] = clientCacheEntry{
		client:      newClient,
		fingerprint: fingerprint,
		createdAt:   time.Now(),
	}
	f.sweepExpiredLocked()
	f.mtx.Unlock()

	return newClient, nil
}

func (f *PersesClientFactoryWithConfig) buildClient(ctx context.Context, client client.Reader, perses persesv1alpha2.Perses) (v1.ClientInterface, error) {
	var urlStr string

	var httpProtocol = "http"
	if isTLSEnabled(&perses) {
		httpProtocol = "https"
	}

	serverURLFlag := flag.Lookup(PersesServerURLFlag)
	if serverURLFlag != nil && serverURLFlag.Value.String() != "" {
		urlStr = serverURLFlag.Value.String()
	} else {
		containerPort := DefaultContainerPort
		if perses.Spec.ContainerPort != nil {
			containerPort = *perses.Spec.ContainerPort
		}
		urlStr = fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d%s", httpProtocol, perses.Name, perses.Namespace, containerPort, perses.Spec.Config.APIPrefix)
	}
	parsedURL, err := common.ParseURL(urlStr)
	if err != nil {
		return nil, err
	}

	config := clientConfig.RestConfigClient{
		URL: parsedURL,
	}

	if isKubernetesAuthEnabled(&perses) {
		tokenBytes, err := os.ReadFile(tokenPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read service account token from %s: %w", tokenPath, err)
		}
		saToken := string(tokenBytes)
		if saToken == "" {
			return nil, fmt.Errorf("service account token is empty, ensure the Perses operator has the correct permissions")
		}
		config.Headers = map[string]string{
			"Authorization": "Bearer " + saToken,
		}
	}

	if isClientTLSEnabled(&perses) {
		tls := perses.Spec.Client.TLS

		insecureSkipVerify := false
		if tls.InsecureSkipVerify != nil {
			insecureSkipVerify = *tls.InsecureSkipVerify
		}

		tlsConfig := &secret.TLSConfig{
			InsecureSkipVerify: insecureSkipVerify,
		}

		if tls.CaCert != nil {
			switch tls.CaCert.Type {
			case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
				caData, _, err := GetTLSCertData(ctx, client, perses.Namespace, perses.Name, tls.CaCert)

				if err != nil {
					return nil, err
				}

				tlsConfig.CA = caData
			case persesv1alpha2.SecretSourceTypeFile:
				tlsConfig.CAFile = tls.CaCert.CertPath
			}
		}

		if tls.UserCert != nil {
			switch tls.UserCert.Type {
			case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
				cert, key, err := GetTLSCertData(ctx, client, perses.Namespace, perses.Name, tls.UserCert)

				if err != nil {
					return nil, err
				}

				tlsConfig.Cert = cert
				tlsConfig.Key = key
			case persesv1alpha2.SecretSourceTypeFile:
				tlsConfig.CertFile = tls.UserCert.CertPath
			}
		}

		config.TLSConfig = tlsConfig
	}

	restClient, err := clientConfig.NewRESTClient(config)
	if err != nil {
		return nil, err
	}

	return v1.NewWithClient(restClient), nil
}

type PersesClientFactoryWithClient struct {
	client v1.ClientInterface
}

func NewWithClient(client v1.ClientInterface) PersesClientFactory {
	return &PersesClientFactoryWithClient{client: client}
}

func (f *PersesClientFactoryWithClient) CreateClient(_ context.Context, _ client.Reader, _ persesv1alpha2.Perses) (v1.ClientInterface, error) {
	return f.client, nil
}
