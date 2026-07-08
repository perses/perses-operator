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

package main

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildMetricsServerOptions_SecureEnabled(t *testing.T) {
	tlsOpts := []func(*tls.Config){
		func(c *tls.Config) { c.NextProtos = []string{"http/1.1"} },
	}

	opts := buildMetricsServerOptions(":8443", true, "/tmp/certs", tlsOpts)

	assert.Equal(t, ":8443", opts.BindAddress)
	assert.True(t, opts.SecureServing)
	assert.NotNil(t, opts.FilterProvider, "FilterProvider should be set when secure metrics is enabled")
	assert.Equal(t, "/tmp/certs", opts.CertDir, "CertDir should be set when secure metrics is enabled and certDir is provided")
	assert.Len(t, opts.TLSOpts, 1)
}

func TestBuildMetricsServerOptions_SecureEnabledNoCertDir(t *testing.T) {
	opts := buildMetricsServerOptions(":8443", true, "", nil)

	assert.True(t, opts.SecureServing)
	assert.NotNil(t, opts.FilterProvider)
	assert.Empty(t, opts.CertDir, "CertDir should not be set when certDir is empty")
}

func TestBuildMetricsServerOptions_SecureDisabled(t *testing.T) {
	opts := buildMetricsServerOptions(":8080", false, "/tmp/certs", nil)

	assert.Equal(t, ":8080", opts.BindAddress)
	assert.False(t, opts.SecureServing)
	assert.Nil(t, opts.FilterProvider, "FilterProvider should not be set when secure metrics is disabled")
	assert.Empty(t, opts.CertDir, "CertDir should not be set when secure metrics is disabled")
}
