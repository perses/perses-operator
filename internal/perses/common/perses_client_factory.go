package common

import (
	"flag"
	"fmt"

	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	v1 "github.com/perses/perses/pkg/client/api/v1"
	clientConfig "github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

type PersesClientFactory interface {
	CreateClient(perses persesv1alpha1.Perses) (v1.ClientInterface, error)
}

type PersesClientFactoryWithConfig struct{}

func NewWithConfig() PersesClientFactory {
	return &PersesClientFactoryWithConfig{}
}

func (f *PersesClientFactoryWithConfig) CreateClient(perses persesv1alpha1.Perses) (v1.ClientInterface, error) {
	var urlStr string

	if flag.Lookup("perses-server-url").Value.String() != "" {
		urlStr = flag.Lookup("perses-server-url").Value.String()
	} else {
		urlStr = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", perses.Name, perses.Namespace, perses.Spec.ContainerPort)
	}
	parsedURL, err := common.ParseURL(urlStr)
	if err != nil {
		return nil, err
	}

	restClient, err := clientConfig.NewRESTClient(clientConfig.RestConfigClient{
		URL: &common.URL{URL: parsedURL.URL},
	})
	if err != nil {
		return nil, err
	}

	persesClient := v1.NewWithClient(restClient)

	return persesClient, nil
}

type PersesClientFactoryWithURL struct {
	url string
}

func NewWithURL(url string) PersesClientFactory {
	return &PersesClientFactoryWithURL{url: url}
}

func (f *PersesClientFactoryWithURL) CreateClient(config persesv1alpha1.Perses) (v1.ClientInterface, error) {
	urStr := f.url
	parsedURL, err := common.ParseURL(urStr)
	if err != nil {
		return nil, err
	}
	restClient, err := clientConfig.NewRESTClient(clientConfig.RestConfigClient{
		URL: &common.URL{URL: parsedURL.URL},
	})

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

func (f *PersesClientFactoryWithClient) CreateClient(config persesv1alpha1.Perses) (v1.ClientInterface, error) {
	return f.client, nil
}
