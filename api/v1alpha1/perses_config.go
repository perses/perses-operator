package v1alpha1

import (
	"github.com/barkimedes/go-deepcopy"
	"github.com/perses/perses/pkg/model/api/config"
)

type PersesConfig struct {
	config.Config `json:",inline"`
}

func (in *PersesConfig) DeepCopyInto(out *PersesConfig) {
	temp, err := deepcopy.Anything(in)

	if err != nil {
		panic(err)
	}

	*out = *(temp.(*PersesConfig))
}
