// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"flag"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/perses/perses/pkg/client/api/v1"
	clientConfig "github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/perses/pkg/model/api/v1/secret"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
)

const tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

type PersesClientFactory interface {
	CreateClient(ctx context.Context, client client.Reader, perses persesv1alpha2.Perses) (v1.ClientInterface, error)
	CreateClientsForAllPods(ctx context.Context, k8sClient client.Reader, perses persesv1alpha2.Perses) ([]v1.ClientInterface, error)
}

type PersesClientFactoryWithConfig struct{}

func NewWithConfig() PersesClientFactory {
	return &PersesClientFactoryWithConfig{}
}

func (f *PersesClientFactoryWithConfig) getProtocolAndPort(perses *persesv1alpha2.Perses) (string, int32) {
	httpProtocol := "http"
	if isTLSEnabled(perses) {
		httpProtocol = "https"
	}
	containerPort := DefaultContainerPort
	if perses.Spec.ContainerPort != nil {
		containerPort = *perses.Spec.ContainerPort
	}
	return httpProtocol, containerPort
}

func (f *PersesClientFactoryWithConfig) buildBaseConfig(ctx context.Context, k8sClient client.Reader, perses *persesv1alpha2.Perses) (clientConfig.RestConfigClient, error) {
	config := clientConfig.RestConfigClient{}

	if isKubernetesAuthEnabled(perses) {
		tokenBytes, err := os.ReadFile(tokenPath)
		if err != nil {
			return config, fmt.Errorf("failed to read service account token from %s: %w", tokenPath, err)
		}
		saToken := string(tokenBytes)

		if saToken == "" {
			return config, fmt.Errorf("service account token is empty, ensure the Perses operator has the correct permissions")
		}

		config.Headers = map[string]string{
			"Authorization": "Bearer " + saToken,
		}
	}

	if isClientTLSEnabled(perses) {
		tls := perses.Spec.Client.TLS

		insecureSkipVerify := false
		if tls.InsecureSkipVerify != nil {
			insecureSkipVerify = *tls.InsecureSkipVerify
		}

		tlsConfig := &secret.TLSConfig{
			InsecureSkipVerify: insecureSkipVerify,
		}

		if tls.CaCert != nil {
			switch tls.CaCert.Type {
			case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
				caData, _, err := GetTLSCertData(ctx, k8sClient, perses.Namespace, perses.Name, tls.CaCert)
				if err != nil {
					return config, err
				}
				tlsConfig.CA = caData
			case persesv1alpha2.SecretSourceTypeFile:
				tlsConfig.CAFile = tls.CaCert.CertPath
			}
		}

		if tls.UserCert != nil {
			switch tls.UserCert.Type {
			case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
				cert, key, err := GetTLSCertData(ctx, k8sClient, perses.Namespace, perses.Name, tls.UserCert)
				if err != nil {
					return config, err
				}
				tlsConfig.Cert = cert
				tlsConfig.Key = key
			case persesv1alpha2.SecretSourceTypeFile:
				tlsConfig.CertFile = tls.UserCert.CertPath
			}
		}

		config.TLSConfig = tlsConfig
	}

	return config, nil
}

func (f *PersesClientFactoryWithConfig) createClientForURL(urlStr string, baseConfig clientConfig.RestConfigClient) (v1.ClientInterface, error) {
	parsedURL, err := common.ParseURL(urlStr)
	if err != nil {
		return nil, err
	}

	config := baseConfig
	config.URL = parsedURL

	restClient, err := clientConfig.NewRESTClient(config)
	if err != nil {
		return nil, err
	}

	return v1.NewWithClient(restClient), nil
}

func (f *PersesClientFactoryWithConfig) getServiceURL(perses *persesv1alpha2.Perses) string {
	httpProtocol, containerPort := f.getProtocolAndPort(perses)

	serverURLFlag := flag.Lookup(PersesServerURLFlag)
	if serverURLFlag != nil && serverURLFlag.Value.String() != "" {
		return serverURLFlag.Value.String()
	}

	return fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d%s", httpProtocol, perses.Name, perses.Namespace, containerPort, perses.Spec.Config.APIPrefix)
}

func (f *PersesClientFactoryWithConfig) CreateClient(ctx context.Context, k8sClient client.Reader, perses persesv1alpha2.Perses) (v1.ClientInterface, error) {
	baseConfig, err := f.buildBaseConfig(ctx, k8sClient, &perses)
	if err != nil {
		return nil, err
	}

	urlStr := f.getServiceURL(&perses)
	return f.createClientForURL(urlStr, baseConfig)
}

func (f *PersesClientFactoryWithConfig) CreateClientsForAllPods(ctx context.Context, k8sClient client.Reader, perses persesv1alpha2.Perses) ([]v1.ClientInterface, error) {
	if perses.Spec.Config.Database.SQL != nil {
		c, err := f.CreateClient(ctx, k8sClient, perses)
		if err != nil {
			return nil, err
		}
		return []v1.ClientInterface{c}, nil
	}

	baseConfig, err := f.buildBaseConfig(ctx, k8sClient, &perses)
	if err != nil {
		return nil, err
	}

	podList := &corev1.PodList{}
	err = k8sClient.List(ctx, podList,
		client.InNamespace(perses.Namespace),
		client.MatchingLabels(LabelsForPerses(perses.Name, &perses)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for Perses instance %s/%s: %w", perses.Namespace, perses.Name, err)
	}

	httpProtocol, containerPort := f.getProtocolAndPort(&perses)

	var clients []v1.ClientInterface
	for i := range podList.Items {
		pod := &podList.Items[i]
		if !isPodReady(pod) {
			continue
		}
		urlStr := fmt.Sprintf("%s://%s:%d%s", httpProtocol, pod.Status.PodIP, containerPort, perses.Spec.Config.APIPrefix)
		c, err := f.createClientForURL(urlStr, baseConfig)
		if err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no ready pods found for Perses instance %s/%s", perses.Namespace, perses.Name)
	}

	return clients, nil
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

type PersesClientFactoryWithClient struct {
	client v1.ClientInterface
}

func NewWithClient(client v1.ClientInterface) PersesClientFactory {
	return &PersesClientFactoryWithClient{client: client}
}

func (f *PersesClientFactoryWithClient) CreateClient(_ context.Context, _ client.Reader, _ persesv1alpha2.Perses) (v1.ClientInterface, error) {
	return f.client, nil
}

func (f *PersesClientFactoryWithClient) CreateClientsForAllPods(_ context.Context, _ client.Reader, _ persesv1alpha2.Perses) ([]v1.ClientInterface, error) {
	return []v1.ClientInterface{f.client}, nil
}
