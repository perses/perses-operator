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
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAllowedSecretFileDirs(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		dirs, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, "")
		require.NoError(t, err)
		assert.Empty(t, dirs.Dirs())
	})

	t.Run("valid directories are cleaned", func(t *testing.T) {
		dirs, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, "/etc/perses/creds/, /var/perses/certs")
		require.NoError(t, err)
		assert.Equal(t, []string{"/etc/perses/creds", "/var/perses/certs"}, dirs.Dirs())
	})

	t.Run("duplicate directories are removed", func(t *testing.T) {
		dirs, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, "/etc/perses/creds,/etc/perses/creds/,/etc/perses/../perses/creds")
		require.NoError(t, err)
		assert.Equal(t, []string{"/etc/perses/creds"}, dirs.Dirs())
	})

	t.Run("symlinked duplicate directories are removed", func(t *testing.T) {
		realDir := t.TempDir()
		linkedDir := filepath.Join(t.TempDir(), "linked")
		require.NoError(t, os.Symlink(realDir, linkedDir))
		dirs, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, linkedDir+","+realDir)
		require.NoError(t, err)
		assert.Equal(t, []string{linkedDir}, dirs.Dirs())
	})

	t.Run("extra protected directory and its parents and children are rejected", func(t *testing.T) {
		parent := t.TempDir()
		protected := filepath.Join(parent, "serving-certs")
		require.NoError(t, os.MkdirAll(filepath.Join(protected, "sub"), 0700))
		for _, dir := range []string{protected, parent, filepath.Join(protected, "sub")} {
			_, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, dir, protected)
			require.Error(t, err, "expected %s to be rejected", dir)
			assert.Contains(t, err.Error(), "overlaps with protected path")
		}

		link := filepath.Join(t.TempDir(), "protected-link")
		require.NoError(t, os.Symlink(protected, link))
		_, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, link, protected)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "overlaps with protected path")
	})

	t.Run("relative extra protected directory is rejected", func(t *testing.T) {
		_, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, "/etc/perses/creds", "relative")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be an absolute path")
	})

	t.Run("errors name the configured flag", func(t *testing.T) {
		_, err := ParseAllowedSecretFileDirs(DatasourceSecretFileAllowedDirsFlag, "relative/path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), DatasourceSecretFileAllowedDirsFlag)
	})

	t.Run("relative path is rejected", func(t *testing.T) {
		_, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, "relative/path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be an absolute path")
	})

	t.Run("root directory is rejected", func(t *testing.T) {
		_, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, "/")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is the root directory, which is not allowed")
	})

	t.Run("symlink to the root directory is rejected", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "root-link")
		require.NoError(t, os.Symlink("/", link))
		_, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, link)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is the root directory, which is not allowed")
	})

	t.Run("symlink to a protected directory is rejected", func(t *testing.T) {
		protected := withForbiddenTempDir(t)
		link := filepath.Join(t.TempDir(), "protected-link")
		require.NoError(t, os.Symlink(protected, link))
		_, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, link)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "overlaps with protected path")
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
			_, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, forbidden)
			require.Error(t, err, "expected %s to be rejected", forbidden)
			assert.Contains(t, err.Error(), "overlaps with protected path")
		}
	})
}

func TestResolveSecretFile(t *testing.T) {
	validate := func(path string, dirs []string) error {
		_, _, err := testAllowedDirs(dirs...).resolve(path)
		return err
	}

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
		err := validate(validFile, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file references in secrets are disabled")
	})

	t.Run("allows valid file inside allowed directory", func(t *testing.T) {
		err := validate(validFile, []string{allowedDir})
		require.NoError(t, err)
	})

	t.Run("rejects relative path", func(t *testing.T) {
		err := validate("relative/file", []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be an absolute path")
	})

	t.Run("rejects file outside allowed directory", func(t *testing.T) {
		err := validate(outsideFile, []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("rejects path traversal out of allowed directory", func(t *testing.T) {
		traversal := filepath.Join(allowedDir, "..", filepath.Base(outsideDir), "secret")
		err := validate(traversal, []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("rejects symlink escaping allowed directory", func(t *testing.T) {
		err := validate(symlinkEscape, []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("missing file inside an allowed directory reports non-existence", func(t *testing.T) {
		missing := filepath.Join(allowedDir, "not-there")
		err := validate(missing, []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
		// Needed for ReadFile's rotation retry.
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("matches a canonical reference against a symlinked allowed directory", func(t *testing.T) {
		realDir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(realDir, "secret"), []byte("canonical"), 0600))
		linkedDir := filepath.Join(t.TempDir(), "linked")
		require.NoError(t, os.Symlink(realDir, linkedDir))

		// Allowed dir is the symlink; the reference uses the resolved canonical path.
		err = validate(filepath.Join(realDir, "secret"), []string{linkedDir})
		require.NoError(t, err)
	})

	t.Run("rejects kubernetes serviceaccount token path directly and via symlink", func(t *testing.T) {
		err := validate("/var/run/secrets/kubernetes.io/serviceaccount/token", []string{allowedDir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")

		// Outside a pod the token is missing; inside, it's protected. Both must fail.
		err = validate(symlinkToSA, []string{allowedDir})
		require.Error(t, err)
	})

	// The SA token case above may pass only because the token is missing.
	t.Run("rejects symlink resolving into a protected directory", func(t *testing.T) {
		protected := withForbiddenTempDir(t)
		require.NoError(t, os.WriteFile(filepath.Join(protected, "token"), []byte("secret"), 0600))
		link := filepath.Join(allowedDir, "protected-link")
		require.NoError(t, os.Symlink(filepath.Join(protected, "token"), link))

		_, err := testAllowedDirs(allowedDir, protected).ReadFile(link)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("ignores allowed directory that resolves into a protected directory", func(t *testing.T) {
		protected := withForbiddenTempDir(t)
		require.NoError(t, os.WriteFile(filepath.Join(protected, "token"), []byte("secret"), 0600))
		// Simulates an allowed directory replaced by a symlink after startup.
		linkedDir := filepath.Join(t.TempDir(), "linked")
		require.NoError(t, os.Symlink(protected, linkedDir))

		_, err := testAllowedDirs(linkedDir).ReadFile(filepath.Join(linkedDir, "token"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})
}

// withForbiddenTempDir adds a temp directory to forbiddenSecretFileDirs for the test.
func withForbiddenTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	original := forbiddenSecretFileDirs
	forbiddenSecretFileDirs = append(slices.Clone(original), dir)
	t.Cleanup(func() { forbiddenSecretFileDirs = original })
	return dir
}

// testAllowedDirs skips startup checks to exercise read-time checks alone.
func testAllowedDirs(dirs ...string) AllowedSecretFileDirs {
	return AllowedSecretFileDirs{flag: SecretFileAllowedDirsFlag, dirs: dirs}
}

func TestReadFileErrorsNameTheFlag(t *testing.T) {
	allowedDir := t.TempDir()
	outsideFile := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0600))

	disabled, err := ParseAllowedSecretFileDirs(DatasourceSecretFileAllowedDirsFlag, "")
	require.NoError(t, err)
	_, err = disabled.ReadFile(outsideFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file references in secrets are disabled; ask your administrator to configure --"+DatasourceSecretFileAllowedDirsFlag)

	allowed, err := ParseAllowedSecretFileDirs(DatasourceSecretFileAllowedDirsFlag, allowedDir)
	require.NoError(t, err)
	_, err = allowed.ReadFile(outsideFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--"+DatasourceSecretFileAllowedDirsFlag)

	_, err = AllowedSecretFileDirs{}.ReadFile(outsideFile)
	require.Error(t, err)
	assert.Equal(t, "file references in secrets are disabled", err.Error())
}

func TestReadFileRejectsExtraProtectedDirectory(t *testing.T) {
	allowedDir := t.TempDir()
	protected, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(protected, "tls.key"), []byte("key"), 0600))
	link := filepath.Join(allowedDir, "key-link")
	require.NoError(t, os.Symlink(filepath.Join(protected, "tls.key"), link))

	allowed, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, allowedDir, protected)
	require.NoError(t, err)
	_, err = allowed.ReadFile(link)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
}

func TestAllowedSecretFileDirsOverlaps(t *testing.T) {
	parse := func(raw string) AllowedSecretFileDirs {
		dirs, err := ParseAllowedSecretFileDirs(SecretFileAllowedDirsFlag, raw)
		require.NoError(t, err)
		return dirs
	}
	operator := parse("/opt/perses-creds")

	assert.True(t, parse("/opt/perses-creds").Overlaps(operator))
	assert.True(t, parse("/opt/perses-creds/datasources").Overlaps(operator))
	assert.True(t, parse("/opt").Overlaps(operator))
	assert.False(t, parse("/opt/perses-datasource-creds").Overlaps(operator))
	assert.False(t, parse("").Overlaps(operator))
}

// Kubernetes volumes use a relative symlink chain
// (<key> -> ..data/<key> -> ..<timestamp>/<key>) that os.Root must follow.
func TestReadSecretFileKubernetesVolumeLayout(t *testing.T) {
	mount := t.TempDir()
	versioned := filepath.Join(mount, "..2026_11_02_09_00_00.000000000")
	require.NoError(t, os.MkdirAll(versioned, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(versioned, "token"), []byte("k8s-token"), 0600))
	require.NoError(t, os.Symlink("..2026_11_02_09_00_00.000000000", filepath.Join(mount, "..data")))
	require.NoError(t, os.Symlink("..data/token", filepath.Join(mount, "token")))

	data, err := testAllowedDirs(mount).ReadFile(filepath.Join(mount, "token"))
	require.NoError(t, err)
	assert.Equal(t, "k8s-token", string(data))
}

func TestReadSecretFileSymlinks(t *testing.T) {
	t.Run("absolute symlink within the allowed directory", func(t *testing.T) {
		allowedDir := t.TempDir()
		target := filepath.Join(allowedDir, "real")
		require.NoError(t, os.WriteFile(target, []byte("abs"), 0600))
		link := filepath.Join(allowedDir, "abs-link")
		require.NoError(t, os.Symlink(target, link))

		data, err := testAllowedDirs(allowedDir).ReadFile(link)
		require.NoError(t, err)
		assert.Equal(t, "abs", string(data))
	})

	t.Run("nested allowed directories work regardless of order", func(t *testing.T) {
		outer := t.TempDir()
		inner := filepath.Join(outer, "inner")
		require.NoError(t, os.MkdirAll(inner, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(outer, "real"), []byte("nested"), 0600))
		uplink := filepath.Join(inner, "uplink")
		require.NoError(t, os.Symlink(filepath.Join("..", "real"), uplink))

		for _, dirs := range [][]string{{outer, inner}, {inner, outer}} {
			data, err := testAllowedDirs(dirs...).ReadFile(uplink)
			require.NoError(t, err, "allowed dirs %v", dirs)
			assert.Equal(t, "nested", string(data))
		}
	})

	t.Run("symlink into another allowed directory", func(t *testing.T) {
		dirA := t.TempDir()
		dirB := t.TempDir()
		target := filepath.Join(dirB, "real")
		require.NoError(t, os.WriteFile(target, []byte("cross"), 0600))
		link := filepath.Join(dirA, "cross-link")
		require.NoError(t, os.Symlink(target, link))

		data, err := testAllowedDirs(dirA, dirB).ReadFile(link)
		require.NoError(t, err)
		assert.Equal(t, "cross", string(data))

		_, err = testAllowedDirs(dirA).ReadFile(link)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed or doesn't exist")
	})

	t.Run("allowed directory that is itself a symlink", func(t *testing.T) {
		realDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(realDir, "secret"), []byte("via-link"), 0600))
		linkedDir := filepath.Join(t.TempDir(), "linked")
		require.NoError(t, os.Symlink(realDir, linkedDir))

		data, err := testAllowedDirs(linkedDir).ReadFile(filepath.Join(linkedDir, "secret"))
		require.NoError(t, err)
		assert.Equal(t, "via-link", string(data))
	})
}

func TestReadSecretFileRejectsUnsuitableFiles(t *testing.T) {
	allowedDir := t.TempDir()

	t.Run("directory", func(t *testing.T) {
		subdir := filepath.Join(allowedDir, "subdir")
		require.NoError(t, os.MkdirAll(subdir, 0700))
		_, err := testAllowedDirs(allowedDir).ReadFile(subdir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a regular file")
	})

	t.Run("file larger than the size limit", func(t *testing.T) {
		big := filepath.Join(allowedDir, "big")
		require.NoError(t, os.WriteFile(big, make([]byte, maxSecretFileSize+1), 0600))
		_, err := testAllowedDirs(allowedDir).ReadFile(big)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds the maximum size")
	})

	t.Run("file at the size limit", func(t *testing.T) {
		exact := filepath.Join(allowedDir, "exact")
		require.NoError(t, os.WriteFile(exact, make([]byte, maxSecretFileSize), 0600))
		data, err := testAllowedDirs(allowedDir).ReadFile(exact)
		require.NoError(t, err)
		assert.Len(t, data, maxSecretFileSize)
	})

	t.Run("empty file", func(t *testing.T) {
		empty := filepath.Join(allowedDir, "empty")
		require.NoError(t, os.WriteFile(empty, nil, 0600))
		_, err := testAllowedDirs(allowedDir).ReadFile(empty)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file is empty")
	})
}

// readFileWithinDir must refuse escapes even if a symlink is swapped in after
// resolve validated the path, so test it without pre-validation.
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
			// Errors may surface in resource status.
			assert.NotContains(t, err.Error(), allowedDir)
			assert.NotContains(t, err.Error(), outsideDir)
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
