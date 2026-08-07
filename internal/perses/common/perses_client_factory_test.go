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
	"flag"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/perses/perses/pkg/model/api/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
)

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

func newPersesForPods() persesv1alpha2.Perses {
	return persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "perses-1",
			Namespace: "default",
		},
		Spec: persesv1alpha2.PersesSpec{
			ContainerPort: ptr.To(int32(8080)),
		},
	}
}

// readyPod builds a pod carrying the operator's selector labels, with a
// configurable phase, IP, and Ready condition.
func readyPod(name, ip string, phase corev1.PodPhase, ready bool) *corev1.Pod {
	perses := newPersesForPods()
	condStatus := corev1.ConditionFalse
	if ready {
		condStatus = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: perses.Namespace,
			Labels:    LabelsForPerses(perses.Name, &perses),
		},
		Status: corev1.PodStatus{
			Phase: phase,
			PodIP: ip,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: condStatus},
			},
		},
	}
}

func TestCreateClientsForAllPods(t *testing.T) {
	ctx := context.Background()
	perses := newPersesForPods()

	t.Run("returns a client only for ready pods", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(
			readyPod("ready-1", "10.0.0.1", corev1.PodRunning, true),
			readyPod("ready-2", "10.0.0.2", corev1.PodRunning, true),
			readyPod("pending", "10.0.0.3", corev1.PodPending, true),
			readyPod("no-ip", "", corev1.PodRunning, true),
			readyPod("not-ready", "10.0.0.4", corev1.PodRunning, false),
		).Build()

		clients, err := NewWithConfig().CreateClientsForAllPods(ctx, reader, perses)
		require.NoError(t, err)
		require.Len(t, clients, 2, "only the two running, IP-assigned, ready pods should yield clients")

		got := []string{
			clients[0].RESTClient().BaseURL.String(),
			clients[1].RESTClient().BaseURL.String(),
		}
		assert.ElementsMatch(t, []string{
			"http://10.0.0.1:8080",
			"http://10.0.0.2:8080",
		}, got)
	})

	t.Run("errors when no pods are ready", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(
			readyPod("not-ready", "10.0.0.4", corev1.PodRunning, false),
		).Build()

		clients, err := NewWithConfig().CreateClientsForAllPods(ctx, reader, perses)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no ready pods found")
		assert.Nil(t, clients)
	})

	t.Run("brackets IPv6 pod IPs", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(
			readyPod("ready-v6", "fd00::1", corev1.PodRunning, true),
		).Build()

		clients, err := NewWithConfig().CreateClientsForAllPods(ctx, reader, perses)
		require.NoError(t, err)
		require.Len(t, clients, 1)
		assert.Equal(t, "http://[fd00::1]:8080", clients[0].RESTClient().BaseURL.String())
	})

	t.Run("pins TLS ServerName to the service FQDN for pod-direct HTTPS", func(t *testing.T) {
		tlsPerses := newPersesForPods()
		tlsPerses.Spec.TLS = &persesv1alpha2.TLS{Enable: ptr.To(true)}

		reader := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(
			readyPod("ready-1", "10.0.0.1", corev1.PodRunning, true),
		).Build()

		clients, err := NewWithConfig().CreateClientsForAllPods(ctx, reader, tlsPerses)
		require.NoError(t, err)
		require.Len(t, clients, 1)

		// The client dials the pod IP but must verify against the service DNS name.
		assert.Equal(t, "https://10.0.0.1:8080", clients[0].RESTClient().BaseURL.String())
		transport, ok := clients[0].RESTClient().Client.Transport.(*http.Transport)
		require.True(t, ok, "expected an *http.Transport")
		require.NotNil(t, transport.TLSClientConfig)
		assert.Equal(t, "perses-1.default.svc.cluster.local", transport.TLSClientConfig.ServerName)
	})

	t.Run("SQL mode returns a single service client", func(t *testing.T) {
		// A configured server URL flag makes the service client deterministic
		// and independent of cluster DNS. The flag is normally registered by
		// main.go, so register it here if the test binary hasn't.
		if flag.Lookup(PersesServerURLFlag) == nil {
			flag.String(PersesServerURLFlag, "", "The Perses backend server URL")
		}
		require.NoError(t, flag.Set(PersesServerURLFlag, "http://perses.example:8080"))
		t.Cleanup(func() { _ = flag.Set(PersesServerURLFlag, "") })

		sqlPerses := newPersesForPods()
		sqlPerses.Spec.Config.Database = config.Database{SQL: &config.SQL{}}

		// No pods exist: SQL mode must not list pods, so this still succeeds.
		reader := fake.NewClientBuilder().WithScheme(newScheme()).Build()

		clients, err := NewWithConfig().CreateClientsForAllPods(ctx, reader, sqlPerses)
		require.NoError(t, err)
		require.Len(t, clients, 1)
		assert.Equal(t, "http://perses.example:8080", clients[0].RESTClient().BaseURL.String())
	})
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{"running, ready, with IP", readyPod("p", "10.0.0.1", corev1.PodRunning, true), true},
		{"not running", readyPod("p", "10.0.0.1", corev1.PodPending, true), false},
		{"no IP", readyPod("p", "", corev1.PodRunning, true), false},
		{"ready condition false", readyPod("p", "10.0.0.1", corev1.PodRunning, false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPodReady(tt.pod))
		})
	}
}
