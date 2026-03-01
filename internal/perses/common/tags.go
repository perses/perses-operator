/*
Copyright The Perses Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"strings"

	"github.com/perses/common/set"
)

const TagsAnnotation = PersesNamespaceDomain + "/tags"

// ParseTags reads the perses.dev/tags annotation from the given annotations map,
// splits the comma-separated value, trims whitespace, and returns a set of tags.
// Empty entries are skipped. Returns nil if the annotation is absent or empty.
func ParseTags(annotations map[string]string) set.Set[string] {
	raw, ok := annotations[TagsAnnotation]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			tags = append(tags, t)
		}
	}

	if len(tags) == 0 {
		return nil
	}

	return set.New(tags...)
}
