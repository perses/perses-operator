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

	"github.com/stretchr/testify/assert"
)

func TestMergeReason(t *testing.T) {
	tests := []struct {
		name     string
		current  ConditionStatusReason
		incoming ConditionStatusReason
		want     ConditionStatusReason
	}{
		{"empty current yields to incoming", "", ReasonBackendError, ReasonBackendError},
		{"validation outranks backend", ReasonBackendError, ReasonValidationFailed, ReasonValidationFailed},
		{"config outranks backend", ReasonBackendError, ReasonInvalidConfiguration, ReasonInvalidConfiguration},
		{"connection outranks backend", ReasonBackendError, ReasonConnectionFailed, ReasonConnectionFailed},
		{"backend does not clobber validation", ReasonValidationFailed, ReasonBackendError, ReasonValidationFailed},
		{"backend does not clobber config", ReasonInvalidConfiguration, ReasonBackendError, ReasonInvalidConfiguration},
		{"validation outranks connection", ReasonConnectionFailed, ReasonValidationFailed, ReasonValidationFailed},
		{"equal severity keeps current", ReasonValidationFailed, ReasonInvalidConfiguration, ReasonValidationFailed},
		{"same reason is stable", ReasonBackendError, ReasonBackendError, ReasonBackendError},
		{"empty incoming does not lower current", ReasonValidationFailed, "", ReasonValidationFailed},
		{"known incoming outranks unknown current", "SomethingElse", ReasonBackendError, ReasonBackendError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MergeReason(tt.current, tt.incoming))
		})
	}
}
