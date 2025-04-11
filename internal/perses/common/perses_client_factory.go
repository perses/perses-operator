package common

import (
	"context"
	"flag"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/perses/perses/pkg/client/api/v1"
	clientConfig "github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/perses/pkg/model/api/v1/secret"

	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
)

type PersesClientFactory interface {
	CreateClient(ctx context.Context, client client.Client, perses persesv1alpha1.Perses) (v1.ClientInterface, error)
}

type PersesClientFactoryWithConfig struct{}

func NewWithConfig() PersesClientFactory {
	return &PersesClientFactoryWithConfig{}
}

func (f *PersesClientFactoryWithConfig) CreateClient(ctx context.Context, client client.Client, perses persesv1alpha1.Perses) (v1.ClientInterface, error) {
	var urlStr string

	var httpProtocol = "http"
	if isTLSEnabled(&perses) {
		httpProtocol = "https"
	}

	serverURLFlag := flag.Lookup(PersesServerURLFlag)
	if serverURLFlag != nil && serverURLFlag.Value.String() != "" {
		urlStr = serverURLFlag.Value.String()
	} else {
		urlStr = fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d", httpProtocol, perses.Name, perses.Namespace, perses.Spec.ContainerPort)
	}
	parsedURL, err := common.ParseURL(urlStr)
	if err != nil {
		return nil, err
	}

	config := clientConfig.RestConfigClient{
		URL: parsedURL,
	}

	if isClientTLSEnabled(&perses) {
		tls := perses.Spec.Client.TLS

		tlsConfig := &secret.TLSConfig{
			InsecureSkipVerify: tls.InsecureSkipVerify,
		}

		if tls.CaCert != nil {
			if tls.CaCert.Type == persesv1alpha1.CertificateTypeSecret || tls.CaCert.Type == persesv1alpha1.CertificateTypeConfigMap {
				caData, _, err := GetTLSCertData(ctx, client, perses.Namespace, perses.Name, tls.CaCert)

				if err != nil {
					return nil, err
				}

				tlsConfig.CA = caData
			} else if tls.CaCert.Type == persesv1alpha1.CertificateTypeFile {
				tlsConfig.CAFile = tls.CaCert.CertPath
			}
		}

		if tls.UserCert != nil {
			if tls.UserCert.Type == persesv1alpha1.CertificateTypeSecret || tls.UserCert.Type == persesv1alpha1.CertificateTypeConfigMap {
				cert, key, err := GetTLSCertData(ctx, client, perses.Namespace, perses.Name, tls.UserCert)

				if err != nil {
					return nil, err
				}

				tlsConfig.Cert = cert
				tlsConfig.Key = key
			} else if tls.UserCert.Type == persesv1alpha1.CertificateTypeFile {
				tlsConfig.CertFile = tls.UserCert.CertPath
			}
		}

		config.TLSConfig = tlsConfig
	}

	restClient, err := clientConfig.NewRESTClient(config)
	if err != nil {
		return nil, err
	}

	persesClient := v1.NewWithClient(restClient)

	return persesClient, nil
}

type PersesClientFactoryWithClient struct {
	client v1.ClientInterface
}

func NewWithClient(client v1.ClientInterface) PersesClientFactory {
	return &PersesClientFactoryWithClient{client: client}
}

func (f *PersesClientFactoryWithClient) CreateClient(_ context.Context, _ client.Client, _ persesv1alpha1.Perses) (v1.ClientInterface, error) {
	return f.client, nil
}
