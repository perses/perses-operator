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

package v1alpha1

import (
	"fmt"

	"github.com/brunoga/deep"
	"github.com/perses/perses/pkg/model/api/config"
)

type PersesConfig struct {
	config.Config `json:",inline"`
}

func (in *PersesConfig) DeepCopyInto(out *PersesConfig) {
	if in == nil {
		return
	}

	copied, err := deep.Copy(in)
	if err != nil {
		panic(fmt.Errorf("failed to deep copy PersesConfig: %w", err))
	}
	*out = *copied
}
