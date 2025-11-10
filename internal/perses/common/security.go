/*
Copyright 2025 The Perses Authors.

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
	"github.com/perses/perses-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

func Int64Ptr(i int64) *int64 {
	return &i
}

func GetPodSecurityContext(perses *v1alpha2.Perses) *corev1.PodSecurityContext {

	// if user specified a custom security context, use it
	if perses.Spec.PodSecurityContext != nil {
		return perses.Spec.PodSecurityContext
	}

	// Return default security context
	// fsGroup 65534 is the "nobody" group, matching the container user
	return &corev1.PodSecurityContext{
		FSGroup: Int64Ptr(65534),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}
