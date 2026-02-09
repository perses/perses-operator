/*
Copyright The Perses Authors.

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

package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	// Create a new registry to avoid conflicts with global registry
	reg := prometheus.NewRegistry()

	// Create metrics but register with our test registry instead of global
	m := &Metrics{
		reconcileErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "perses_operator_reconcile_errors_total",
				Help: "Total number of reconciliation errors by controller and reason",
			},
			[]string{"controller", "reason"},
		),
		persesInstances: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "perses_operator_perses_instances",
				Help: "Number of Perses instances per namespace",
			},
			[]string{"namespace"},
		),
		ready: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "perses_operator_ready",
				Help: "Whether the operator is ready (1=yes, 0=no)",
			},
		),
		resources: make(map[resourceKey]map[string]int),
	}

	reg.MustRegister(m.reconcileErrors, m.persesInstances, m.ready, m)

	// Set some values so metrics appear in output
	m.reconcileErrors.WithLabelValues("test", "test_reason").Add(0)
	m.persesInstances.WithLabelValues("test-ns").Set(1)
	m.ready.Set(1)

	// Verify metrics are registered
	metricFamilies, err := reg.Gather()
	assert.NoError(t, err)
	assert.NotEmpty(t, metricFamilies)

	// Check that expected metrics exist
	metricNames := []string{
		"perses_operator_reconcile_errors_total",
		"perses_operator_perses_instances",
		"perses_operator_ready",
	}

	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamilies {
		foundMetrics[mf.GetName()] = true
	}

	for _, name := range metricNames {
		assert.True(t, foundMetrics[name], "Expected metric %s to be registered", name)
	}
}

func TestReconcileErrorsCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reconcileErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "perses_operator_reconcile_errors_total",
				Help: "Total number of reconciliation errors",
			},
			[]string{"controller", "reason"},
		),
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m.reconcileErrors)

	// Increment error counter
	m.ReconcileErrors("perses", "get_failed").Inc()
	m.ReconcileErrors("perses", "get_failed").Inc()
	m.ReconcileErrors("persesdashboard", "reconciliation_failed").Inc()

	// Verify counts
	expected := `
		# HELP perses_operator_reconcile_errors_total Total number of reconciliation errors
		# TYPE perses_operator_reconcile_errors_total counter
		perses_operator_reconcile_errors_total{controller="perses",reason="get_failed"} 2
		perses_operator_reconcile_errors_total{controller="persesdashboard",reason="reconciliation_failed"} 1
	`
	err := testutil.CollectAndCompare(m.reconcileErrors, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestPersesInstancesGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		persesInstances: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "perses_operator_perses_instances",
				Help: "Number of Perses instances per namespace",
			},
			[]string{"namespace"},
		),
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m.persesInstances)

	// Set instance counts
	m.PersesInstances("perses-dev").Set(1)
	m.PersesInstances("production").Set(3)

	// Verify values
	expected := `
		# HELP perses_operator_perses_instances Number of Perses instances per namespace
		# TYPE perses_operator_perses_instances gauge
		perses_operator_perses_instances{namespace="perses-dev"} 1
		perses_operator_perses_instances{namespace="production"} 3
	`
	err := testutil.CollectAndCompare(m.persesInstances, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestReadyGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		ready:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "perses_operator_ready"}),
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m.ready)

	// Initially not ready
	m.Ready().Set(0)
	value := testutil.ToFloat64(m.ready)
	assert.Equal(t, 0.0, value)

	// Mark as ready
	m.Ready().Set(1)
	value = testutil.ToFloat64(m.ready)
	assert.Equal(t, 1.0, value)
}

func TestManagedResourcesCollector(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m)

	// Set synced and failed resources
	m.SetSyncedResources("perses-dev/perses-sample", "perses", 1)
	m.SetSyncedResources("perses-dev/dashboard-1", "dashboard", 1)
	m.SetFailedResources("perses-dev/dashboard-2", "dashboard", 1)

	// Collect metrics
	metricCh := make(chan prometheus.Metric, 10)
	m.Collect(metricCh)
	close(metricCh)

	// Verify we got metrics
	count := 0
	for metric := range metricCh {
		count++
		// Verify metric is valid
		assert.NotNil(t, metric)
	}

	// Should have metrics for: perses-synced, dashboard-synced, dashboard-failed
	assert.GreaterOrEqual(t, count, 2, "Expected at least 2 managed resource metrics")
}

func TestSetSyncedAndFailedResources(t *testing.T) {
	m := &Metrics{
		resources: make(map[resourceKey]map[string]int),
	}

	// Set multiple resources
	m.SetSyncedResources("ns1/resource1", "dashboard", 1)
	m.SetSyncedResources("ns1/resource2", "dashboard", 1)
	m.SetFailedResources("ns1/resource3", "dashboard", 1)

	// Verify internal state
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	syncedKey := resourceKey{resource: "dashboard", state: synced}
	failedKey := resourceKey{resource: "dashboard", state: failed}

	assert.Len(t, m.resources[syncedKey], 2, "Should have 2 synced resources")
	assert.Len(t, m.resources[failedKey], 1, "Should have 1 failed resource")
}

func TestResourceStateString(t *testing.T) {
	tests := []struct {
		state    resourceState
		expected string
	}{
		{synced, "synced"},
		{failed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestMetricsConcurrency(t *testing.T) {
	m := &Metrics{
		resources: make(map[resourceKey]map[string]int),
	}

	// Simulate concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			m.SetSyncedResources("test/resource", "dashboard", id)
			m.SetFailedResources("test/resource2", "datasource", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no race conditions (test will fail if there are data races)
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	assert.NotNil(t, m.resources)
}
