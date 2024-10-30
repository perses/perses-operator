package v1alpha1

import (
	"encoding/json"

	persesv1 "github.com/perses/perses/pkg/model/api/v1"
)

type Dashboard struct {
	persesv1.DashboardSpec `json:",inline"`
}

// DeepCopyInto is a manually implemented deep copy function and this is required because:
// 1. The embedded persesv1.DashboardSpec from the Perses project doesn't implement DeepCopyInto
// 2. controller-gen can't automatically generate DeepCopy methods for types it doesn't own
func (in *Dashboard) DeepCopyInto(out *Dashboard) {
	*out = *in
	// Create a deep copy of the embedded DashboardSpec
	outSpec := persesv1.DashboardSpec{}
	bytes, _ := json.Marshal(in.DashboardSpec)
	_ = json.Unmarshal(bytes, &outSpec)
	out.DashboardSpec = outSpec
}
