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

package common

import (
	"fmt"
	"strings"

	"github.com/perses/perses-operator/api/v1alpha2"
	"k8s.io/apimachinery/pkg/util/validation"
)

func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func sanitizeLabel(label string) string {
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		":", "-",
	)
	sanitized := replacer.Replace(strings.ToLower(label))

	if len(sanitized) > 0 && !isAlphaNumeric(rune(sanitized[0])) {
		sanitized = "x" + sanitized[1:]
	}
	if len(sanitized) > validation.LabelValueMaxLength {
		sanitized = sanitized[:validation.LabelValueMaxLength]
	}
	if len(sanitized) > 0 && !isAlphaNumeric(rune(sanitized[len(sanitized)-1])) {
		sanitized = sanitized[:len(sanitized)-1] + "x"
	}

	return sanitized
}

// SelectorLabelsForPerses returns the operator-set labels that identify the
// workload's pods. These are used for the immutable spec.selector.matchLabels
// of the Deployment and StatefulSet and for the Service selector, so they must
// never include user-supplied labels from spec.metadata.labels: those can
// change over the lifetime of the Perses instance (for example when a label
// key encodes a port), and a changed selector is rejected by the API server
// with "field is immutable". The operator-set labels are constants, and
// keeping all of them in the selector means instances without user labels
// keep the selector they were created with, and that pods of other operators
// using the same name and instance values are never selected.
// LabelsForPerses returns a superset of these for object and pod-template
// metadata.
func SelectorLabelsForPerses(name string, perses *v1alpha2.Perses) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       sanitizeLabel(name),
		"app.kubernetes.io/instance":   sanitizeLabel(perses.Name),
		"app.kubernetes.io/part-of":    "perses-operator",
		"app.kubernetes.io/created-by": "controller-manager",
		"app.kubernetes.io/managed-by": "perses-operator",
	}
}

func LabelsForPerses(name string, perses *v1alpha2.Perses) map[string]string {
	persesLabels := SelectorLabelsForPerses(name, perses)

	if perses.Spec.Metadata != nil {
		for label, value := range perses.Spec.Metadata.Labels {
			// don't overwrite default labels
			if _, ok := persesLabels[label]; !ok {
				persesLabels[label] = value
			}
		}
	}

	return persesLabels

}

// ImageForPerses resolves the Perses server image in priority order:
//  1. spec.image from the Perses CR (highest)
//  2. --perses-default-base-image flag (persesImageFromFlags)
func ImageForPerses(perses *v1alpha2.Perses, persesImageFromFlags string) (string, error) {
	var image string
	switch {
	case perses.Spec.Image != nil && *perses.Spec.Image != "":
		image = *perses.Spec.Image
	case persesImageFromFlags != "":
		image = persesImageFromFlags
	default:
		return "", fmt.Errorf("no image specified: set spec.image in the Perses CR or pass --perses-default-base-image")
	}

	if !strings.Contains(image, ":") {
		return "", fmt.Errorf("image %q must include a tag (e.g. :v1.0.0)", image)
	}

	return image, nil
}

func GetConfigName(instanceName string) string {
	return fmt.Sprintf("%s-config", instanceName)
}

func GetStorageName(instanceName string) string {
	return fmt.Sprintf("%s-storage", instanceName)
}
