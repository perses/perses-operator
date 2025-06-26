package common

import (
	"context"
	"fmt"

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
