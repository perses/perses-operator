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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/perses/perses/scripts/pkg/changelog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateChangelog(t *testing.T) {
	now := time.Now()
	title := fmt.Sprintf("## 0.20.0 / %s", now.Format("2006-01-02"))
	testSuite := []struct {
		title    string
		clog     *changelog.Changelog
		expected string
	}{
		{
			title:    "empty changelog",
			clog:     &changelog.Changelog{},
			expected: fmt.Sprintf("%s\n%s\n", title, ""),
		},
		{
			title: "changelog with every entry",
			clog: &changelog.Changelog{
				Features: []string{"Discard Changes Confirmation Dialog (#834)"},
				Enhancements: []string{"Variable UX fixes (#842)",
					"legend options editor UX improvements (#845)",
					"Make it possible to adjust the height of the time range controls (#829)",
				},
				BugFixes:        []string{"Fix time units display, allow decimalPlaces to be used (#837)"},
				BreakingChanges: []string{"legend.position now required in time series panel (#848)"},
				Docs:            []string{"Complete documentation about the API. (#1471) (##1479) (##1483) (#1490) (#1491) (#1500)"},
				Unknown:         []string{"Use exact versions for internal npm dependencies (#846)", "Support snapshot UI releases (#844)"},
			},
			expected: fmt.Sprintf("%s\n%s", title, `
- [FEATURE] Discard Changes Confirmation Dialog (#834)
- [ENHANCEMENT] Variable UX fixes (#842)
- [ENHANCEMENT] legend options editor UX improvements (#845)
- [ENHANCEMENT] Make it possible to adjust the height of the time range controls (#829)
- [BUGFIX] Fix time units display, allow decimalPlaces to be used (#837)
- [BREAKINGCHANGE] legend.position now required in time series panel (#848)
- [DOC] Complete documentation about the API. (#1471) (##1479) (##1483) (#1490) (#1491) (#1500)

[//]: <UNKNOWN ENTRIES. Release shepherd, please review the following list and categorize them or remove them>

- [UNKNOWN] Use exact versions for internal npm dependencies (#846)
- [UNKNOWN] Support snapshot UI releases (#844)
`),
		},
	}
	for _, test := range testSuite {
		t.Run(test.title, func(t *testing.T) {
			assert.Equal(t, test.expected, generateChangelog(test.clog, "0.20.0"))
		})
	}
}

func TestWrite(t *testing.T) {
	clog := &changelog.Changelog{
		Features: []string{"add new widget (#10)"},
		BugFixes: []string{"fix crash on startup (#9)"},
	}

	now := time.Now()
	date := now.Format("2006-01-02")
	newSection := fmt.Sprintf("## 1.1.0 / %s\n\n- [FEATURE] add new widget (#10)\n- [BUGFIX] fix crash on startup (#9)\n", date)

	testSuite := []struct {
		title           string
		existingContent string
		expected        string
	}{
		{
			title:           "changelog without top-level title (operator-style)",
			existingContent: "## 1.0.0 / 2026-01-01\n\n- [FEATURE] initial release (#1)\n",
			expected:        fmt.Sprintf("%s\n## 1.0.0 / 2026-01-01\n\n- [FEATURE] initial release (#1)\n", newSection),
		},
		{
			title:           "changelog with top-level title",
			existingContent: "# Changelog\n\n## 1.0.0 / 2026-01-01\n\n- [FEATURE] initial release (#1)\n",
			expected:        fmt.Sprintf("# Changelog\n\n%s\n## 1.0.0 / 2026-01-01\n\n- [FEATURE] initial release (#1)\n", newSection),
		},
	}

	for _, test := range testSuite {
		t.Run(test.title, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "CHANGELOG.md")
			require.NoError(t, os.WriteFile(path, []byte(test.existingContent), 0600))

			origDir, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(dir))
			t.Cleanup(func() { _ = os.Chdir(origDir) })

			Write(clog, "1.1.0")

			result, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, test.expected, string(result))
		})
	}
}
