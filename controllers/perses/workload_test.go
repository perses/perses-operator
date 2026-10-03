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

package perses

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apiserver/pkg/storage/names"
)

var _ = Describe("generateNamePrefix", func() {
	It("appends a dash to short names", func() {
		Expect(generateNamePrefix("perses")).To(Equal("perses-"))
	})

	It("keeps the generated name within the DNS label limit", func() {
		long := strings.Repeat("a", validation.DNS1123LabelMaxLength)
		prefix := generateNamePrefix(long)

		Expect(prefix).To(HaveSuffix("-"))
		Expect(prefix).To(HaveLen(names.MaxGeneratedNameLength))
		Expect(prefix).To(HavePrefix(long[:10]))
	})
})
