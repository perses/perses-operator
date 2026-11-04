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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// forbiddenSecretFileDirs lists directories that can never be used as allowed
// secret file directories in the operator container, even if explicitly configured.
var forbiddenSecretFileDirs = []string{
	"/var/run/secrets",
	"/run/secrets",
	"/proc",
	"/sys",
	"/etc/kubernetes",
}

// ParseSecretFileAllowedDirs parses and validates a comma-separated list of
// allowed directories from which the operator may read credential or certificate files.
func ParseSecretFileAllowedDirs(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	dirs := make([]string, 0, len(parts))
	for _, part := range parts {
		dir := strings.TrimSpace(part)
		if dir == "" {
			continue
		}
		if !filepath.IsAbs(dir) {
			return nil, fmt.Errorf("%s: %q must be an absolute path", SecretFileAllowedDirsFlag, dir)
		}
		cleaned := filepath.Clean(dir)
		if cleaned == string(filepath.Separator) {
			return nil, fmt.Errorf("%s: the root directory is not allowed", SecretFileAllowedDirsFlag)
		}
		for _, forbidden := range forbiddenSecretFileDirs {
			if cleaned == forbidden || isPathWithin(cleaned, forbidden) || isPathWithin(forbidden, cleaned) {
				return nil, fmt.Errorf("%s: directory %q overlaps with protected path %q", SecretFileAllowedDirsFlag, dir, forbidden)
			}
		}
		dirs = append(dirs, cleaned)
	}
	return dirs, nil
}

// ValidateSecretFilePath ensures that a file path referenced by a secret source
// with type "file" is located (both lexically and after symlink resolution) inside
// one of the operator's allowed directories.
func ValidateSecretFilePath(path string, allowedDirs []string) error {
	if len(allowedDirs) == 0 {
		return errors.New("file references in secrets are disabled; ask your administrator to configure --" + SecretFileAllowedDirsFlag)
	}

	notAllowed := fmt.Errorf("file %q is not allowed or doesn't exist; it must be located in one of the directories listed in --%s", path, SecretFileAllowedDirsFlag)
	if !filepath.IsAbs(path) {
		return fmt.Errorf("file %q must be an absolute path", path)
	}

	cleaned := filepath.Clean(path)
	for _, forbidden := range forbiddenSecretFileDirs {
		if cleaned == forbidden || isPathWithin(cleaned, forbidden) {
			return notAllowed
		}
	}

	// Lexical check before touching the filesystem.
	if !isInAllowedDirs(cleaned, allowedDirs, false) {
		return notAllowed
	}

	// Resolve symlinks so a symlink inside an allowed directory cannot point outside it.
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return notAllowed
	}
	for _, forbidden := range forbiddenSecretFileDirs {
		if resolved == forbidden || isPathWithin(resolved, forbidden) {
			return notAllowed
		}
	}
	if !isInAllowedDirs(resolved, allowedDirs, true) {
		return notAllowed
	}

	return nil
}

// ReadSecretFile validates path with ValidateSecretFilePath and then reads it
// through an os.Root opened on the allowed directory that contains it. os.Root
// enforces containment at open time, so a symlink swapped in between validation
// and read still cannot escape the allowed directory.
func ReadSecretFile(path string, allowedDirs []string) ([]byte, error) {
	if err := ValidateSecretFilePath(path, allowedDirs); err != nil {
		return nil, err
	}
	cleaned := filepath.Clean(path)
	for _, dir := range allowedDirs {
		if isPathWithin(cleaned, dir) {
			return readFileWithinDir(dir, cleaned)
		}
	}
	return nil, fmt.Errorf("file %q is not located in any of the directories listed in --%s", path, SecretFileAllowedDirsFlag)
}

func readFileWithinDir(dir, path string) ([]byte, error) {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open allowed directory %q: %w", dir, err)
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile(rel)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	return data, nil
}

func isInAllowedDirs(path string, allowedDirs []string, resolveDir bool) bool {
	for _, dir := range allowedDirs {
		if isPathWithin(path, dir) {
			return true
		}
		if resolveDir {
			if resolvedDir, err := filepath.EvalSymlinks(dir); err == nil && isPathWithin(path, resolvedDir) {
				return true
			}
		}
	}
	return false
}

// isPathWithin returns true if path is strictly inside dir.
func isPathWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
