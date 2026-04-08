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

package perses

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/perses/perses-operator/api/v1alpha2"
)

func TestFindPersesForSecret_WithPartialObjectMetadata(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	perses := &v1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-perses",
			Namespace: "test-ns",
		},
		Spec: v1alpha2.PersesSpec{
			Provisioning: &v1alpha2.Provisioning{
				SecretRefs: []*v1alpha2.ProvisioningSecret{
					{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "my-secret",
							},
							Key: "password",
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(perses).Build()
	reconciler := &PersesReconciler{Client: fakeClient}

	tests := []struct {
		name           string
		secretName     string
		secretNs       string
		expectRequests int
	}{
		{
			name:           "matching secret triggers reconciliation",
			secretName:     "my-secret",
			secretNs:       "test-ns",
			expectRequests: 1,
		},
		{
			name:           "non-matching secret name returns empty",
			secretName:     "other-secret",
			secretNs:       "test-ns",
			expectRequests: 0,
		},
		{
			name:           "matching name in different namespace returns empty",
			secretName:     "my-secret",
			secretNs:       "other-ns",
			expectRequests: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// WatchesMetadata passes PartialObjectMetadata instead of *corev1.Secret
			obj := &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.secretName,
					Namespace: tt.secretNs,
				},
			}
			obj.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

			requests := reconciler.findPersesForSecret(context.Background(), obj)
			assert.Len(t, requests, tt.expectRequests)
			if tt.expectRequests > 0 {
				assert.Equal(t, "test-perses", requests[0].Name)
				assert.Equal(t, "test-ns", requests[0].Namespace)
			}
		})
	}
}

func TestFindPersesForSecret_MultiplePersesReferencingSameSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	perses1 := &v1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "perses-one",
			Namespace: "test-ns",
		},
		Spec: v1alpha2.PersesSpec{
			Provisioning: &v1alpha2.Provisioning{
				SecretRefs: []*v1alpha2.ProvisioningSecret{
					{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "shared-secret",
							},
							Key: "password",
						},
					},
				},
			},
		},
	}

	perses2 := &v1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "perses-two",
			Namespace: "test-ns",
		},
		Spec: v1alpha2.PersesSpec{
			Provisioning: &v1alpha2.Provisioning{
				SecretRefs: []*v1alpha2.ProvisioningSecret{
					{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "shared-secret",
							},
							Key: "token",
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(perses1, perses2).Build()
	reconciler := &PersesReconciler{Client: fakeClient}

	obj := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-secret",
			Namespace: "test-ns",
		},
	}
	obj.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

	requests := reconciler.findPersesForSecret(context.Background(), obj)
	assert.Len(t, requests, 2)

	names := []string{requests[0].Name, requests[1].Name}
	assert.Contains(t, names, "perses-one")
	assert.Contains(t, names, "perses-two")
}

func TestFindPersesForSecret_NoPersesInNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha2.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &PersesReconciler{Client: fakeClient}

	obj := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "any-secret",
			Namespace: "empty-ns",
		},
	}

	requests := reconciler.findPersesForSecret(context.Background(), obj)
	assert.Empty(t, requests)
}

func TestFindPersesForSecret_PersesWithoutProvisioning(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha2.AddToScheme(scheme)

	perses := &v1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-perses",
			Namespace: "test-ns",
		},
		Spec: v1alpha2.PersesSpec{},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(perses).Build()
	reconciler := &PersesReconciler{Client: fakeClient}

	obj := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "test-ns",
		},
	}

	requests := reconciler.findPersesForSecret(context.Background(), obj)
	assert.Empty(t, requests)
}
