// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"github.com/perses/common/set"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseTags", func() {
	DescribeTable("parses perses.dev/tags annotation correctly",
		func(annotations map[string]string, expected set.Set[string]) {
			result := ParseTags(annotations)
			if expected == nil {
				Expect(result).To(BeNil())
			} else {
				Expect(result).To(Equal(expected))
			}
		},
		Entry("nil annotations",
			(map[string]string)(nil), nil),
		Entry("empty annotations",
			map[string]string{}, nil),
		Entry("annotation not present",
			map[string]string{"other": "value"}, nil),
		Entry("empty annotation value",
			map[string]string{TagsAnnotation: ""},
			set.New[string]()),
		Entry("whitespace-only annotation value",
			map[string]string{TagsAnnotation: "   "},
			set.New[string]()),
		Entry("single tag",
			map[string]string{TagsAnnotation: "oncall"},
			set.New("oncall")),
		Entry("multiple tags",
			map[string]string{TagsAnnotation: "oncall,high_severity"},
			set.New("oncall", "high_severity")),
		Entry("tags with whitespace trimmed",
			map[string]string{TagsAnnotation: " oncall , high_severity "},
			set.New("oncall", "high_severity")),
		Entry("empty entries skipped",
			map[string]string{TagsAnnotation: "oncall,,high_severity"},
			set.New("oncall", "high_severity")),
		Entry("trailing comma ignored",
			map[string]string{TagsAnnotation: "oncall,high_severity,"},
			set.New("oncall", "high_severity")),
		Entry("single tag with whitespace",
			map[string]string{TagsAnnotation: "  oncall  "},
			set.New("oncall")),
		Entry("uppercase tags normalized to lowercase",
			map[string]string{TagsAnnotation: "OnCall,HIGH_SEVERITY"},
			set.New("oncall", "high_severity")),
		Entry("mixed case with whitespace",
			map[string]string{TagsAnnotation: " Production , Staging "},
			set.New("production", "staging")),
		Entry("duplicate tags after normalization",
			map[string]string{TagsAnnotation: "oncall,OnCall,ONCALL"},
			set.New("oncall")),
	)
})
