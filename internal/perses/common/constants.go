/*
Copyright 2023 The Perses Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import "github.com/perses/perses-operator/api/v1alpha2"

const (
	PersesFinalizer     = "perses.dev/finalizer"
	TypeAvailablePerses = "Available"
	TypeDegradedPerses  = "Degraded"

	// Flags
	PersesServerURLFlag = "perses-server-url"

	// Volume names
	configVolumeName  = "config"
	StorageVolumeName = "storage"

	// TLS volume names
	caVolumeName     = "ca"
	caMountPath      = "/ca"
	tlsVolumeName    = "tls"
	tlsCertMountPath = "/tls"

	// Mount paths
	storageMountPath  = "/perses"
	configMountPath   = "/etc/perses/config"
	defaultConfigPath = configMountPath + "/config.yaml"

	defaultFileMode = 420
)

// isTLSEnabled checks if TLS is enabled in the Perses configuration
func isTLSEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.TLS != nil &&
		perses.Spec.TLS.Enable
}

// hasTLSConfiguration checks if valid TLS configuration is present
func hasTLSConfiguration(perses *v1alpha2.Perses) bool {
	return isTLSEnabled(perses) &&
		perses.Spec.TLS.UserCert != nil &&
		perses.Spec.TLS.UserCert.CertPath != "" &&
		perses.Spec.TLS.UserCert.PrivateKeyPath != ""
}

// isClientTLSEnabled checks if TLS is enabled in the Perses client configuration
func isClientTLSEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.Client != nil &&
		perses.Spec.Client.TLS != nil &&
		perses.Spec.Client.TLS.Enable
}

func isKubernetesAuthEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.Client != nil &&
		perses.Spec.Client.KubernetesAuth != nil &&
		perses.Spec.Client.KubernetesAuth.Enable
}
