package v1alpha1

import (
	"k8s.io/apimachinery/pkg/conversion"
	conv "sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// ConvertTo converts Perses (v1alpha1) to the Hub version (v1alpha2)
func (src *Perses) ConvertTo(dstRaw conv.Hub) error {
	dst := dstRaw.(*v1alpha2.Perses)

	return Convert_v1alpha1_Perses_To_v1alpha2_Perses(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha2) to Perses (v1alpha1)
func (dst *Perses) ConvertFrom(srcRaw conv.Hub) error {
	src := srcRaw.(*v1alpha2.Perses)

	return Convert_v1alpha2_Perses_To_v1alpha1_Perses(src, dst, nil)
}

// Convert_v1alpha2_PersesSpec_To_v1alpha1_PersesSpec converts a PersesSpec from v1alpha2 to v1alpha1.
func Convert_v1alpha2_PersesSpec_To_v1alpha1_PersesSpec(in *v1alpha2.PersesSpec, out *PersesSpec, s conversion.Scope) error {
	// NOTE: PodSecurityContext is not supported in v1alpha1, it will be dropped during conversion
	return autoConvert_v1alpha2_PersesSpec_To_v1alpha1_PersesSpec(in, out, s)
}
