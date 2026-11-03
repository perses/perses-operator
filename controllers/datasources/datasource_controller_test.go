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
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()

	allowedIDFile := filepath.Join(allowedDir, "client-id")
	require.NoError(t, os.WriteFile(allowedIDFile, []byte("my-client-id"), 0600))

	outsideIDFile := filepath.Join(outsideDir, "client-id")
	require.NoError(t, os.WriteFile(outsideIDFile, []byte("secret-token"), 0600))

	allowed, err := persescommon.ParseAllowedSecretFileDirs(persescommon.DatasourceSecretFileAllowedDirsFlag, allowedDir)
	require.NoError(t, err)
	disabled, err := persescommon.ParseAllowedSecretFileDirs(persescommon.DatasourceSecretFileAllowedDirsFlag, "")
	require.NoError(t, err)

	makeDS := func(clientIDPath string) *persesv1alpha2.PersesDatasource {
		return &persesv1alpha2.PersesDatasource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ds",
				Namespace: "default",
			},
			Spec: persesv1alpha2.DatasourceSpec{
				Client: &persesv1alpha2.Client{
					OAuth: &persesv1alpha2.OAuth{
						SecretSource: persesv1alpha2.SecretSource{
							Type: persesv1alpha2.SecretSourceTypeFile,
						},
						ClientIDPath:     ptr.To(clientIDPath),
						ClientSecretPath: ptr.To("/etc/perses/oauth/client-secret"),
						TokenURL:         "https://example.com/token",
					},
				},
			},
		}
	}

	t.Run("rejects file when AllowedSecretFileDirs is empty", func(t *testing.T) {
		r := &PersesDatasourceReconciler{
			AllowedSecretFileDirs: disabled,
		}
		mockClient := new(internal.MockClient)

		_, reason, err := r.syncPersesSecret(ctx, mockClient, makeDS(allowedIDFile))
		require.Error(t, err)
		assert.Equal(t, persescommon.ReasonInvalidConfiguration, reason)
		assert.Contains(t, err.Error(), "file references in secrets are disabled")
		assert.Contains(t, err.Error(), "--"+persescommon.DatasourceSecretFileAllowedDirsFlag)
	})

	t.Run("rejects file outside AllowedSecretFileDirs", func(t *testing.T) {
		r := &PersesDatasourceReconciler{
			AllowedSecretFileDirs: allowed,
		}
		mockClient := new(internal.MockClient)

		_, reason, err := r.syncPersesSecret(ctx, mockClient, makeDS(outsideIDFile))
		require.Error(t, err)
		assert.Equal(t, persescommon.ReasonInvalidConfiguration, reason)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("allows file inside AllowedSecretFileDirs", func(t *testing.T) {
		r := &PersesDatasourceReconciler{
			AllowedSecretFileDirs: allowed,
		}
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

		_, reason, err := r.syncPersesSecret(ctx, mockClient, makeDS(allowedIDFile))
		require.NoError(t, err)
		assert.Empty(t, reason)
		mockSecret.AssertExpectations(t)
	})
}
