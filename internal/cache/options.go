// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	configv1 "github.com/openshift/api/config/v1"
	"github.com/perses/perses-operator/internal/perses/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ParseSecretLabelSelector parses a label selector string using the native
// Kubernetes label selector syntax (e.g. "key=value,key2!=value2,key3 in (a,b)").
// Returns nil for an empty input string.
func ParseSecretLabelSelector(raw string) (labels.Selector, error) {
	if raw == "" {
		return nil, nil
	}
	return labels.Parse(raw)
}

// BuildCacheByObject builds the cache.ByObject map for the manager's cache configuration.
//
// Operator-created resources (Deployment, StatefulSet, ConfigMap, Service) are filtered
// by the fixed label app.kubernetes.io/managed-by=perses-operator.
//
// Secrets are filtered by secretSelector. If secretSelector is nil (no flag provided),
// the default label perses.dev/watch=true is used. If watchAllSecrets is true, no label
// filter is applied to secrets (preserving pre-change behavior).
//
// Secret data is always stripped from the cache via Transform regardless of label filtering.
func BuildCacheByObject(secretSelector labels.Selector, watchAllSecrets bool, tlsClusterProfile bool) map[client.Object]cache.ByObject {
	managedBySelector := labels.SelectorFromSet(labels.Set{
		common.PersesManagedByLabel: common.PersesManagedByValue,
	})

	byObject := map[client.Object]cache.ByObject{
		&appsv1.Deployment{}: {
			Label: managedBySelector,
		},
		&appsv1.StatefulSet{}: {
			Label: managedBySelector,
		},
		&corev1.ConfigMap{}: {
			Label: managedBySelector,
		},
		&corev1.Service{}: {
			Label: managedBySelector,
		},
	}

	secretEntry := cache.ByObject{
		// Strip secret data from the cache to reduce memory usage.
		// All controllers watch or read secrets but only need metadata for change detection.
		// Controllers that need actual secret data use the APIReader instead.
		Transform: func(obj any) (any, error) {
			if secret, ok := obj.(*corev1.Secret); ok {
				secret.Data = nil
				secret.StringData = nil
			}
			return obj, nil
		},
	}

	if !watchAllSecrets {
		if secretSelector == nil {
			secretSelector = labels.SelectorFromSet(labels.Set{
				common.PersesWatchLabel: common.PersesWatchLabelValue,
			})
		}
		secretEntry.Label = secretSelector
	}

	byObject[&corev1.Secret{}] = secretEntry

	if tlsClusterProfile {
		byObject[&configv1.APIServer{}] = cache.ByObject{
			Label: labels.Everything(),
		}
	}

	return byObject
}
