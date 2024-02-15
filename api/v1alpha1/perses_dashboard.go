package v1alpha1

import (
	"github.com/barkimedes/go-deepcopy"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
)

type Dashboard struct {
	persesv1.Dashboard `json:",inline"`
}

func (in *Dashboard) DeepCopyInto(out *Dashboard) {
	temp, err := deepcopy.Anything(in)

	if err != nil {
		panic(err)
	}

	*out = *(temp.(*Dashboard))
}
