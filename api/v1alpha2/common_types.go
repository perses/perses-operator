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
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// ProvisioningSecretPrefix is the prefix for provisioning secrets volume names
const provisioningSecretPrefix = "provisioning-"

// volumeNameRegex is used to replace non-alphanumeric characters in volume names with hyphens.
var volumeNameRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Provisioning configuration for provisioning secrets
type Provisioning struct {
	// secretRefs is a list of references to Kubernetes secrets used for provisioning sensitive data.
	// +optional
	//nolint:kubeapilinter // SecretKeySelector fields are defined in an external type
	SecretRefs []*ProvisioningSecret `json:"secretRefs,omitempty"`
}

type ProvisioningSecret struct {
	corev1.SecretKeySelector `json:",inline"`
}

func (p *ProvisioningSecret) String() string {
	return fmt.Sprintf("%s-%s", p.Name, p.Key)
}

// GetSecretVolumeName returns a valid volume name for the provisioning secret
func (p *ProvisioningSecret) GetSecretVolumeName() string {
	name := strings.ToLower(p.String())

	name = volumeNameRegex.ReplaceAllString(name, "-")

	name = strings.Trim(name, "-")

	return provisioningSecretPrefix + name
}

// SecretVersion represents a secret version
type SecretVersion struct {
	// name is the name of the provisioning secret
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name,omitempty"`
	// version is the resource version of the provisioning secret
	// +required
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version,omitempty"`
}
