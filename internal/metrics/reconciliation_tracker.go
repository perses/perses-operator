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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	syncsDesc = prometheus.NewDesc(
		"perses_operator_syncs",
		"Number of objects per sync status (ok/failed)",
		[]string{"status"},
		nil,
	)
)

// ReconciliationStatus represents the status of a reconciliation operation.
type ReconciliationStatus struct {
	err     error
	reason  string
	message string
}

// Reason returns the reconciliation reason.
func (rs ReconciliationStatus) Reason() string {
	if rs.Ok() {
		return rs.reason
	}
	return "ReconciliationFailed"
}

// Message returns the reconciliation message.
func (rs ReconciliationStatus) Message() string {
	if rs.Ok() {
		return rs.message
	}
	return rs.err.Error()
}

// Ok returns true if the reconciliation was successful.
func (rs ReconciliationStatus) Ok() bool {
	return rs.err == nil
}

// ReconciliationTracker tracks reconciliation status per object.
//
// It uses the namespace/name key to identify objects.
type ReconciliationTracker struct {
	once sync.Once

	// mtx protects all fields below.
	mtx            sync.RWMutex
	statusByObject map[string]ReconciliationStatus
}

func (rt *ReconciliationTracker) init() {
	rt.once.Do(func() {
		rt.statusByObject = map[string]ReconciliationStatus{}
	})
}

// SetStatus updates the last reconciliation status for the object identified by key.
func (rt *ReconciliationTracker) SetStatus(key string, err error) {
	rt.init()
	rt.mtx.Lock()
	defer rt.mtx.Unlock()

	rs := rt.statusByObject[key]
	rs.err = err
	rt.statusByObject[key] = rs
}

// SetReasonAndMessage updates the reason and message for the object identified by key.
// The reason and message are only used when the reconciliation returned no error.
func (rt *ReconciliationTracker) SetReasonAndMessage(key string, reason, message string) {
	rt.init()
	rt.mtx.Lock()
	defer rt.mtx.Unlock()

	rs := rt.statusByObject[key]
	rs.reason = reason
	rs.message = message
	rt.statusByObject[key] = rs
}

// GetStatus returns the last reconciliation status for the given object.
// The second value indicates whether the object is known or not.
func (rt *ReconciliationTracker) GetStatus(k string) (ReconciliationStatus, bool) {
	rt.mtx.RLock()
	defer rt.mtx.RUnlock()

	s, found := rt.statusByObject[k]
	if !found {
		return ReconciliationStatus{}, false
	}

	return s, true
}

// ForgetObject removes the given object from the tracker.
// It should be called when the controller detects that the object has been deleted.
func (rt *ReconciliationTracker) ForgetObject(key string) {
	rt.mtx.Lock()
	defer rt.mtx.Unlock()

	if rt.statusByObject == nil {
		return
	}

	delete(rt.statusByObject, key)
}

// Describe implements the prometheus.Collector interface.
func (rt *ReconciliationTracker) Describe(ch chan<- *prometheus.Desc) {
	ch <- syncsDesc
}

// Collect implements the prometheus.Collector interface.
func (rt *ReconciliationTracker) Collect(ch chan<- prometheus.Metric) {
	rt.mtx.RLock()
	defer rt.mtx.RUnlock()

	var ok, failed float64
	for _, st := range rt.statusByObject {
		if st.Ok() {
			ok++
		} else {
			failed++
		}
	}

	ch <- prometheus.MustNewConstMetric(
		syncsDesc,
		prometheus.GaugeValue,
		ok,
		"ok",
	)
	ch <- prometheus.MustNewConstMetric(
		syncsDesc,
		prometheus.GaugeValue,
		failed,
		"failed",
	)
}

// NewReconciliationTracker creates a new ReconciliationTracker and registers it with the metrics registry.
func NewReconciliationTracker(reg prometheus.Registerer) *ReconciliationTracker {
	rt := &ReconciliationTracker{}
	if reg != nil {
		reg.MustRegister(rt)
	}
	return rt
}
