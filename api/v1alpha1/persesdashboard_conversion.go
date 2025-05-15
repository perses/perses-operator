package v1alpha1

import (
	"k8s.io/apimachinery/pkg/conversion"
	conv "sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// ConvertTo converts PersesDashboard (v1alpha1) to the Hub version (v1alpha2)
func (src PersesDashboard) ConvertTo(dstRaw conv.Hub) error {
	dst := dstRaw.(*v1alpha2.PersesDashboard)

	return Convert_v1alpha1_PersesDashboard_To_v1alpha2_PersesDashboard(&src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha2) to PersesDashboard (v1alpha1)
func (dst *PersesDashboard) ConvertFrom(srcRaw conv.Hub) error {
	src := srcRaw.(*v1alpha2.PersesDashboard)

	return Convert_v1alpha2_PersesDashboard_To_v1alpha1_PersesDashboard(src, dst, nil)
}

// Manual conversions
// Convert_v1alpha1_Dashboard_To_v1alpha2_PersesDashboardSpec converts a v1alpha1 Dashboard to v1alpha2 PersesDashboardSpec.
func Convert_v1alpha1_Dashboard_To_v1alpha2_PersesDashboardSpec(in *Dashboard, out *v1alpha2.PersesDashboardSpec, s conversion.Scope) error {
	out.Config.DashboardSpec = in.DashboardSpec
	return nil
}

// Convert_v1alpha2_PersesDashboardSpec_To_v1alpha1_Dashboard converts a PersesDashboardSpec from v1alpha2 to v1alpha1.
func Convert_v1alpha2_PersesDashboardSpec_To_v1alpha1_Dashboard(in *v1alpha2.PersesDashboardSpec, out *Dashboard, s conversion.Scope) error {
	out.DashboardSpec = in.Config.DashboardSpec
	return nil
}
