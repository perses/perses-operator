package v1alpha1

import (
	"k8s.io/apimachinery/pkg/conversion"
	conv "sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// ConvertTo converts PersesDatasource (v1alpha1) to the Hub version (v1alpha2)
//
// NOTE: BasicAuth.PasswordPath uses different JSON field names between versions:
//   - v1alpha1: "password_path" (snake_case for backward compatibility)
//   - v1alpha2: "passwordPath" (camelCase per Kubernetes conventions)
//
// The conversion is handled automatically since the Go field name is identical.
func (src *PersesDatasource) ConvertTo(dstRaw conv.Hub) error {
	dst := dstRaw.(*v1alpha2.PersesDatasource)

	return Convert_v1alpha1_PersesDatasource_To_v1alpha2_PersesDatasource(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha2) to PersesDatasource (v1alpha1)
func (dst *PersesDatasource) ConvertFrom(srcRaw conv.Hub) error {
	src := srcRaw.(*v1alpha2.PersesDatasource)

	return Convert_v1alpha2_PersesDatasource_To_v1alpha1_PersesDatasource(src, dst, nil)
}

// Manual conversions
// Convert_v1alpha2_DatasourceSpec_To_v1alpha1_DatasourceSpec converts a v1alpha2 DatasourceSpec to v1alpha1 DatasourceSpec.
func Convert_v1alpha2_DatasourceSpec_To_v1alpha1_DatasourceSpec(in *v1alpha2.DatasourceSpec, out *DatasourceSpec, s conversion.Scope) error {
	out.Config.DatasourceSpec = in.Config.DatasourceSpec
	return nil
}
