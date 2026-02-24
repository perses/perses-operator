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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PersesSpec defines the desired state of Perses
type PersesSpec struct {
	// metadata specifies additional metadata to add to deployed pods
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Metadata *Metadata `json:"metadata,omitempty"`
	// client specifies the Perses client configuration
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Client *Client `json:"client,omitempty"`
	// config specifies the Perses server configuration
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Config PersesConfig `json:"config,omitempty"`
	// args are extra command-line arguments to pass to the Perses server
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Args []string `json:"args,omitempty"`
	// containerPort is the port on which the Perses server listens for HTTP requests
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ContainerPort *int32 `json:"containerPort,omitempty"`
	// replicas is the number of desired pod replicas for the Perses deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// resources defines the compute resources configured for the container
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// nodeSelector constrains pods to nodes with matching labels
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// tolerations allow pods to schedule onto nodes with matching taints
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// affinity specifies the pod's scheduling constraints
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// image specifies the container image that should be used for the Perses deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Image *string `json:"image,omitempty"`
	// service specifies the service configuration for the Perses instance
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Service *PersesService `json:"service,omitempty"`
	// livenessProbe specifies the liveness probe configuration for the Perses container
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`
	// readinessProbe specifies the readiness probe configuration for the Perses container
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
	// tls specifies the TLS configuration for the Perses instance
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	TLS *TLS `json:"tls,omitempty"`
	// storage configuration used by the StatefulSet
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Storage *StorageConfiguration `json:"storage,omitempty"`
	// serviceAccountName is the name of the ServiceAccount to use for the Perses deployment or statefulset
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`
	// podSecurityContext holds pod-level security attributes and common container settings
	// If not specified, defaults to fsGroup: 65534 to ensure proper volume permissions for the nobody user
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// logLevel defines the log level for Perses
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:validation:Enum=panic;fatal;error;warning;info;debug;trace
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`
	// logMethodTrace when true, includes the calling method as a field in the log
	// It can be useful to see immediately where the log comes from
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	LogMethodTrace *bool `json:"logMethodTrace,omitempty"`
	// provisioning configuration for provisioning secrets
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Provisioning *Provisioning `json:"provisioning,omitempty"`
}

// Metadata to add to deployed pods
type Metadata struct {
	// labels are key/value pairs attached to pods
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// annotations are key/value pairs attached to pods for non-identifying metadata
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PersesService defines service configuration for Perses
type PersesService struct {
	// name is the name of the Kubernetes Service resource
	// If not specified, a default name will be generated
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Name *string `json:"name,omitempty"`
	// annotations are key/value pairs attached to the Service for non-identifying metadata
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type Client struct {
	// basicAuth provides username/password authentication configuration for the Perses client
	// +optional
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
	// oauth provides OAuth 2.0 authentication configuration for the Perses client
	// +optional
	OAuth *OAuth `json:"oauth,omitempty"`
	// tls provides TLS/SSL configuration for secure connections to Perses
	// +optional
	TLS *TLS `json:"tls,omitempty"`
	// kubernetesAuth enables Kubernetes native authentication for the Perses client
	// +optional
	KubernetesAuth *KubernetesAuth `json:"kubernetesAuth,omitempty"`
}

type KubernetesAuth struct {
	// enable determines whether Kubernetes authentication is enabled for the Perses client
	// +optional
	Enable *bool `json:"enable,omitempty"`
}

type BasicAuth struct {
	SecretSource `json:",inline"`
	// username is the username credential for basic authentication
	// +required
	// +kubebuilder:validation:MinLength=1
	Username string `json:"username"`
	// passwordPath specifies the key name within the secret/configmap or filesystem path
	// (depending on SecretSource.Type) where the password is stored
	// +required
	// +kubebuilder:validation:MinLength=1
	PasswordPath string `json:"passwordPath"`
}

type OAuth struct {
	SecretSource `json:",inline"`
	// clientIDPath specifies the key name within the secret/configmap or filesystem path
	// (depending on SecretSource.Type) where the OAuth client ID is stored
	// +optional
	ClientIDPath *string `json:"clientIDPath,omitempty"`
	// clientSecretPath specifies the key name within the secret/configmap or filesystem path
	// (depending on SecretSource.Type) where the OAuth client secret is stored
	// +optional
	ClientSecretPath *string `json:"clientSecretPath,omitempty"`
	// tokenURL is the OAuth 2.0 provider's token endpoint URL
	// This is a constant specific to each OAuth provider
	// +required
	// +kubebuilder:validation:MinLength=1
	TokenURL string `json:"tokenURL"`
	// scopes specifies optional requested permissions for the OAuth token
	// +optional
	Scopes []string `json:"scopes,omitempty"`
	// endpointParams specifies additional parameters to include in requests to the token endpoint
	// +optional
	EndpointParams map[string][]string `json:"endpointParams,omitempty"`
	// authStyle specifies how the endpoint wants the client ID and client secret sent
	// The zero value means to auto-detect
	// +optional
	AuthStyle *int32 `json:"authStyle,omitempty"`
}

type TLS struct {
	// enable determines whether TLS is enabled for connections to Perses
	// +optional
	Enable *bool `json:"enable,omitempty"`
	// caCert specifies the CA certificate to verify the Perses server's certificate
	// +optional
	CaCert *Certificate `json:"caCert,omitempty"`
	// userCert specifies the client certificate and key for mutual TLS (mTLS) authentication
	// +optional
	UserCert *Certificate `json:"userCert,omitempty"`
	// insecureSkipVerify determines whether to skip verification of the Perses server's certificate
	// Setting this to true is insecure and should only be used for testing
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`
}

// SecretSourceType types of secret sources in k8s
type SecretSourceType string

const (
	SecretSourceTypeSecret    SecretSourceType = "secret"
	SecretSourceTypeConfigMap SecretSourceType = "configmap"
	SecretSourceTypeFile      SecretSourceType = "file"
)

// SecretSource configuration for a perses secret source
// +kubebuilder:validation:XValidation:rule="self.type != 'secret' && self.type != 'configmap' || has(self.name)",message="name is required when type is secret or configmap"
// +kubebuilder:validation:XValidation:rule="self.type != 'secret' && self.type != 'configmap' || has(self.namespace)",message="namespace is required when type is secret or configmap"
type SecretSource struct {
	// type specifies the source type for secret data (secret, configmap, or file)
	// +required
	// +kubebuilder:validation:Enum=secret;configmap;file
	Type SecretSourceType `json:"type"`
	// name is the name of the Kubernetes Secret or ConfigMap resource
	// Required when Type is "secret" or "configmap", ignored when Type is "file"
	// +optional
	// +kubebuilder:validation:MinLength=1
	Name *string `json:"name,omitempty"`
	// namespace is the namespace of the Kubernetes Secret or ConfigMap resource
	// Required when Type is "secret" or "configmap", ignored when Type is "file"
	// +optional
	// +kubebuilder:validation:MinLength=1
	Namespace *string `json:"namespace,omitempty"`
}

type Certificate struct {
	SecretSource `json:",inline"`
	// certPath specifies the key name within the secret/configmap or filesystem path
	// (depending on SecretSource.Type) where the certificate is stored
	// +required
	// +kubebuilder:validation:MinLength=1
	CertPath string `json:"certPath"`
	// privateKeyPath specifies the key name within the secret/configmap or filesystem path
	// (depending on SecretSource.Type) where the private key is stored
	// Required for client certificates (UserCert), optional for CA certificates (CaCert)
	// +optional
	PrivateKeyPath *string `json:"privateKeyPath,omitempty"`
}

// StorageConfiguration is the configuration used to create and reconcile PVCs
// +kubebuilder:validation:XValidation:rule="!(has(self.emptyDir) && has(self.pvcTemplate))",message="emptyDir and pvcTemplate are mutually exclusive"
type StorageConfiguration struct {
	// emptyDir to use for ephemeral storage.
	// When set, data will be lost when the pod is deleted or restarted.
	// Mutually exclusive with PersistentVolumeClaimTemplate.
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`

	// pvcTemplate is the template for PVCs that will be created.
	// Mutually exclusive with EmptyDir.
	// +optional
	PersistentVolumeClaimTemplate *corev1.PersistentVolumeClaimSpec `json:"pvcTemplate,omitempty"`
}

// PersesStatus defines the observed state of Perses
type PersesStatus struct {
	// conditions represent the latest observations of the Perses resource state
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// provisioning contains the versions of provisioning secrets currently in use
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +optional
	Provisioning []SecretVersion `json:"provisioning,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=per
//+kubebuilder:conversion:hub
//+versionName=v1alpha2
//+kubebuilder:storageversion

// Perses is the Schema for the perses API
type Perses struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard Kubernetes ObjectMeta
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the Perses resource
	Spec PersesSpec `json:"spec,omitempty"`
	// status is the observed state of the Perses resource
	Status PersesStatus `json:"status,omitempty"`
}

// RequiresDeployment returns true if the Perses instance should be deployed as a Deployment.
// This is the case when using SQL database OR file database with EmptyDir storage.
func (p *Perses) RequiresDeployment() bool {
	usesSQLDatabase := p.Spec.Config.Database.SQL != nil
	usesFileWithEmptyDir := p.Spec.Config.Database.File != nil &&
		p.Spec.Storage != nil && p.Spec.Storage.EmptyDir != nil
	return usesSQLDatabase || usesFileWithEmptyDir
}

// RequiresStatefulSet returns true if the Perses instance should be deployed as a StatefulSet.
// This is the case when using file database with persistent volume storage (not EmptyDir).
func (p *Perses) RequiresStatefulSet() bool {
	return p.Spec.Config.Database.File != nil &&
		(p.Spec.Storage == nil || p.Spec.Storage.EmptyDir == nil)
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
