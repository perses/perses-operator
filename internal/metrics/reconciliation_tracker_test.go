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
	"errors"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestReconciliationTrackerSetStatus(t *testing.T) {
	rt := &ReconciliationTracker{}

	// Set successful status
	rt.SetStatus("ns1/resource1", nil)

	status, found := rt.GetStatus("ns1/resource1")
	assert.True(t, found)
	assert.True(t, status.Ok())
	assert.NoError(t, status.err)

	// Set failed status
	testErr := errors.New("reconciliation failed")
	rt.SetStatus("ns1/resource2", testErr)

	status, found = rt.GetStatus("ns1/resource2")
	assert.True(t, found)
	assert.False(t, status.Ok())
	assert.Equal(t, testErr, status.err)
}

func TestReconciliationTrackerSetReasonAndMessage(t *testing.T) {
	rt := &ReconciliationTracker{}

	rt.SetStatus("ns1/resource1", nil)
	rt.SetReasonAndMessage("ns1/resource1", "TestReason", "Test message")

	status, found := rt.GetStatus("ns1/resource1")
	assert.True(t, found)
	assert.Equal(t, "TestReason", status.Reason())
	assert.Equal(t, "Test message", status.Message())
}

func TestReconciliationTrackerForgetObject(t *testing.T) {
	rt := &ReconciliationTracker{}

	// Add status for an object
	rt.SetStatus("ns1/resource1", nil)
	_, found := rt.GetStatus("ns1/resource1")
	assert.True(t, found)

	// Forget the object
	rt.ForgetObject("ns1/resource1")

	// Should not be found
	_, found = rt.GetStatus("ns1/resource1")
	assert.False(t, found)
}

func TestReconciliationTrackerGetStatusUnknown(t *testing.T) {
	rt := &ReconciliationTracker{}

	status, found := rt.GetStatus("unknown/resource")
	assert.False(t, found)
	assert.True(t, status.Ok()) // Default zero value
}

func TestReconciliationStatusReason(t *testing.T) {
	tests := []struct {
		name           string
		status         ReconciliationStatus
		expectedReason string
	}{
		{
			name: "successful reconciliation with custom reason",
			status: ReconciliationStatus{
				err:    nil,
				reason: "CustomReason",
			},
			expectedReason: "CustomReason",
		},
		{
			name: "failed reconciliation",
			status: ReconciliationStatus{
				err: errors.New("test error"),
			},
			expectedReason: "ReconciliationFailed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedReason, tt.status.Reason())
		})
	}
}

func TestReconciliationStatusMessage(t *testing.T) {
	tests := []struct {
		name            string
		status          ReconciliationStatus
		expectedMessage string
	}{
		{
			name: "successful with custom message",
			status: ReconciliationStatus{
				err:     nil,
				message: "All good",
			},
			expectedMessage: "All good",
		},
		{
			name: "failed with error",
			status: ReconciliationStatus{
				err: errors.New("reconciliation error"),
			},
			expectedMessage: "reconciliation error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedMessage, tt.status.Message())
		})
	}
}

func TestReconciliationTrackerCollectMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	rt := &ReconciliationTracker{}
	reg.MustRegister(rt)

	// Add mixed successful and failed reconciliations
	rt.SetStatus("ns1/resource1", nil)
	rt.SetStatus("ns1/resource2", nil)
	rt.SetStatus("ns1/resource3", errors.New("failed"))
	rt.SetStatus("ns2/resource1", errors.New("failed"))

	// Verify metrics collection
	expected := `
		# HELP perses_operator_syncs Number of objects per sync status (ok/failed)
		# TYPE perses_operator_syncs gauge
		perses_operator_syncs{status="ok"} 2
		perses_operator_syncs{status="failed"} 2
	`
	err := testutil.CollectAndCompare(rt, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestReconciliationTrackerCollectEmpty(t *testing.T) {
	reg := prometheus.NewRegistry()
	rt := &ReconciliationTracker{}
	reg.MustRegister(rt)

	// Collect with no data
	expected := `
		# HELP perses_operator_syncs Number of objects per sync status (ok/failed)
		# TYPE perses_operator_syncs gauge
		perses_operator_syncs{status="ok"} 0
		perses_operator_syncs{status="failed"} 0
	`
	err := testutil.CollectAndCompare(rt, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestReconciliationTrackerConcurrency(t *testing.T) {
	rt := &ReconciliationTracker{}

	// Simulate concurrent reconciliations
	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func(id int) {
			key := "test/resource"
			if id%2 == 0 {
				rt.SetStatus(key, nil)
			} else {
				rt.SetStatus(key, errors.New("test error"))
			}
			rt.GetStatus(key)
			rt.SetReasonAndMessage(key, "reason", "message")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 50; i++ {
		<-done
	}

	// Verify tracker still works (no race conditions)
	_, found := rt.GetStatus("test/resource")
	assert.True(t, found)
}

func TestNewReconciliationTracker(t *testing.T) {
	reg := prometheus.NewRegistry()
	rt := NewReconciliationTracker(reg)

	assert.NotNil(t, rt)

	// Verify it's registered by trying to gather metrics
	metricFamilies, err := reg.Gather()
	assert.NoError(t, err)

	// Should have the syncs metric
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "perses_operator_syncs" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected perses_operator_syncs metric to be registered")
}
