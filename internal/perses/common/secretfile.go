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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// maxSecretFileSize fits large CA bundles while bounding operator memory use.
const maxSecretFileSize = 1 << 20 // 1 MiB

// forbiddenSecretFileDirs can never be allowed or read.
var forbiddenSecretFileDirs = []string{
	"/var/run/secrets",
	"/run/secrets",
	"/proc",
	"/sys",
	"/etc/kubernetes",
}

// AllowedSecretFileDirs is a validated set of directories the operator may read
// secret files from. The zero value disables reads.
type AllowedSecretFileDirs struct {
	flag string // for error messages
	dirs []string
	// protected extends forbiddenSecretFileDirs, in configured and resolved form.
	protected []string
}

// ParseAllowedSecretFileDirs parses the comma-separated directories set by
// flagName. Files under extraProtected can never be read.
func ParseAllowedSecretFileDirs(flagName, raw string, extraProtected ...string) (AllowedSecretFileDirs, error) {
	allowed := AllowedSecretFileDirs{flag: flagName}
	for _, dir := range extraProtected {
		if !filepath.IsAbs(dir) {
			return AllowedSecretFileDirs{}, fmt.Errorf("protected directory %q must be an absolute path", dir)
		}
		cleaned := filepath.Clean(dir)
		allowed.protected = append(allowed.protected, cleaned)
		if resolved, err := filepath.EvalSymlinks(cleaned); err == nil && resolved != cleaned {
			allowed.protected = append(allowed.protected, resolved)
		}
	}

	// Dedupe by resolved path when the directory exists.
	seen := map[string]bool{}
	for part := range strings.SplitSeq(raw, ",") {
		dir := strings.TrimSpace(part)
		if dir == "" {
			continue
		}
		if !filepath.IsAbs(dir) {
			return AllowedSecretFileDirs{}, fmt.Errorf("%s: %q must be an absolute path", flagName, dir)
		}
		cleaned := filepath.Clean(dir)
		if err := allowed.checkAllowedDir(cleaned); err != nil {
			return AllowedSecretFileDirs{}, fmt.Errorf("%s: directory %q %w", flagName, dir, err)
		}
		key := cleaned
		// Reads use the resolved directory; missing ones are re-checked on read.
		if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
			if err := allowed.checkAllowedDir(resolved); err != nil {
				return AllowedSecretFileDirs{}, fmt.Errorf("%s: directory %q resolves to %q, which %w", flagName, dir, resolved, err)
			}
			key = resolved
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		allowed.dirs = append(allowed.dirs, cleaned)
	}
	return allowed, nil
}

// Dirs returns the allowed directories.
func (a AllowedSecretFileDirs) Dirs() []string {
	return slices.Clone(a.dirs)
}

// Overlaps reports whether any configured directories are equal or nested.
func (a AllowedSecretFileDirs) Overlaps(other AllowedSecretFileDirs) bool {
	for _, dir := range a.dirs {
		for _, otherDir := range other.dirs {
			if pathsOverlap(dir, otherDir) {
				return true
			}
		}
	}
	return false
}

func (a AllowedSecretFileDirs) protectedDirs() []string {
	return slices.Concat(forbiddenSecretFileDirs, a.protected)
}

// checkAllowedDir rejects "/" and directories overlapping protected ones.
func (a AllowedSecretFileDirs) checkAllowedDir(dir string) error {
	if dir == string(filepath.Separator) {
		return errors.New("is the root directory, which is not allowed")
	}
	for _, forbidden := range a.protectedDirs() {
		if pathsOverlap(dir, forbidden) {
			return fmt.Errorf("overlaps with protected path %q", forbidden)
		}
	}
	return nil
}

func (a AllowedSecretFileDirs) isForbiddenPath(path string) bool {
	for _, forbidden := range a.protectedDirs() {
		if path == forbidden || isPathWithin(path, forbidden) {
			return true
		}
	}
	return false
}

// resolve validates path and returns it and its allowed directory, both
// symlink-resolved.
func (a AllowedSecretFileDirs) resolve(path string) (resolved, rootDir string, err error) {
	if len(a.dirs) == 0 {
		msg := "file references in secrets are disabled"
		if a.flag != "" {
			msg += "; ask your administrator to configure --" + a.flag
		}
		return "", "", errors.New(msg)
	}

	notAllowed := fmt.Errorf("file %q is not allowed or doesn't exist; it must be located in one of the directories listed in --%s", path, a.flag)
	if !filepath.IsAbs(path) {
		return "", "", fmt.Errorf("file %q must be an absolute path", path)
	}

	cleaned := filepath.Clean(path)
	if a.isForbiddenPath(cleaned) {
		return "", "", notAllowed
	}

	// Empty entries are directories that can't currently be resolved.
	resolvedDirs := make([]string, len(a.dirs))
	for i, dir := range a.dirs {
		if resolvedDir, err := filepath.EvalSymlinks(dir); err == nil {
			resolvedDirs[i] = resolvedDir
		}
	}

	// Check lexically before touching the file, so paths outside the allowed
	// directories can't probe for existence.
	lexicallyAllowed := false
	for i, dir := range a.dirs {
		if isPathWithin(cleaned, dir) || (resolvedDirs[i] != "" && isPathWithin(cleaned, resolvedDirs[i])) {
			lexicallyAllowed = true
			break
		}
	}
	if !lexicallyAllowed {
		return "", "", notAllowed
	}

	resolved, err = filepath.EvalSymlinks(cleaned)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Safe to reveal inside an allowed directory; wrapping lets ReadFile
			// retry a Kubernetes "..data" rotation race.
			return "", "", fmt.Errorf("file %q does not exist: %w", path, fs.ErrNotExist)
		}
		return "", "", notAllowed
	}
	if a.isForbiddenPath(resolved) {
		return "", "", notAllowed
	}
	for _, resolvedDir := range resolvedDirs {
		if resolvedDir == "" || a.checkAllowedDir(resolvedDir) != nil {
			continue
		}
		if isPathWithin(resolved, resolvedDir) {
			return resolved, resolvedDir, nil
		}
	}
	return "", "", notAllowed
}

// ReadFile validates path and reads it through an os.Root on its allowed
// directory, so a symlink swapped in after validation can't escape.
func (a AllowedSecretFileDirs) ReadFile(path string) ([]byte, error) {
	data, err := a.resolveAndReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		// Kubelet may have swapped "..data" mid-read.
		data, err = a.resolveAndReadFile(path)
	}
	return data, err
}

func (a AllowedSecretFileDirs) resolveAndReadFile(path string) ([]byte, error) {
	resolved, rootDir, err := a.resolve(path)
	if err != nil {
		return nil, err
	}
	data, err := readFileWithinDir(rootDir, resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	return data, nil
}

// readFileWithinDir reads a regular file of at most maxSecretFileSize from dir.
// Errors omit paths since they may surface in resource status.
func readFileWithinDir(dir, path string) ([]byte, error) {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return nil, errors.New("could not determine file location within the allowed directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, pathErrorCause(err)
	}
	defer func() { _ = root.Close() }()

	// O_NONBLOCK keeps a FIFO from blocking the open.
	f, err := root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, pathErrorCause(err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, pathErrorCause(err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	// Bound the bytes actually read rather than trusting a racy stat size.
	data, err := io.ReadAll(io.LimitReader(f, maxSecretFileSize+1))
	if err != nil {
		return nil, pathErrorCause(err)
	}
	if len(data) > maxSecretFileSize {
		return nil, fmt.Errorf("file exceeds the maximum size of %d bytes", maxSecretFileSize)
	}
	if len(data) == 0 {
		return nil, errors.New("file is empty")
	}
	return data, nil
}

// pathErrorCause drops the path from an *os.PathError.
func pathErrorCause(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// pathsOverlap reports whether a and b are equal or one contains the other.
func pathsOverlap(a, b string) bool {
	return a == b || isPathWithin(a, b) || isPathWithin(b, a)
}

// isPathWithin returns true if path is strictly inside dir.
func isPathWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	// IsLocal(".") is true, so exclude dir itself.
	return err == nil && rel != "." && filepath.IsLocal(rel)
}
