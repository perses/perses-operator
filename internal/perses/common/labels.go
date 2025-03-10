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

package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/perses/perses-operator/api/v1alpha1"
)

func LabelsForPerses(persesImageFromFlags string, name string, perses *v1alpha1.Perses) (map[string]string, error) {
	instanceName := perses.Name
	var imageTag string
	image, err := ImageForPerses(perses, persesImageFromFlags)

	if err != nil {
		return nil, fmt.Errorf("unable to get the image for perses: %s", err)
	}

	if strings.Contains(image, ":") {
		imageTag = strings.Split(image, ":")[1]
	} else {
		imageTag = "latest"
	}

	persesLabels := map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   instanceName,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "perses-operator",
		"app.kubernetes.io/created-by": "controller-manager",
		"app.kubernetes.io/managed-by": "perses-operator",
	}

	if perses.Spec.Metadata != nil {
		for label, value := range perses.Spec.Metadata.Labels {
			// don't overwrite default labels
			if _, ok := persesLabels[label]; !ok {
				persesLabels[label] = value
			}
		}
	}

	return persesLabels, nil

}

// imageForPerses gets the Operand image which is managed by this controller
// from the image field in the CR or PERSES_IMAGE environment variable defined in the config/manager/manager.yaml
func ImageForPerses(perses *v1alpha1.Perses, persesImageFromFlags string) (string, error) {
	image := os.Getenv("PERSES_IMAGE")

	if persesImageFromFlags != "" {
		image = persesImageFromFlags
	}

	if len(perses.Spec.Image) > 0 {
		image = perses.Spec.Image
	}

	if image == "" {
		return "", fmt.Errorf("perses image operand was not provided")
	}

	imageParts := strings.Split(image, ":")

	if len(imageParts) < 2 {
		return "", fmt.Errorf("image provided for perses %s does not have a tag version", image)
	}

	return image, nil
}

func GetConfigName(instanceName string) string {
	return fmt.Sprintf("%s-config", instanceName)
}

func GetStorageName(instanceName string) string {
	return fmt.Sprintf("%s-storage", instanceName)
}
