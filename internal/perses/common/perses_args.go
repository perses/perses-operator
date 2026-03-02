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

package common

import (
	"fmt"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// GetPersesArgs returns the command line arguments for the Perses server.
// It includes the configuration file path, TLS settings if enabled,
// log settings, and any additional user-specified arguments.
func GetPersesArgs(perses *v1alpha2.Perses) []string {
	args := []string{fmt.Sprintf("--config=%s", defaultConfigPath)}

	// Append TLS cert args if TLS is enabled and user certificates are provided
	if hasTLSConfiguration(perses) {
		args = append(args, fmt.Sprintf("--web.tls-cert-file=%s/%s",
			tlsCertMountPath, perses.Spec.TLS.UserCert.CertPath))
		privateKeyPath := ""
		if perses.Spec.TLS.UserCert.PrivateKeyPath != nil {
			privateKeyPath = *perses.Spec.TLS.UserCert.PrivateKeyPath
		}
		args = append(args, fmt.Sprintf("--web.tls-key-file=%s/%s",
			tlsCertMountPath, privateKeyPath))
	}

	// Append log level if specified
	if perses.Spec.LogLevel != nil {
		args = append(args, fmt.Sprintf("--log.level=%s", *perses.Spec.LogLevel))
	}

	// Append log method trace if enabled
	if perses.Spec.LogMethodTrace != nil && *perses.Spec.LogMethodTrace {
		args = append(args, "--log.method-trace")
	}

	// Append user provided args
	args = append(args, perses.Spec.Args...)

	return args
}
