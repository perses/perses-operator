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
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SnapshotConditions returns a shallow copy of the conditions slice.
// Use before applying mutations so the original values are preserved
// for comparison with ConditionsChanged.
func SnapshotConditions(conditions []metav1.Condition) []metav1.Condition {
	snapshot := make([]metav1.Condition, len(conditions))
	copy(snapshot, conditions)
	return snapshot
}

// ConditionsChanged reports whether the conditions slice differs from
// a previously taken snapshot. It is safe to call with nil or empty slices.
func ConditionsChanged(before, after []metav1.Condition) bool {
	return !equality.Semantic.DeepEqual(before, after)
}
