/*
Copyright The Perses Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PersesDashboardStatus defines the observed state of PersesDashboard
type PersesDashboardStatus struct {
	// conditions represent the latest observations of the PersesDashboard resource state
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// PersesDashboardSpec defines the desired state of PersesDashboard
type PersesDashboardSpec struct {
	// config specifies the Perses dashboard configuration
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +required
	//nolint:kubeapilinter // Dashboard uses flexible JSON schema; struct-level required fields are not applicable
	Config Dashboard `json:"config"`
	// instanceSelector selects Perses instances where this dashboard will be created
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	InstanceSelector *metav1.LabelSelector `json:"instanceSelector,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=perdb
//+kubebuilder:conversion:hub
//+versionName=v1alpha2
//+kubebuilder:storageversion

// PersesDashboard is the Schema for the persesdashboards API
type PersesDashboard struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard Kubernetes ObjectMeta
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the PersesDashboard resource
	// +required
	Spec PersesDashboardSpec `json:"spec,omitzero"`
	// status is the observed state of the PersesDashboard resource
	// +optional
	//nolint:kubeapilinter // non-pointer Status is the standard pattern for Kubernetes controllers
	Status PersesDashboardStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PersesDashboardList contains a list of PersesDashboard
type PersesDashboardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PersesDashboard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PersesDashboard{}, &PersesDashboardList{})
}
