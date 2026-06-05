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
		reconcileOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "perses_operator_reconcile_operations_total",
				Help: "Total number of reconciliation operations by controller",
			},
			[]string{"controller"},
		),
		reconcileErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "perses_operator_reconcile_errors_total",
				Help: "Total number of reconciliation errors by controller and reason",
			},
			[]string{"controller", "reason"},
		),
		persesInstances: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "perses_operator_managed_perses_instances",
				Help: "Number of Perses instances managed by the operator",
			},
			[]string{"resource_namespace", "resource_name"},
		),
		ready: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "perses_operator_ready",
				Help: "Whether the operator is ready (1=yes, 0=no)",
			},
			[]string{"controller"},
		),
		resources: make(map[resourceKey]map[string]int),
	}

	reg.MustRegister(m.reconcileOperations, m.reconcileErrors, m.persesInstances, m.ready, m)

	// Set some values so metrics appear in output
	m.reconcileOperations.WithLabelValues("test").Add(1)
	m.reconcileErrors.WithLabelValues("test", "test_reason").Add(0)
	m.persesInstances.WithLabelValues("test-ns", "test-perses").Set(1)
	m.Ready("test").Set(1)

	// Verify metrics are registered
	metricFamilies, err := reg.Gather()
	assert.NoError(t, err)
	assert.NotEmpty(t, metricFamilies)

	// Check that expected metrics exist
	metricNames := []string{
		"perses_operator_reconcile_operations_total",
		"perses_operator_reconcile_errors_total",
		"perses_operator_managed_perses_instances",
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

func TestReconcileOperationsCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reconcileOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "perses_operator_reconcile_operations_total",
				Help: "Total number of reconciliation operations by controller",
			},
			[]string{"controller"},
		),
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m.reconcileOperations)

	// Increment operations counter
	m.ReconcileOperations("perses").Inc()
	m.ReconcileOperations("perses").Inc()
	m.ReconcileOperations("perses").Inc()
	m.ReconcileOperations("persesdashboard").Inc()

	// Verify counts
	expected := `
		# HELP perses_operator_reconcile_operations_total Total number of reconciliation operations by controller
		# TYPE perses_operator_reconcile_operations_total counter
		perses_operator_reconcile_operations_total{controller="perses"} 3
		perses_operator_reconcile_operations_total{controller="persesdashboard"} 1
	`
	err := testutil.CollectAndCompare(m.reconcileOperations, strings.NewReader(expected))
	assert.NoError(t, err)
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
				Name: "perses_operator_managed_perses_instances",
				Help: "Number of Perses instances managed by the operator",
			},
			[]string{"resource_namespace", "resource_name"},
		),
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m.persesInstances)

	// Set instance counts
	m.PersesInstances("perses-dev", "perses-1").Set(1)
	m.PersesInstances("production", "perses-prod").Set(1)

	// Verify values
	expected := `
		# HELP perses_operator_managed_perses_instances Number of Perses instances managed by the operator
		# TYPE perses_operator_managed_perses_instances gauge
		perses_operator_managed_perses_instances{resource_name="perses-1",resource_namespace="perses-dev"} 1
		perses_operator_managed_perses_instances{resource_name="perses-prod",resource_namespace="production"} 1
	`
	err := testutil.CollectAndCompare(m.persesInstances, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestReadyGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		ready:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "perses_operator_ready"}, []string{"controller"}),
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m.ready)

	// Initially not ready
	m.Ready("perses").Set(0)
	value := testutil.ToFloat64(m.ready.WithLabelValues("perses"))
	assert.Equal(t, 0.0, value)

	// Mark as ready
	m.Ready("perses").Set(1)
	value = testutil.ToFloat64(m.ready.WithLabelValues("perses"))
	assert.Equal(t, 1.0, value)

	// Each controller tracks independently
	m.Ready("persesdashboard").Set(0)
	assert.Equal(t, 1.0, testutil.ToFloat64(m.ready.WithLabelValues("perses")))
	assert.Equal(t, 0.0, testutil.ToFloat64(m.ready.WithLabelValues("persesdashboard")))
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

func TestDeletePersesInstance(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		persesInstances: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "perses_operator_managed_perses_instances",
				Help: "Number of Perses instances managed by the operator",
			},
			[]string{"resource_namespace", "resource_name"},
		),
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m.persesInstances)

	// Set instances for two namespaces
	m.PersesInstances("perses-dev", "perses-1").Set(1)
	m.PersesInstances("production", "perses-prod").Set(1)

	// Delete one instance
	m.DeletePersesInstance("perses-dev", "perses-1")

	// Verify only production remains
	expected := `
		# HELP perses_operator_managed_perses_instances Number of Perses instances managed by the operator
		# TYPE perses_operator_managed_perses_instances gauge
		perses_operator_managed_perses_instances{resource_name="perses-prod",resource_namespace="production"} 1
	`
	err := testutil.CollectAndCompare(m.persesInstances, strings.NewReader(expected))
	assert.NoError(t, err)

	// Deleting a non-existent instance should not panic
	m.DeletePersesInstance("nonexistent", "nonexistent")
}

func TestForgetObject(t *testing.T) {
	m := &Metrics{
		resources: make(map[resourceKey]map[string]int),
	}

	// Set synced and failed entries for multiple objects
	m.SetSyncedResources("ns1/resource1", "dashboard", 1)
	m.SetSyncedResources("ns1/resource2", "dashboard", 1)
	m.SetFailedResources("ns1/resource1", "dashboard", 1)
	m.SetSyncedResources("ns2/resource3", "datasource", 1)

	// Forget resource1
	m.ForgetObject("ns1/resource1")

	// Verify resource1 is removed from both synced and failed maps
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	syncedDashboard := resourceKey{resource: "dashboard", state: synced}
	failedDashboard := resourceKey{resource: "dashboard", state: failed}
	syncedDatasource := resourceKey{resource: "datasource", state: synced}

	assert.Equal(t, 1, len(m.resources[syncedDashboard]), "Should have 1 synced dashboard after forget")
	assert.Equal(t, 1, len(m.resources[syncedDatasource]), "Datasource should be unaffected")

	// Empty outer map entries should be removed
	_, outerExists := m.resources[failedDashboard]
	assert.False(t, outerExists, "Empty outer map entry should be removed")

	// resource2 should still exist
	assert.Equal(t, 1, m.resources[syncedDashboard]["ns1/resource2"])
	// resource1 should be gone
	_, exists := m.resources[syncedDashboard]["ns1/resource1"]
	assert.False(t, exists, "resource1 should be removed from synced map")
}

func TestForgetObjectCollectOutput(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		resources: make(map[resourceKey]map[string]int),
	}
	reg.MustRegister(m)

	m.SetSyncedResources("ns1/resource1", "dashboard", 1)
	m.SetSyncedResources("ns1/resource2", "dashboard", 1)
	m.SetFailedResources("ns1/resource3", "dashboard", 1)

	// Forget resource1 and resource3
	m.ForgetObject("ns1/resource1")
	m.ForgetObject("ns1/resource3")

	// Collect and verify totals — the failed entry should disappear entirely
	// since no objects remain in that category
	expected := `
		# HELP perses_operator_managed_resources Number of resources managed by the operator per state (synced/failed)
		# TYPE perses_operator_managed_resources gauge
		perses_operator_managed_resources{resource="dashboard",state="synced"} 1
	`
	err := testutil.CollectAndCompare(m, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestForgetObjectConcurrency(t *testing.T) {
	m := &Metrics{
		resources: make(map[resourceKey]map[string]int),
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "test/resource"
			m.SetSyncedResources(key, "dashboard", id)
			m.SetFailedResources(key, "datasource", id)
			m.ForgetObject(key)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	m.mtx.RLock()
	defer m.mtx.RUnlock()
	assert.NotNil(t, m.resources)
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
