package common

import (
	"fmt"

	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	v1 "github.com/perses/perses/pkg/client/api/v1"
	perseshttp "github.com/perses/perses/pkg/client/perseshttp"
)

type PersesClientFactory interface {
	CreateClient(perses persesv1alpha1.Perses) (v1.ClientInterface, error)
}

type PersesClientFactoryWithConfig struct{}

func NewWithConfig() PersesClientFactory {
	return &PersesClientFactoryWithConfig{}
}

func (f *PersesClientFactoryWithConfig) CreateClient(perses persesv1alpha1.Perses) (v1.ClientInterface, error) {
	restClient, err := perseshttp.NewFromConfig(perseshttp.RestConfigClient{
		URL: fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", perses.Name, perses.Namespace, perses.Spec.ContainerPort),
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
	restClient, err := perseshttp.NewFromConfig(perseshttp.RestConfigClient{
		URL: f.url,
	})

	if err != nil {
		return nil, err
	}

	persesClient := v1.NewWithClient(restClient)

	return persesClient, nil
}
