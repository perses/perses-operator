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

package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSecretFileAllowedDirs(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		dirs, err := ParseSecretFileAllowedDirs("")
		require.NoError(t, err)
		assert.Nil(t, dirs)
	})

	t.Run("valid directories are cleaned", func(t *testing.T) {
		dirs, err := ParseSecretFileAllowedDirs("/etc/perses/creds/, /var/perses/certs")
		require.NoError(t, err)
		assert.Equal(t, []string{"/etc/perses/creds", "/var/perses/certs"}, dirs)
	})

	t.Run("relative path is rejected", func(t *testing.T) {
		_, err := ParseSecretFileAllowedDirs("relative/path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be an absolute path")
	})

	t.Run("root directory is rejected", func(t *testing.T) {
		_, err := ParseSecretFileAllowedDirs("/")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the root directory is not allowed")
	})

	t.Run("kubernetes serviceaccount directory and parents are rejected", func(t *testing.T) {
		for _, forbidden := range []string{
			"/var/run/secrets",
			"/var/run/secrets/kubernetes.io/serviceaccount",
			"/var/run",
			"/var",
			"/run/secrets",
			"/proc",
			"/sys",
			"/etc/kubernetes",
		} {
			_, err := ParseSecretFileAllowedDirs(forbidden)
			require.Error(t, err, "expected %s to be rejected", forbidden)
			assert.Contains(t, err.Error(), "overlaps with protected path")
		}
	})
}

func TestValidateSecretFilePath(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()

	validFile := filepath.Join(allowedDir, "client-id")
	require.NoError(t, os.WriteFile(validFile, []byte("my-client-id"), 0600))

	outsideFile := filepath.Join(outsideDir, "secret")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0600))

	symlinkEscape := filepath.Join(allowedDir, "escape-link")
	require.NoError(t, os.Symlink(outsideFile, symlinkEscape))

	symlinkToSA := filepath.Join(allowedDir, "sa-token-link")
	require.NoError(t, os.Symlink("/var/run/secrets/kubernetes.io/serviceaccount/token", symlinkToSA))

	t.Run("disabled when no allowed directories configured", func(t *testing.T) {
		err := ValidateSecretFilePath(validFile, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file references in secrets are disabled")
	})

	t.Run("allows valid file inside allowed directory", func(t *testing.T) {
		err := ValidateSecretFilePath(validFile, []string{allowedDir})
		require.NoError(t, err)
	})

	t.Run("rejects relative path", func(t *testing.T) {
		err := ValidateSecretFilePath("relative/file", []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be an absolute path")
	})

	t.Run("rejects file outside allowed directory", func(t *testing.T) {
		err := ValidateSecretFilePath(outsideFile, []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("rejects path traversal out of allowed directory", func(t *testing.T) {
		traversal := filepath.Join(allowedDir, "..", filepath.Base(outsideDir), "secret")
		err := ValidateSecretFilePath(traversal, []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("rejects symlink escaping allowed directory", func(t *testing.T) {
		err := ValidateSecretFilePath(symlinkEscape, []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("rejects kubernetes serviceaccount token path directly and via symlink", func(t *testing.T) {
		err := ValidateSecretFilePath("/var/run/secrets/kubernetes.io/serviceaccount/token", []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")

		err = ValidateSecretFilePath(symlinkToSA, []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})
}

// Kubernetes Secret/ConfigMap/projected volumes expose files through a relative
// symlink chain (<key> -> ..data/<key> -> ..<timestamp>/<key>); os.Root must
// still be able to follow it.
func TestReadSecretFileKubernetesVolumeLayout(t *testing.T) {
	mount := t.TempDir()
	versioned := filepath.Join(mount, "..2026_11_02_09_00_00.000000000")
	require.NoError(t, os.MkdirAll(versioned, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(versioned, "token"), []byte("k8s-token"), 0600))
	require.NoError(t, os.Symlink("..2026_11_02_09_00_00.000000000", filepath.Join(mount, "..data")))
	require.NoError(t, os.Symlink("..data/token", filepath.Join(mount, "token")))

	data, err := ReadSecretFile(filepath.Join(mount, "token"), []string{mount})
	require.NoError(t, err)
	assert.Equal(t, "k8s-token", string(data))
}

// readFileWithinDir is the second line of defence: even if a symlink is swapped
// in after ValidateSecretFilePath ran, os.Root must refuse to follow it outside
// the allowed directory. Exercise it directly, without the pre-validation.
func TestReadFileWithinDirRefusesEscapes(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()

	outsideFile := filepath.Join(outsideDir, "secret")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0600))

	absLink := filepath.Join(allowedDir, "abs-link")
	require.NoError(t, os.Symlink(outsideFile, absLink))

	relLink := filepath.Join(allowedDir, "rel-link")
	require.NoError(t, os.Symlink(filepath.Join("..", filepath.Base(outsideDir), "secret"), relLink))

	dirLink := filepath.Join(allowedDir, "dir-link")
	require.NoError(t, os.Symlink(outsideDir, dirLink))

	for name, path := range map[string]string{
		"absolute symlink":         absLink,
		"relative symlink":         relLink,
		"file under dir symlink":   filepath.Join(dirLink, "secret"),
		"lexical parent traversal": filepath.Join(allowedDir, "..", filepath.Base(outsideDir), "secret"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := readFileWithinDir(allowedDir, path)
			require.Error(t, err)
		})
	}

	t.Run("regular file still readable", func(t *testing.T) {
		regular := filepath.Join(allowedDir, "regular")
		require.NoError(t, os.WriteFile(regular, []byte("ok"), 0600))
		data, err := readFileWithinDir(allowedDir, regular)
		require.NoError(t, err)
		assert.Equal(t, "ok", string(data))
	})
}
