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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

func makeCondition(typ string, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               typ,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
}

func TestConditionsChanged(t *testing.T) {
	tests := []struct {
		name    string
		before  []metav1.Condition
		after   []metav1.Condition
		changed bool
	}{
		{
			name:    "both empty",
			before:  []metav1.Condition{},
			after:   []metav1.Condition{},
			changed: false,
		},
		{
			name:    "both nil",
			before:  nil,
			after:   nil,
			changed: false,
		},
		{
			name:    "identical single condition",
			before:  []metav1.Condition{makeCondition("Available", metav1.ConditionTrue, "Reconciled", "ok")},
			after:   []metav1.Condition{makeCondition("Available", metav1.ConditionTrue, "Reconciled", "ok")},
			changed: false,
		},
		{
			name:    "different status",
			before:  []metav1.Condition{makeCondition("Available", metav1.ConditionFalse, "Reconciled", "ok")},
			after:   []metav1.Condition{makeCondition("Available", metav1.ConditionTrue, "Reconciled", "ok")},
			changed: true,
		},
		{
			name:    "different reason",
			before:  []metav1.Condition{makeCondition("Available", metav1.ConditionTrue, "Reconciling", "ok")},
			after:   []metav1.Condition{makeCondition("Available", metav1.ConditionTrue, "Reconciled", "ok")},
			changed: true,
		},
		{
			name:    "different message",
			before:  []metav1.Condition{makeCondition("Available", metav1.ConditionTrue, "Reconciled", "starting")},
			after:   []metav1.Condition{makeCondition("Available", metav1.ConditionTrue, "Reconciled", "done")},
			changed: true,
		},
		{
			name:   "condition added",
			before: []metav1.Condition{makeCondition("Available", metav1.ConditionTrue, "Reconciled", "ok")},
			after: []metav1.Condition{
				makeCondition("Available", metav1.ConditionTrue, "Reconciled", "ok"),
				makeCondition("Degraded", metav1.ConditionFalse, "Reconciled", "ok"),
			},
			changed: true,
		},
		{
			name:    "empty to non-empty",
			before:  []metav1.Condition{},
			after:   []metav1.Condition{makeCondition("Available", metav1.ConditionUnknown, "Reconciling", "starting")},
			changed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.changed, ConditionsChanged(tt.before, tt.after))
		})
	}
}

func TestSnapshotConditions(t *testing.T) {
	original := []metav1.Condition{
		makeCondition("Available", metav1.ConditionTrue, "Reconciled", "ok"),
		makeCondition("Degraded", metav1.ConditionFalse, "Reconciled", "ok"),
	}

	snapshot := SnapshotConditions(original)

	assert.Equal(t, original, snapshot)
	assert.False(t, ConditionsChanged(snapshot, original))

	// Mutating the original via SetStatusCondition should not affect the snapshot
	meta.SetStatusCondition(&original, metav1.Condition{
		Type:   "Available",
		Status: metav1.ConditionFalse,
		Reason: "Error",
	})

	assert.True(t, ConditionsChanged(snapshot, original),
		"snapshot should differ after mutating original")
}

func TestSnapshotConditionsEmpty(t *testing.T) {
	snapshot := SnapshotConditions(nil)
	assert.NotNil(t, snapshot)
	assert.Empty(t, snapshot)
}

func TestSetStatusConditionIdempotency(t *testing.T) {
	conditions := []metav1.Condition{}

	// First call: sets the condition
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    "Available",
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciled",
		Message: "Dashboard reconciled successfully",
	})

	snapshot := SnapshotConditions(conditions)

	// Second call with identical values: meta.SetStatusCondition is a no-op
	// (doesn't update LastTransitionTime when Status unchanged)
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    "Available",
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciled",
		Message: "Dashboard reconciled successfully",
	})

	assert.False(t, ConditionsChanged(snapshot, conditions),
		"re-applying identical condition should not register as changed")
}
