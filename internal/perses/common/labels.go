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
)

func LabelsForPerses(name string, instanceName string) map[string]string {
	var imageTag string
	image, err := ImageForPerses()
	if err == nil {
		imageTag = strings.Split(image, ":")[1]
	}
	return map[string]string{"app.kubernetes.io/name": name,
		"app.kubernetes.io/instance":   instanceName,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "perses-operator",
		"app.kubernetes.io/created-by": "controller-manager",
		"app.kubernetes.io/managed-by": "perses-operator",
	}
}

// imageForPerses gets the Operand image which is managed by this controller
// from the Perses_IMAGE environment variable defined in the config/manager/manager.yaml
func ImageForPerses() (string, error) {
	var imageEnvVar = "PERSES_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if !found {
		return "", fmt.Errorf("unable to find %s environment variable with the image", imageEnvVar)
	}

	imageParts := strings.Split(image, ":")
	if len(imageParts) < 2 {
		return "", fmt.Errorf("image provided for perses %s does not have a tag version", imageEnvVar)
	}
	return image, nil
}

func GetConfigName(instanceName string) string {
	return fmt.Sprintf("%s-config", instanceName)
}
