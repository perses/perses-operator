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

package datasources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/perses/perses/pkg/client/perseshttp"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/secret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	internal "github.com/perses/perses-operator/internal/perses"
	persescommon "github.com/perses/perses-operator/internal/perses/common"
)

func TestSyncPersesSecretOAuthFileValidation(t *testing.T) {
	ctx := context.Background()

	makeDS := func(oauth persesv1alpha2.OAuth) *persesv1alpha2.PersesDatasource {
		oauth.SecretSource = persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeFile}
		oauth.ClientSecretPath = ptr.To("/etc/perses/oauth/client-secret")
		oauth.TokenURL = "https://example.com/token"
		return &persesv1alpha2.PersesDatasource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ds",
				Namespace: "default",
			},
			Spec: persesv1alpha2.DatasourceSpec{
				Client: &persesv1alpha2.Client{OAuth: &oauth},
			},
		}
	}

	t.Run("rejects clientIDPath without reading the file", func(t *testing.T) {
		idFile := filepath.Join(t.TempDir(), "client-id")
		require.NoError(t, os.WriteFile(idFile, []byte("operator-credential"), 0600))
		r := &PersesDatasourceReconciler{}

		_, reason, err := r.syncPersesSecret(ctx, new(internal.MockClient), makeDS(persesv1alpha2.OAuth{ClientIDPath: ptr.To(idFile)}))
		require.Error(t, err)
		assert.Equal(t, persescommon.ReasonInvalidConfiguration, reason)
		assert.Contains(t, err.Error(), "oauth.clientIDPath is not supported when oauth.type is file")
		assert.NotContains(t, err.Error(), "operator-credential")
	})

	t.Run("requires clientID", func(t *testing.T) {
		r := &PersesDatasourceReconciler{}

		_, reason, err := r.syncPersesSecret(ctx, new(internal.MockClient), makeDS(persesv1alpha2.OAuth{}))
		require.Error(t, err)
		assert.Equal(t, persescommon.ReasonInvalidConfiguration, reason)
		assert.Contains(t, err.Error(), "oauth.clientID is required when oauth.type is file")
	})

	t.Run("uses the literal clientID", func(t *testing.T) {
		r := &PersesDatasourceReconciler{}
		mockClient := new(internal.MockClient)
		mockSecret := new(internal.MockSecret)
		mockClient.On("Secret", "default").Return(mockSecret)
		mockSecret.On("Get", "test-ds-secret").Return(&persesv1.Secret{}, perseshttp.RequestNotFoundError)
		mockSecret.On("Create", &persesv1.Secret{
			Kind: persesv1.KindSecret,
			Metadata: persesv1.ProjectMetadata{
				Metadata: persesv1.Metadata{Name: "test-ds-secret"},
			},
			Spec: persesv1.SecretSpec{
				OAuth: &secret.OAuth{
					ClientID:         "my-client-id",
					ClientSecretFile: "/etc/perses/oauth/client-secret",
					TokenURL:         "https://example.com/token",
				},
			},
		}).Return(&persesv1.Secret{}, nil)

		_, reason, err := r.syncPersesSecret(ctx, mockClient, makeDS(persesv1alpha2.OAuth{ClientID: ptr.To("my-client-id")}))
		require.NoError(t, err)
		assert.Empty(t, reason)
		mockSecret.AssertExpectations(t)
	})
}
