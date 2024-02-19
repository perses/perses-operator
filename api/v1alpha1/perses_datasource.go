package v1alpha1

import (
	"github.com/barkimedes/go-deepcopy"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
)

type Datasource struct {
	persesv1.DatasourceSpec `json:",inline"`
}

func (in *Datasource) DeepCopyInto(out *Datasource) {
	temp, err := deepcopy.Anything(in)

	if err != nil {
		panic(err)
	}

	*out = *(temp.(*Datasource))
}
