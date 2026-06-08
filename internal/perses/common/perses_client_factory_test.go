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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	persesv1alpha2 "github.com/rhobs/perses-operator/api/v1alpha2"
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
