package v1alpha2

import (
	"fmt"

	"github.com/brunoga/deep"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
)

// Datasource represents the Perses datasource configuration including
// display metadata, default flag, and plugin-specific settings.
type Datasource struct {
	persesv1.DatasourceSpec `json:",inline"`
}

func (in *Datasource) DeepCopyInto(out *Datasource) {
	if in == nil {
		return
	}

	copied, err := deep.Copy(in)
	if err != nil {
		panic(fmt.Errorf("failed to deep copy Datasource: %w", err))
	}
	*out = *copied
}
