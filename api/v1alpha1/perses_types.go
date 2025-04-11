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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PersesSpec defines the desired state of Perses
type PersesSpec struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Metadata *Metadata `json:"metadata,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	// Perses client configuration
	Client *Client `json:"client,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Config PersesConfig `json:"config,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Args extra arguments to pass to perses
	Args []string `json:"args,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerPort int32 `json:"containerPort,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	// Image specifies the container image that should be used for the Perses deployment.
	Image string `json:"image,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	// service specifies the service configuration for the perses instance
	Service *PersesService `json:"service,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	// tls specifies the tls configuration for the perses instance
	TLS *TLS `json:"tls,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default:={size: "1Gi"}
	// +optional
	// Storage configuration used by the StatefulSet
	Storage *StorageConfiguration `json:"storage,omitempty"`
}

// Metadata to add to deployed pods
type Metadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type PersesService struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Name string `json:"name,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type Client struct {
	// TLS the equivalent to the tls_config for perses client
	// +optional
	TLS *TLS `json:"tls,omitempty"`
}

type TLS struct {
	// Enable TLS connection to perses
	Enable bool `json:"enable"`
	// CaCert to verify the perses certificate
	// +optional
	CaCert *Certificate `json:"caCert,omitempty"`
	// UserCert client cert/key for mTLS
	// +optional
	UserCert *Certificate `json:"userCert,omitempty"`
	// InsecureSkipVerify skip verify of perses certificate
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// CertificateType types of certificate sources in k8s
type CertificateType string

const (
	CertificateTypeSecret    CertificateType = "secret"
	CertificateTypeConfigMap CertificateType = "configmap"
	CertificateTypeFile      CertificateType = "file"
)

type Certificate struct {
	// +kubebuilder:validation:Enum:={"secret", "configmap", "file"}
	// Type source type of certificate
	Type CertificateType `json:"type"`
	// Name of certificate k8s resource (when type is secret or configmap)
	// +optional
	Name string `json:"name,omitempty"`
	// Namsespace of certificate k8s resource (when type is secret or configmap)
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Path to Certificate
	CertPath string `json:"certPath"`
	// Path to Private key certificate
	// +optional
	PrivateKeyPath string `json:"privateKeyPath,omitempty"`
}

// StorageConfiguration is the configuration used to create and reconcile PVCs
type StorageConfiguration struct {
	// StorageClass to use for PVCs.
	// If not specified, will use the default storage class
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`
	// Size of the storage.
	// cannot be decreased.
	// +optional
	Size resource.Quantity `json:"size,omitempty"`
}

// PersesStatus defines the observed state of Perses
type PersesStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=per

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
