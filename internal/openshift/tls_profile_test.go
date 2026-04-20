// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openshift

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = configv1.Install(s)
	return s
}

func TestFetchTLSProfile_Intermediate(t *testing.T) {
	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type:         configv1.TLSProfileIntermediateType,
				Intermediate: &configv1.IntermediateTLSProfile{},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(apiServer).Build()
	minVersion, cipherSuites, profileSpec, err := FetchTLSProfile(context.Background(), c)
	require.NoError(t, err)

	assert.NotEmpty(t, minVersion, "minVersion should be set for Intermediate profile")
	assert.NotEmpty(t, profileSpec.MinTLSVersion, "profileSpec.MinTLSVersion should be set")
	assert.NotEmpty(t, profileSpec.Ciphers, "profileSpec.Ciphers should be set")
	assert.NotEmpty(t, cipherSuites, "cipherSuites should be set for Intermediate profile")
	assert.NotContains(t, cipherSuites, " ", "cipherSuites should not contain spaces")
}

func TestFetchTLSProfile_Old(t *testing.T) {
	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileOldType,
				Old:  &configv1.OldTLSProfile{},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(apiServer).Build()
	minVersion, cipherSuites, _, err := FetchTLSProfile(context.Background(), c)
	require.NoError(t, err)

	assert.NotEmpty(t, minVersion)
	assert.NotEmpty(t, cipherSuites)
}

func TestFetchTLSProfile_Custom(t *testing.T) {
	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS13,
						Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
					},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(apiServer).Build()
	minVersion, cipherSuites, profileSpec, err := FetchTLSProfile(context.Background(), c)
	require.NoError(t, err)

	assert.Equal(t, "VersionTLS13", minVersion)
	assert.Equal(t, "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384", cipherSuites)
	assert.Equal(t, configv1.VersionTLS13, profileSpec.MinTLSVersion)
	assert.Equal(t, []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"}, profileSpec.Ciphers)
}

func TestFetchTLSProfile_NoCiphers(t *testing.T) {
	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(apiServer).Build()
	_, cipherSuites, _, err := FetchTLSProfile(context.Background(), c)
	require.NoError(t, err)

	assert.Empty(t, cipherSuites, "cipherSuites should be empty when no ciphers specified")
}

func TestFetchTLSProfile_NoAPIServer(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newScheme()).Build()
	_, _, _, err := FetchTLSProfile(context.Background(), c)

	assert.Error(t, err, "should error when APIServer resource not found")
}

func TestFetchTLSProfile_NilTLSProfile(t *testing.T) {
	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       configv1.APIServerSpec{},
	}

	c := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(apiServer).Build()
	minVersion, cipherSuites, _, err := FetchTLSProfile(context.Background(), c)
	require.NoError(t, err)

	// When no TLS profile is set, the library returns a default profile
	assert.NotEmpty(t, minVersion, "should return a default minVersion when no profile is set")
	_ = cipherSuites
}
