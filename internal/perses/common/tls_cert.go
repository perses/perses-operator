package common

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/perses/perses-operator/api/v1alpha1"
)

// GetTLSCertData get tls certs from a Certificate resoruce
func GetTLSCertData(ctx context.Context, client client.Client, namespace string, name string, cert *v1alpha1.Certificate) (string, string, error) {
	var certData string
	var keyData string

	if cert.Type == v1alpha1.CertificateTypeConfigMap || cert.Type == v1alpha1.CertificateTypeSecret {
		if len(cert.Name) == 0 {
			return "", "", fmt.Errorf("No name found for tls certificate: %s with type: %s", cert.CertPath, cert.Type)
		}

		switch cert.Type {
		case v1alpha1.CertificateTypeSecret:
			secret := &corev1.Secret{}

			err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cert.Name}, secret)

			if err != nil {
				return "", "", err
			}

			certData = string(secret.Data[cert.CertPath])
			keyData = string(secret.Data[cert.PrivateKeyPath])
		case v1alpha1.CertificateTypeConfigMap:
			cm := &corev1.ConfigMap{}
			err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cert.Name}, cm)

			if err != nil {
				return "", "", err
			}

			certData = cm.Data[cert.CertPath]
			keyData = cm.Data[cert.PrivateKeyPath]
		}

		if certData == "" {
			return "", "", fmt.Errorf("No data found for certificate: %s in namespace: %s for %s", cert.CertPath, namespace, name)
		}

		return certData, keyData, nil
	}

	return "", "", nil
}
