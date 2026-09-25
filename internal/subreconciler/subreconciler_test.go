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

package subreconciler

import (
	"testing"
	"time"
)

func TestRequeueForPeriodicSync(t *testing.T) {
	t.Parallel()

	t.Run("positive interval requeues after delay", func(t *testing.T) {
		t.Parallel()
		result, err := RequeueForPeriodicSync(5 * time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.RequeueAfter != 5*time.Minute {
			t.Fatalf("expected RequeueAfter=%v, got %v", 5*time.Minute, result.RequeueAfter)
		}
	})

	t.Run("zero interval disables periodic sync", func(t *testing.T) {
		t.Parallel()
		result, err := RequeueForPeriodicSync(0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.RequeueAfter != 0 {
			t.Fatalf("expected RequeueAfter=0, got %v", result.RequeueAfter)
		}
	})

	t.Run("negative interval disables periodic sync", func(t *testing.T) {
		t.Parallel()
		result, err := RequeueForPeriodicSync(-time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.RequeueAfter != 0 {
			t.Fatalf("expected RequeueAfter=0, got %v", result.RequeueAfter)
		}
	})
}
