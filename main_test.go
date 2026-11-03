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

package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
)

func TestRegisterSecretFileFlags(t *testing.T) {
	t.Setenv(common.SecretFileAllowedDirsEnv, "/opt/env-operator")
	t.Setenv(common.DatasourceSecretFileAllowedDirsEnv, "/opt/env-datasource")

	parse := func(args ...string) (operatorDirs, datasourceDirs string) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		registerSecretFileFlags(fs, &operatorDirs, &datasourceDirs)
		require.NoError(t, fs.Parse(args))
		return operatorDirs, datasourceDirs
	}

	t.Run("env vars are the defaults", func(t *testing.T) {
		operatorDirs, datasourceDirs := parse()
		assert.Equal(t, "/opt/env-operator", operatorDirs)
		assert.Equal(t, "/opt/env-datasource", datasourceDirs)
	})

	t.Run("flags override env vars", func(t *testing.T) {
		operatorDirs, datasourceDirs := parse(
			"--"+common.SecretFileAllowedDirsFlag+"=/opt/flag-operator",
			"--"+common.DatasourceSecretFileAllowedDirsFlag+"=",
		)
		assert.Equal(t, "/opt/flag-operator", operatorDirs)
		assert.Empty(t, datasourceDirs)
	})
}

func TestSetupSecretFileReaders(t *testing.T) {
	writeOAuthFiles := func(t *testing.T, dir string) (idPath, secretPath string) {
		t.Helper()
		idPath = filepath.Join(dir, "client-id")
		secretPath = filepath.Join(dir, "client-secret")
		require.NoError(t, os.WriteFile(idPath, []byte("id"), 0600))
		require.NoError(t, os.WriteFile(secretPath, []byte("secret"), 0600))
		return idPath, secretPath
	}
	createClient := func(readers secretFileReaders, idPath, secretPath string) error {
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system"},
			Spec: persesv1alpha2.PersesSpec{Client: &persesv1alpha2.Client{OAuth: &persesv1alpha2.OAuth{
				SecretSource:     persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeFile},
				ClientIDPath:     ptr.To(idPath),
				ClientSecretPath: ptr.To(secretPath),
				TokenURL:         "http://127.0.0.1/token",
			}}},
		}
		_, err := readers.clientFactory.CreateClient(context.Background(), nil, perses)
		return err
	}

	t.Run("operator list goes to the client factory and datasource list to the reconcilers", func(t *testing.T) {
		operatorDir, datasourceDir := t.TempDir(), t.TempDir()
		operatorID, operatorSecret := writeOAuthFiles(t, operatorDir)
		datasourceID, datasourceSecret := writeOAuthFiles(t, datasourceDir)

		readers, err := setupSecretFileReaders(operatorDir, datasourceDir, filepath.Join(t.TempDir(), "certs"))
		require.NoError(t, err)
		assert.False(t, readers.overlap)

		require.NoError(t, createClient(readers, operatorID, operatorSecret))
		err = createClient(readers, datasourceID, datasourceSecret)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--"+common.SecretFileAllowedDirsFlag)

		_, err = readers.datasourceDirs.ReadFile(datasourceID)
		require.NoError(t, err)
		_, err = readers.datasourceDirs.ReadFile(operatorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--"+common.DatasourceSecretFileAllowedDirsFlag)
	})

	t.Run("empty lists disable reads", func(t *testing.T) {
		readers, err := setupSecretFileReaders("", "", filepath.Join(t.TempDir(), "certs"))
		require.NoError(t, err)
		_, err = readers.datasourceDirs.ReadFile("/opt/client-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file references in secrets are disabled")
	})

	t.Run("overlapping lists are reported", func(t *testing.T) {
		base := t.TempDir()
		nested := filepath.Join(base, "nested")
		require.NoError(t, os.Mkdir(nested, 0700))
		readers, err := setupSecretFileReaders(base, nested, filepath.Join(t.TempDir(), "certs"))
		require.NoError(t, err)
		assert.True(t, readers.overlap)
	})

	for _, flagName := range []string{common.SecretFileAllowedDirsFlag, common.DatasourceSecretFileAllowedDirsFlag} {
		setup := func(raw, certDir string) error {
			operatorRaw, datasourceRaw := raw, ""
			if flagName == common.DatasourceSecretFileAllowedDirsFlag {
				operatorRaw, datasourceRaw = "", raw
			}
			_, err := setupSecretFileReaders(operatorRaw, datasourceRaw, certDir)
			return err
		}

		t.Run(flagName+" protects the webhook cert dir", func(t *testing.T) {
			certDir := filepath.Join(t.TempDir(), "certs")
			require.NoError(t, os.Mkdir(certDir, 0700))
			for _, raw := range []string{certDir, filepath.Dir(certDir)} {
				err := setup(raw, certDir)
				require.Error(t, err, raw)
				assert.Contains(t, err.Error(), "invalid --"+flagName+" value")
				assert.Contains(t, err.Error(), "overlaps with protected path")
			}
		})

		t.Run(flagName+" protects a relative webhook cert dir", func(t *testing.T) {
			base := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(base, "certs"), 0700))
			t.Chdir(base)
			err := setup(filepath.Join(base, "certs"), "certs")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "overlaps with protected path")
		})

		t.Run(flagName+" rejects relative directories", func(t *testing.T) {
			err := setup("relative", filepath.Join(t.TempDir(), "certs"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid --"+flagName+" value")
		})
	}
}
