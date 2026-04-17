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

package tls

import (
	"crypto/tls"
	"strings"

	k8sapiflag "k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("tls")

func ParseTLSVersion(versionStr string) (uint16, error) {
	if versionStr == "" {
		return 0, nil
	}
	version, err := k8sapiflag.TLSVersion(versionStr)
	if err != nil {
		return 0, err
	}
	if version < tls.VersionTLS12 {
		log.Info("WARNING: TLS version older than 1.2 is deprecated and insecure", "version", versionStr)
	}
	return version, nil
}

func ParseCipherSuites(cipherSuitesStr string) ([]uint16, error) {
	if cipherSuitesStr == "" {
		return nil, nil
	}

	names := strings.Split(cipherSuitesStr, ",")
	trimmed := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			trimmed = append(trimmed, name)
		}
	}
	if len(trimmed) == 0 {
		return nil, nil
	}
	return k8sapiflag.TLSCipherSuites(trimmed)
}

func ConfigureTLS(minVersion uint16, cipherSuites []uint16, enableHTTP2 bool) func(*tls.Config) {
	return func(c *tls.Config) {
		if !enableHTTP2 {
			c.NextProtos = []string{"http/1.1"}
		}
		if minVersion != 0 {
			c.MinVersion = minVersion
		}
		if len(cipherSuites) > 0 {
			c.CipherSuites = cipherSuites
		}
	}
}
