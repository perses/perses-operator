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
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/stretchr/testify/assert"
)

func TestReasonError(t *testing.T) {
	inner := fmt.Errorf("something broke")
	re := NewReasonError(inner, ReasonValidationFailed)

	assert.Equal(t, "something broke", re.Error())
	assert.Equal(t, ReasonValidationFailed, re.Reason)
	assert.True(t, errors.Is(re, inner))
}

func TestExtractReason_WithReasonError(t *testing.T) {
	inner := fmt.Errorf("bad spec")
	re := NewReasonError(inner, ReasonValidationFailed)

	assert.Equal(t, ReasonValidationFailed, ExtractReason(re, "reconciliation_failed"))
}

func TestExtractReason_WithWrappedReasonError(t *testing.T) {
	inner := fmt.Errorf("bad spec")
	re := NewReasonError(inner, ReasonMissingPerses)
	wrapped := fmt.Errorf("outer context: %w", re)

	assert.Equal(t, ReasonMissingPerses, ExtractReason(wrapped, "reconciliation_failed"))
}

func TestExtractReason_WithPlainError(t *testing.T) {
	err := fmt.Errorf("plain error")

	assert.Equal(t, ConditionStatusReason("reconciliation_failed"), ExtractReason(err, "reconciliation_failed"))
}

func TestExtractReason_WithNilError(t *testing.T) {
	assert.Equal(t, ConditionStatusReason("reconciliation_failed"), ExtractReason(nil, "reconciliation_failed"))
}

func TestIsClientError_400(t *testing.T) {
	err := &perseshttp.RequestError{Message: "bad request", StatusCode: http.StatusBadRequest}
	assert.True(t, IsClientError(err))
}

func TestIsClientError_422(t *testing.T) {
	err := &perseshttp.RequestError{Message: "unprocessable", StatusCode: http.StatusUnprocessableEntity}
	assert.True(t, IsClientError(err))
}

func TestIsClientError_500(t *testing.T) {
	assert.False(t, IsClientError(perseshttp.RequestInternalError))
}

func TestIsClientError_404(t *testing.T) {
	assert.True(t, IsClientError(perseshttp.RequestNotFoundError))
}

func TestIsClientError_NetworkError(t *testing.T) {
	err := fmt.Errorf("connection refused")
	assert.False(t, IsClientError(err))
}

func TestIsClientError_Nil(t *testing.T) {
	assert.False(t, IsClientError(nil))
}

func TestIsClientError_WrappedRequestError(t *testing.T) {
	inner := &perseshttp.RequestError{Message: "bad request", StatusCode: http.StatusBadRequest}
	wrapped := fmt.Errorf("validation call failed: %w", inner)
	assert.True(t, IsClientError(wrapped))
}

func TestIsClientError_Wrapped500(t *testing.T) {
	inner := &perseshttp.RequestError{Message: "server error", StatusCode: http.StatusServiceUnavailable}
	wrapped := fmt.Errorf("validate endpoint: %w", inner)
	assert.False(t, IsClientError(wrapped))
}
