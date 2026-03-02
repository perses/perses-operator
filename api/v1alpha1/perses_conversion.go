// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/conversion"
	conv "sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// ConvertTo converts Perses (v1alpha1) to the Hub version (v1alpha2)
//
// NOTE: BasicAuth.PasswordPath uses different JSON field names between versions:
//   - v1alpha1: "password_path" (snake_case for backward compatibility)
//   - v1alpha2: "passwordPath" (camelCase per Kubernetes conventions)
//
// The conversion is handled automatically since the Go field name is identical.
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
	// NOTE: The following v1alpha2 fields are not supported in v1alpha1 and will be dropped during conversion:
	// PodSecurityContext, LogLevel, LogMethodTrace, Provisioning, Volumes, VolumeMounts
	return autoConvert_v1alpha2_PersesSpec_To_v1alpha1_PersesSpec(in, out, s)
}

// Convert_v1alpha2_PersesStatus_To_v1alpha1_PersesStatus converts a PersesStatus from v1alpha2 to v1alpha1.
func Convert_v1alpha2_PersesStatus_To_v1alpha1_PersesStatus(in *v1alpha2.PersesStatus, out *PersesStatus, s conversion.Scope) error {
	// NOTE: Provisioning is not supported in v1alpha1, it will be dropped during conversion
	return autoConvert_v1alpha2_PersesStatus_To_v1alpha1_PersesStatus(in, out, s)
}

// Convert_v1alpha1_OAuth_To_v1alpha2_OAuth converts OAuth from v1alpha1 to v1alpha2.
func Convert_v1alpha1_OAuth_To_v1alpha2_OAuth(in *OAuth, out *v1alpha2.OAuth, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_OAuth_To_v1alpha2_OAuth(in, out, s); err != nil {
		return err
	}
	// Convert AuthStyle from int to *int32
	if in.AuthStyle != 0 {
		authStyle := int32(in.AuthStyle)
		out.AuthStyle = &authStyle
	}
	return nil
}

// Convert_v1alpha2_OAuth_To_v1alpha1_OAuth converts OAuth from v1alpha2 to v1alpha1.
func Convert_v1alpha2_OAuth_To_v1alpha1_OAuth(in *v1alpha2.OAuth, out *OAuth, s conversion.Scope) error {
	if err := autoConvert_v1alpha2_OAuth_To_v1alpha1_OAuth(in, out, s); err != nil {
		return err
	}
	// Convert AuthStyle from *int32 to int
	if in.AuthStyle != nil {
		out.AuthStyle = int(*in.AuthStyle)
	}
	return nil
}

// Convert_v1alpha1_StorageConfiguration_To_v1alpha2_StorageConfiguration converts v1alpha1 storage to v1alpha2.
// Migrates StorageClass and Size fields into PersistentVolumeClaimTemplate.
func Convert_v1alpha1_StorageConfiguration_To_v1alpha2_StorageConfiguration(in *StorageConfiguration, out *v1alpha2.StorageConfiguration, s conversion.Scope) error {
	// Only create PVC template if v1alpha1 has storage config
	if in.StorageClass == nil && in.Size.IsZero() {
		return nil
	}

	out.PersistentVolumeClaimTemplate = &corev1.PersistentVolumeClaimSpec{
		StorageClassName: in.StorageClass,
	}

	if !in.Size.IsZero() {
		out.PersistentVolumeClaimTemplate.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: in.Size,
		}
	}

	return nil
}

// Convert_v1alpha2_StorageConfiguration_To_v1alpha1_StorageConfiguration converts v1alpha2 storage to v1alpha1.
// Extracts StorageClass and Size from PersistentVolumeClaimTemplate.
func Convert_v1alpha2_StorageConfiguration_To_v1alpha1_StorageConfiguration(in *v1alpha2.StorageConfiguration, out *StorageConfiguration, s conversion.Scope) error {
	// Extract from PVC template if it exists
	if in.PersistentVolumeClaimTemplate != nil {
		out.StorageClass = in.PersistentVolumeClaimTemplate.StorageClassName

		if storage, ok := in.PersistentVolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage]; ok {
			out.Size = storage
		}
	}

	// EmptyDir cannot be converted to v1alpha1 (not supported)
	// It will be silently dropped

	return nil
}
