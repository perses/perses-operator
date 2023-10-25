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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=json;yaml
type FileExtension string

type File struct {
	Folder    string        `json:"folder" yaml:"folder"`
	Extension FileExtension `json:"extension" yaml:"extension"`
}

type Database struct {
	File *File `json:"file,omitempty" yaml:"file,omitempty"`
}

type Schemas struct {
	// +kubebuilder:validation:optional
	PanelsPath string `json:"panels_path,omitempty" yaml:"panels_path,omitempty"`
	// +kubebuilder:validation:optional
	QueriesPath string `json:"queries_path,omitempty" yaml:"queries_path,omitempty"`
	// +kubebuilder:validation:optional
	DatasourcesPath string `json:"datasources_path,omitempty" yaml:"datasources_path,omitempty"`
	// +kubebuilder:validation:optional
	VariablesPath string `json:"variables_path,omitempty" yaml:"variables_path,omitempty"`
	// +kubebuilder:validation:optional
	Interval time.Duration `json:"interval,omitempty" yaml:"interval,omitempty"`
}

// TODO: import this from https://github.com/perses/perses/blob/main/internal/api/config/config.go#L51
type PersesConfig struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:validation:optional
	Readonly bool     `json:"readonly" yaml:"readonly"`
	Database Database `json:"database" yaml:"database"`
	Schemas  Schemas  `json:"schemas" yaml:"schemas"`
}

// PersesSpec defines the desired state of Perses
type PersesSpec struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Config PersesConfig `json:"config,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerPort int32 `json:"containerPort,omitempty"`
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
