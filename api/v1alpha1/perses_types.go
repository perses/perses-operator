/*
Copyright 2023 The Perses Authors.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PersesSpec defines the desired state of Perses
type PersesSpec struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Metadata *Metadata `json:"metadata,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Config PersesConfig `json:"config,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerPort int32 `json:"containerPort,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Replicas *int32 `json:"replicas,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// Metadata to add to deployed pods
type Metadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PersesStatus defines the observed state of Perses
type PersesStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Perses is the Schema for the perses API
type Perses struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PersesSpec   `json:"spec,omitempty"`
	Status PersesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PersesList contains a list of Perses
type PersesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Perses `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Perses{}, &PersesList{})
}
