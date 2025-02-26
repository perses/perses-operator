package common

import (
	"fmt"
	"github.com/perses/perses-operator/api/v1alpha1"
)

func GetPersesArgs(tls *v1alpha1.TLS, args []string) []string {
	defaultArgs := []string{"--config=/perses/config/config.yaml"}

	// append tls cert args if user cert and key is provided
	if tls != nil && tls.Enable && tls.UserCert != nil {
		defaultArgs = append(defaultArgs, fmt.Sprintf("--web.tls-cert-file=/tls/%s", tls.UserCert.CertFile))
		defaultArgs = append(defaultArgs, fmt.Sprintf("--web.tls-key-file=/tls/%s", tls.UserCert.CertKeyFile))
	}

	// append user provided args
	defaultArgs = append(defaultArgs, args...)

	return defaultArgs
}
