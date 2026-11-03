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
	"io"
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

	parse := func(args ...string) (string, error) {
		var operatorDirs string
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		registerSecretFileFlags(fs, &operatorDirs)
		return operatorDirs, fs.Parse(args)
	}

	t.Run("env var is the default", func(t *testing.T) {
		operatorDirs, err := parse()
		require.NoError(t, err)
		assert.Equal(t, "/opt/env-operator", operatorDirs)
	})

	t.Run("flag overrides env var", func(t *testing.T) {
		operatorDirs, err := parse("--" + common.SecretFileAllowedDirsFlag + "=/opt/flag-operator")
		require.NoError(t, err)
		assert.Equal(t, "/opt/flag-operator", operatorDirs)
	})

	t.Run("datasource allow-list flag no longer exists", func(t *testing.T) {
		_, err := parse("--datasource-secret-file-allowed-directories=/opt/datasource")
		require.Error(t, err)
	})
}

func TestNewPersesClientFactory(t *testing.T) {
	createClient := func(t *testing.T, raw, idPath, secretPath string) error {
		t.Helper()
		factory, err := newPersesClientFactory(raw, filepath.Join(t.TempDir(), "certs"))
		require.NoError(t, err)
		perses := persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{Name: "perses", Namespace: "perses-system"},
			Spec: persesv1alpha2.PersesSpec{Client: &persesv1alpha2.Client{OAuth: &persesv1alpha2.OAuth{
				SecretSource:     persesv1alpha2.SecretSource{Type: persesv1alpha2.SecretSourceTypeFile},
				ClientIDPath:     ptr.To(idPath),
				ClientSecretPath: ptr.To(secretPath),
				TokenURL:         "http://127.0.0.1/token",
			}}},
		}
		_, err = factory.CreateClient(context.Background(), nil, perses)
		return err
	}

	t.Run("allow-list is used by the client factory", func(t *testing.T) {
		dir := t.TempDir()
		idPath := filepath.Join(dir, "client-id")
		secretPath := filepath.Join(dir, "client-secret")
		require.NoError(t, os.WriteFile(idPath, []byte("id"), 0600))
		require.NoError(t, os.WriteFile(secretPath, []byte("secret"), 0600))

		require.NoError(t, createClient(t, dir, idPath, secretPath))

		err := createClient(t, t.TempDir(), idPath, secretPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--"+common.SecretFileAllowedDirsFlag)
	})

	t.Run("empty list disables reads", func(t *testing.T) {
		err := createClient(t, "", "/opt/client-id", "/opt/client-secret")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file references in secrets are disabled")
	})

	t.Run("protects the webhook cert dir", func(t *testing.T) {
		certDir := filepath.Join(t.TempDir(), "certs")
		require.NoError(t, os.Mkdir(certDir, 0700))
		for _, raw := range []string{certDir, filepath.Dir(certDir)} {
			_, err := newPersesClientFactory(raw, certDir)
			require.Error(t, err, raw)
			assert.Contains(t, err.Error(), "invalid --"+common.SecretFileAllowedDirsFlag+" value")
			assert.Contains(t, err.Error(), "overlaps with protected path")
		}
	})

	t.Run("protects a relative webhook cert dir", func(t *testing.T) {
		base := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(base, "certs"), 0700))
		t.Chdir(base)
		_, err := newPersesClientFactory(filepath.Join(base, "certs"), "certs")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "overlaps with protected path")
	})

	t.Run("rejects relative directories", func(t *testing.T) {
		_, err := newPersesClientFactory("relative", filepath.Join(t.TempDir(), "certs"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --"+common.SecretFileAllowedDirsFlag+" value")
	})
}
