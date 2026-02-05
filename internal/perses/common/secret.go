/*
Copyright 2026 The Perses Authors.

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

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/perses/perses-operator/api/v1alpha2"
)

const SecretNameSuffix = "-secret"

func HasSecretConfig(c *v1alpha2.Client) bool {
	return c != nil && (c.TLS != nil && c.TLS.Enable || c.BasicAuth != nil || c.OAuth != nil)
}

// GetBasicAuthData get basic auth from a BasicAuth resource
func GetBasicAuthData(ctx context.Context, client client.Client, namespace string, name string, basicAuth *v1alpha2.BasicAuth) (string, error) {
	var passwordData string

	if basicAuth.Type == v1alpha2.SecretSourceTypeSecret || basicAuth.Type == v1alpha2.SecretSourceTypeConfigMap {
		if len(basicAuth.Name) == 0 {
			return "", fmt.Errorf("no name found for basic auth: %s with type: %s", basicAuth.Username, basicAuth.Type)
		}

		if len(basicAuth.Namespace) != 0 {
			namespace = basicAuth.Namespace
		}

		switch basicAuth.Type {
		case v1alpha2.SecretSourceTypeSecret:
			secret := &corev1.Secret{}

			err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: basicAuth.Name}, secret)

			if err != nil {
				return "", err
			}

			passwordData = string(secret.Data[basicAuth.PasswordPath])
		case v1alpha2.SecretSourceTypeConfigMap:
			cm := &corev1.ConfigMap{}
			err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: basicAuth.Name}, cm)

			if err != nil {
				return "", err
			}

			passwordData = cm.Data[basicAuth.PasswordPath]
		}

		if passwordData == "" {
			return "", fmt.Errorf("no password data found for basic auth: %s in namespace: %s for %s", basicAuth.PasswordPath, namespace, name)
		}

		return passwordData, nil
	}

	return "", nil
}

// GetOAuthData get basic auth from a OAuth resource
func GetOAuthData(ctx context.Context, client client.Client, namespace string, name string, oauth *v1alpha2.OAuth) (string, string, error) {
	var clientIDData string
	var clientSecretData string

	if oauth.Type == v1alpha2.SecretSourceTypeSecret || oauth.Type == v1alpha2.SecretSourceTypeConfigMap {
		if len(oauth.Name) == 0 {
			return "", "", fmt.Errorf("no name found for oauth: %s with type: %s", oauth.ClientIDPath, oauth.Type)
		}

		if len(oauth.Namespace) != 0 {
			namespace = oauth.Namespace
		}

		switch oauth.Type {
		case v1alpha2.SecretSourceTypeSecret:
			secret := &corev1.Secret{}

			err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: oauth.Name}, secret)

			if err != nil {
				return "", "", err
			}

			clientIDData = string(secret.Data[oauth.ClientIDPath])
			clientSecretData = string(secret.Data[oauth.ClientSecretPath])
		case v1alpha2.SecretSourceTypeConfigMap:
			cm := &corev1.ConfigMap{}
			err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: oauth.Name}, cm)

			if err != nil {
				return "", "", err
			}

			clientIDData = cm.Data[oauth.ClientIDPath]
			clientSecretData = cm.Data[oauth.ClientSecretPath]
		}

		if clientIDData == "" {
			return "", "", fmt.Errorf("no client id data found for oauth: %s in namespace: %s for %s", oauth.ClientIDPath, namespace, name)
		}

		if clientSecretData == "" {
			return "", "", fmt.Errorf("no client secret data found for oauth: %s in namespace: %s for %s", oauth.ClientSecretPath, namespace, name)
		}

		return clientIDData, clientSecretData, nil
	}

	return "", "", nil
}

// GetTLSCertData get tls certs from a Certificate resource
func GetTLSCertData(ctx context.Context, client client.Client, namespace string, name string, cert *v1alpha2.Certificate) (string, string, error) {
	var certData string
	var keyData string

	if cert.Type == v1alpha2.SecretSourceTypeSecret || cert.Type == v1alpha2.SecretSourceTypeConfigMap {
		if len(cert.Name) == 0 {
			return "", "", fmt.Errorf("no name found for tls certificate: %s with type: %s", cert.CertPath, cert.Type)
		}

		if len(cert.Namespace) != 0 {
			namespace = cert.Namespace
		}

		switch cert.Type {
		case v1alpha2.SecretSourceTypeSecret:
			secret := &corev1.Secret{}

			err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cert.Name}, secret)

			if err != nil {
				return "", "", err
			}

			certData = string(secret.Data[cert.CertPath])
			keyData = string(secret.Data[cert.PrivateKeyPath])
		case v1alpha2.SecretSourceTypeConfigMap:
			cm := &corev1.ConfigMap{}
			err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cert.Name}, cm)

			if err != nil {
				return "", "", err
			}

			certData = cm.Data[cert.CertPath]
			keyData = cm.Data[cert.PrivateKeyPath]
		}

		if certData == "" {
			return "", "", fmt.Errorf("no data found for certificate: %s in namespace: %s for %s", cert.CertPath, namespace, name)
		}

		return certData, keyData, nil
	}

	return "", "", nil
}

// ComputeTLSCertificateChecksum computes a SHA256 checksum of all TLS certificate data
// referenced in the Perses TLS configuration. This checksum can be used as a pod annotation
// to trigger rolling updates when certificates are rotated.
func ComputeTLSCertificateChecksum(ctx context.Context, c client.Client, perses *v1alpha2.Perses) (string, error) {
	if !isTLSEnabled(perses) {
		return "", nil
	}

	var checksumParts []string

	if perses.Spec.TLS.CaCert != nil {
		certData, _, err := GetTLSCertData(ctx, c, perses.Namespace, perses.Name, perses.Spec.TLS.CaCert)
		if err != nil {
			return "", fmt.Errorf("failed to get CA certificate data: %w", err)
		}
		if certData != "" {
			checksumParts = append(checksumParts, certData)
		}
	}

	if perses.Spec.TLS.UserCert != nil {
		certData, keyData, err := GetTLSCertData(ctx, c, perses.Namespace, perses.Name, perses.Spec.TLS.UserCert)
		if err != nil {
			return "", fmt.Errorf("failed to get user certificate data: %w", err)
		}
		if certData != "" {
			checksumParts = append(checksumParts, certData)
		}
		if keyData != "" {
			checksumParts = append(checksumParts, keyData)
		}
	}

	if len(checksumParts) == 0 {
		return "", nil
	}

	// Sort for deterministic ordering in case new cert fields are added
	sort.Strings(checksumParts)

	hash := sha256.Sum256([]byte(strings.Join(checksumParts, "")))
	return hex.EncodeToString(hash[:]), nil
}

// GetTLSSecretReferences returns a list of Secret references used in TLS configuration.
// This only returns references with Type=Secret (not ConfigMap).
// This can be used to set up watches for certificate rotation.
func GetTLSSecretReferences(perses *v1alpha2.Perses) []types.NamespacedName {
	if !isTLSEnabled(perses) {
		return nil
	}

	var refs []types.NamespacedName
	namespace := perses.Namespace

	if perses.Spec.TLS.CaCert != nil &&
		perses.Spec.TLS.CaCert.Type == v1alpha2.SecretSourceTypeSecret &&
		perses.Spec.TLS.CaCert.Name != "" {
		ns := namespace
		if perses.Spec.TLS.CaCert.Namespace != "" {
			ns = perses.Spec.TLS.CaCert.Namespace
		}
		refs = append(refs, types.NamespacedName{
			Namespace: ns,
			Name:      perses.Spec.TLS.CaCert.Name,
		})
	}

	if perses.Spec.TLS.UserCert != nil &&
		perses.Spec.TLS.UserCert.Type == v1alpha2.SecretSourceTypeSecret &&
		perses.Spec.TLS.UserCert.Name != "" {
		ns := namespace
		if perses.Spec.TLS.UserCert.Namespace != "" {
			ns = perses.Spec.TLS.UserCert.Namespace
		}
		refs = append(refs, types.NamespacedName{
			Namespace: ns,
			Name:      perses.Spec.TLS.UserCert.Name,
		})
	}

	return refs
}
