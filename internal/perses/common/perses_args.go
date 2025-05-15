package common

import (
	"fmt"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// GetPersesArgs returns the command line arguments for the Perses server.
// It includes the configuration file path, TLS settings if enabled,
// and any additional user-specified arguments.
func GetPersesArgs(perses *v1alpha2.Perses) []string {
	args := []string{fmt.Sprintf("--config=%s", defaultConfigPath)}

	// Append TLS cert args if TLS is enabled and user certificates are provided
	if hasTLSConfiguration(perses) {
		args = append(args, fmt.Sprintf("--web.tls-cert-file=%s/%s",
			tlsCertMountPath, perses.Spec.TLS.UserCert.CertPath))
		args = append(args, fmt.Sprintf("--web.tls-key-file=%s/%s",
			tlsCertMountPath, perses.Spec.TLS.UserCert.PrivateKeyPath))
	}

	// Append user provided args
	args = append(args, perses.Spec.Args...)

	return args
}
