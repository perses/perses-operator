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
		args = append(args, fmt.Sprintf("--web.tls-key-file=%s/%s",
			tlsCertMountPath, perses.Spec.TLS.UserCert.PrivateKeyPath))
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
