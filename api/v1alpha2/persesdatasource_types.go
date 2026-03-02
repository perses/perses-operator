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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PersesDatasourceStatus defines the observed state of PersesDatasource
type PersesDatasourceStatus struct {
	// conditions represent the latest observations of the PersesDatasource resource state
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// DatasourceSpec defines the desired state of a Perses datasource
type DatasourceSpec struct {
	// config specifies the Perses datasource configuration
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +required
	//nolint:kubeapilinter // Datasource uses flexible JSON schema; struct-level required fields are not applicable
	Config Datasource `json:"config"`
	// client specifies authentication and TLS configuration for the datasource
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Client *Client `json:"client,omitempty"`
	// instanceSelector selects Perses instances where this datasource will be created
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	InstanceSelector *metav1.LabelSelector `json:"instanceSelector,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=perds
//+kubebuilder:conversion:hub
//+versionName=v1alpha2
//+kubebuilder:storageversion

// PersesDatasource is the Schema for the PersesDatasources API
type PersesDatasource struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard Kubernetes ObjectMeta
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the PersesDatasource resource
	// +required
	Spec DatasourceSpec `json:"spec,omitzero"`
	// status is the observed state of the PersesDatasource resource
	// +optional
	//nolint:kubeapilinter // non-pointer Status is the standard pattern for Kubernetes controllers
	Status PersesDatasourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PersesDatasourceList contains a list of PersesDatasource
type PersesDatasourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PersesDatasource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PersesDatasource{}, &PersesDatasourceList{})
}
