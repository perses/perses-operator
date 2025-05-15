package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// ConvertTo converts Perses (v1alpha1) to the Hub version (v1alpha2)
func (src *Perses) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.Perses)

	return Convert_v1alpha1_Perses_To_v1alpha2_Perses(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha2) to Perses (v1alpha1)
func (dst *Perses) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.Perses)

	return Convert_v1alpha2_Perses_To_v1alpha1_Perses(src, dst, nil)
}
