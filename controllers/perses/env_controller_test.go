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
	"testing"

	"github.com/rhobs/perses-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func newPersesWithEnv(env []corev1.EnvVar, envFrom []corev1.EnvFromSource) *v1alpha2.Perses {
	image := "docker.io/persesdev/perses:latest"
	return &v1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.PersesSpec{
			Image:   &image,
			Env:     env,
			EnvFrom: envFrom,
		},
	}
}

func newEnvReconciler(t *testing.T) *PersesReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha2 to scheme: %v", err)
	}
	return &PersesReconciler{Scheme: scheme}
}

func TestCreatePersesDeployment_EnvLiteralValue(t *testing.T) {
	perses := newPersesWithEnv(
		[]corev1.EnvVar{
			{Name: "PERSES_SECURITY_AUTHENTICATION_PROVIDERS_OIDC_0_CLIENT_ID", Value: "my-client-id"},
		},
		nil,
	)

	dep, err := newEnvReconciler(t).createPersesDeployment(perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := dep.Spec.Template.Spec.Containers[0]
	if len(container.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(container.Env))
	}
	if container.Env[0].Name != "PERSES_SECURITY_AUTHENTICATION_PROVIDERS_OIDC_0_CLIENT_ID" {
		t.Errorf("env name = %q, want PERSES_SECURITY_AUTHENTICATION_PROVIDERS_OIDC_0_CLIENT_ID", container.Env[0].Name)
	}
	if container.Env[0].Value != "my-client-id" {
		t.Errorf("env value = %q, want my-client-id", container.Env[0].Value)
	}
}

func TestCreatePersesDeployment_EnvValueFrom(t *testing.T) {
	perses := newPersesWithEnv(
		[]corev1.EnvVar{
			{
				Name: "PERSES_SECURITY_AUTHENTICATION_PROVIDERS_OIDC_0_CLIENT_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "perses-oidc"},
						Key:                  "oidc-client-id",
					},
				},
			},
		},
		nil,
	)

	dep, err := newEnvReconciler(t).createPersesDeployment(perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := dep.Spec.Template.Spec.Containers[0]
	if len(container.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(container.Env))
	}
	ref := container.Env[0].ValueFrom
	if ref == nil || ref.SecretKeyRef == nil {
		t.Fatal("expected SecretKeyRef to be set")
	}
	if ref.SecretKeyRef.Name != "perses-oidc" {
		t.Errorf("SecretKeyRef.Name = %q, want perses-oidc", ref.SecretKeyRef.Name)
	}
	if ref.SecretKeyRef.Key != "oidc-client-id" {
		t.Errorf("SecretKeyRef.Key = %q, want oidc-client-id", ref.SecretKeyRef.Key)
	}
}

func TestCreatePersesDeployment_EnvFrom(t *testing.T) {
	perses := newPersesWithEnv(
		nil,
		[]corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "perses-oidc-env"},
				},
			},
		},
	)

	dep, err := newEnvReconciler(t).createPersesDeployment(perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := dep.Spec.Template.Spec.Containers[0]
	if len(container.EnvFrom) != 1 {
		t.Fatalf("expected 1 envFrom entry, got %d", len(container.EnvFrom))
	}
	if container.EnvFrom[0].SecretRef == nil {
		t.Fatal("expected SecretRef to be set")
	}
	if container.EnvFrom[0].SecretRef.Name != "perses-oidc-env" {
		t.Errorf("SecretRef.Name = %q, want perses-oidc-env", container.EnvFrom[0].SecretRef.Name)
	}
}

func TestCreatePersesDeployment_NoEnv(t *testing.T) {
	perses := newPersesWithEnv(nil, nil)

	dep, err := newEnvReconciler(t).createPersesDeployment(perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := dep.Spec.Template.Spec.Containers[0]
	if len(container.Env) != 0 {
		t.Errorf("expected no env vars, got %d", len(container.Env))
	}
	if len(container.EnvFrom) != 0 {
		t.Errorf("expected no envFrom, got %d", len(container.EnvFrom))
	}
}

func TestCreatePersesStatefulSet_EnvLiteralValue(t *testing.T) {
	perses := newPersesWithEnv(
		[]corev1.EnvVar{
			{Name: "PERSES_SECURITY_AUTHENTICATION_PROVIDERS_OIDC_0_CLIENT_ID", Value: "my-client-id"},
		},
		nil,
	)

	ss, err := newEnvReconciler(t).createPersesStatefulSet(perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := ss.Spec.Template.Spec.Containers[0]
	if len(container.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(container.Env))
	}
	if container.Env[0].Name != "PERSES_SECURITY_AUTHENTICATION_PROVIDERS_OIDC_0_CLIENT_ID" {
		t.Errorf("env name = %q, want PERSES_SECURITY_AUTHENTICATION_PROVIDERS_OIDC_0_CLIENT_ID", container.Env[0].Name)
	}
	if container.Env[0].Value != "my-client-id" {
		t.Errorf("env value = %q, want my-client-id", container.Env[0].Value)
	}
}

func TestCreatePersesStatefulSet_EnvValueFrom(t *testing.T) {
	perses := newPersesWithEnv(
		[]corev1.EnvVar{
			{
				Name: "PERSES_SECURITY_AUTHENTICATION_PROVIDERS_OIDC_0_CLIENT_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "perses-oidc"},
						Key:                  "oidc-client-id",
					},
				},
			},
		},
		nil,
	)

	ss, err := newEnvReconciler(t).createPersesStatefulSet(perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := ss.Spec.Template.Spec.Containers[0]
	if len(container.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(container.Env))
	}
	ref := container.Env[0].ValueFrom
	if ref == nil || ref.SecretKeyRef == nil {
		t.Fatal("expected SecretKeyRef to be set")
	}
	if ref.SecretKeyRef.Name != "perses-oidc" {
		t.Errorf("SecretKeyRef.Name = %q, want perses-oidc", ref.SecretKeyRef.Name)
	}
	if ref.SecretKeyRef.Key != "oidc-client-id" {
		t.Errorf("SecretKeyRef.Key = %q, want oidc-client-id", ref.SecretKeyRef.Key)
	}
}

func TestCreatePersesStatefulSet_EnvFrom(t *testing.T) {
	perses := newPersesWithEnv(
		nil,
		[]corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "perses-oidc-env"},
				},
			},
		},
	)

	ss, err := newEnvReconciler(t).createPersesStatefulSet(perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := ss.Spec.Template.Spec.Containers[0]
	if len(container.EnvFrom) != 1 {
		t.Fatalf("expected 1 envFrom entry, got %d", len(container.EnvFrom))
	}
	if container.EnvFrom[0].SecretRef == nil {
		t.Fatal("expected SecretRef to be set")
	}
	if container.EnvFrom[0].SecretRef.Name != "perses-oidc-env" {
		t.Errorf("SecretRef.Name = %q, want perses-oidc-env", container.EnvFrom[0].SecretRef.Name)
	}
}

func TestCreatePersesStatefulSet_NoEnv(t *testing.T) {
	perses := newPersesWithEnv(nil, nil)

	ss, err := newEnvReconciler(t).createPersesStatefulSet(perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := ss.Spec.Template.Spec.Containers[0]
	if len(container.Env) != 0 {
		t.Errorf("expected no env vars, got %d", len(container.Env))
	}
	if len(container.EnvFrom) != 0 {
		t.Errorf("expected no envFrom, got %d", len(container.EnvFrom))
	}
}
