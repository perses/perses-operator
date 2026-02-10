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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	resourcesDesc = prometheus.NewDesc(
		"perses_operator_managed_resources",
		"Number of resources managed by the operator per state (synced/failed)",
		[]string{"resource", "state"},
		nil,
	)
)

// Metrics represents metrics associated with the operator.
type Metrics struct {
	// Reconciliation error counters
	reconcileErrors *prometheus.CounterVec

	// Perses instance metrics
	persesInstances *prometheus.GaugeVec

	// Operator readiness
	ready prometheus.Gauge

	// mtx protects all fields below
	mtx       sync.RWMutex
	resources map[resourceKey]map[string]int
}

type resourceKey struct {
	resource string
	state    resourceState
}

type resourceState int

const (
	synced resourceState = iota
	failed
)

func (r resourceState) String() string {
	switch r {
	case synced:
		return "synced"
	case failed:
		return "failed"
	}
	return ""
}

// NewMetrics initializes operator metrics and registers them with the controller-runtime registry.
func NewMetrics() *Metrics {
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
				Name: "perses_operator_managed_perses_instances",
				Help: "Number of Perses instances managed by the operator",
			},
			[]string{"resource_namespace"},
		),
		ready: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "perses_operator_ready",
				Help: "Whether the operator is ready (1=yes, 0=no)",
			},
		),
		resources: make(map[resourceKey]map[string]int),
	}

	// Register all metrics with controller-runtime metrics registry
	metrics.Registry.MustRegister(
		m.reconcileErrors,
		m.persesInstances,
		m.ready,
		m, // Register self as custom collector for managed_resources
	)

	return m
}

// ReconcileErrors returns a counter to track reconciliation errors.
func (m *Metrics) ReconcileErrors(controller, reason string) prometheus.Counter {
	return m.reconcileErrors.With(prometheus.Labels{"controller": controller, "reason": reason})
}

// PersesInstances returns a gauge to track Perses instance count.
func (m *Metrics) PersesInstances(namespace string) prometheus.Gauge {
	return m.persesInstances.With(prometheus.Labels{"resource_namespace": namespace})
}

// Ready returns a gauge to track operator readiness.
func (m *Metrics) Ready() prometheus.Gauge {
	return m.ready
}

// SetSyncedResources sets the number of resources that synced successfully for the given object's key.
func (m *Metrics) SetSyncedResources(objKey, resource string, v int) {
	m.setResources(objKey, resourceKey{resource: resource, state: synced}, v)
}

// SetFailedResources sets the number of resources that failed to sync for the given object's key.
func (m *Metrics) SetFailedResources(objKey, resource string, v int) {
	m.setResources(objKey, resourceKey{resource: resource, state: failed}, v)
}

func (m *Metrics) setResources(objKey string, resKey resourceKey, v int) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, found := m.resources[resKey]; !found {
		m.resources[resourceKey{resource: resKey.resource, state: synced}] = make(map[string]int)
		m.resources[resourceKey{resource: resKey.resource, state: failed}] = make(map[string]int)
	}

	m.resources[resKey][objKey] = v
}

// Describe implements the prometheus.Collector interface.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- resourcesDesc
}

// Collect implements the prometheus.Collector interface.
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	for rKey := range m.resources {
		var total int
		for _, v := range m.resources[rKey] {
			total += v
		}
		ch <- prometheus.MustNewConstMetric(
			resourcesDesc,
			prometheus.GaugeValue,
			float64(total),
			rKey.resource,
			rKey.state.String(),
		)
	}
}
