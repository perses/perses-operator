/*
Copyright 2025.

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

// PersesGlobalDatasourceStatus defines the observed state of PersesGlobalDatasource
type PersesGlobalDatasourceStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=pergds
//+kubebuilder:conversion:hub
//+versionName=v1alpha2
//+kubebuilder:storageversion

// PersesGlobalDatasource is the Schema for the PersesGlobalDatasources API
type PersesGlobalDatasource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatasourceSpec               `json:"spec,omitempty"`
	Status PersesGlobalDatasourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PersesGlobalDatasourceList contains a list of PersesGlobalDatasource
type PersesGlobalDatasourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PersesGlobalDatasource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PersesGlobalDatasource{}, &PersesGlobalDatasourceList{})
}
