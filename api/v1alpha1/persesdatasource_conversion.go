package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// ConvertTo converts PersesDatasource (v1alpha1) to the Hub version (v1alpha2)
func (src *PersesDatasource) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.PersesDatasource)

	return Convert_v1alpha1_PersesDatasource_To_v1alpha2_PersesDatasource(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha2) to PersesDatasource (v1alpha1)
func (dst *PersesDatasource) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.PersesDatasource)

	return Convert_v1alpha2_PersesDatasource_To_v1alpha1_PersesDatasource(src, dst, nil)
}
