// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0

package common

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rhobs/perses-operator/api/v1alpha2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHasExplicitEncryptionKey(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "key", Namespace: "test"}, Data: map[string][]byte{"PERSES_SECURITY_ENCRYPTION_KEY": []byte(strings.Repeat("a", 32)), "value": []byte(strings.Repeat("b", 32))}}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "env", Namespace: "test"}, Data: map[string]string{"SECURITY_ENCRYPTION_KEY_FILE": "/external/key"}}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, cm).Build()
	cases := []struct {
		name    string
		setup   func(*v1alpha2.Perses)
		want    bool
		wantErr bool
	}{
		{"absent", func(p *v1alpha2.Perses) {}, false, false},
		{"inline", func(p *v1alpha2.Perses) { p.Spec.Config.Security.EncryptionKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }, true, false},
		{"file", func(p *v1alpha2.Perses) { p.Spec.Config.Security.EncryptionKeyFile = "/external/key" }, true, false},
		{"env literal", func(p *v1alpha2.Perses) {
			p.Spec.Env = []corev1.EnvVar{{Name: "PERSES_SECURITY_ENCRYPTION_KEY", Value: strings.Repeat("a", 32)}}
		}, true, false},
		{"empty explicit env", func(p *v1alpha2.Perses) {
			p.Spec.Env = []corev1.EnvVar{{Name: "PERSES_SECURITY_ENCRYPTION_KEY", Value: ""}}
		}, true, false},
		{"unrelated env", func(p *v1alpha2.Perses) { p.Spec.Env = []corev1.EnvVar{{Name: "PERSES_LOG_LEVEL", Value: "debug"}} }, false, false},
		{"secret envFrom", func(p *v1alpha2.Perses) {
			p.Spec.EnvFrom = []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "key"}}}}
		}, true, false},
		{"prefixed configmap", func(p *v1alpha2.Perses) {
			p.Spec.EnvFrom = []corev1.EnvFromSource{{Prefix: "PERSES_", ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "env"}}}}
		}, true, false},
		{"optional absent envFrom", func(p *v1alpha2.Perses) {
			p.Spec.EnvFrom = []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "missing"}, Optional: ptr.To(true)}}}
		}, false, false},
		{"required absent envFrom", func(p *v1alpha2.Perses) {
			p.Spec.EnvFrom = []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "missing"}}}}
		}, false, true},
		{"secretKeyRef", func(p *v1alpha2.Perses) {
			p.Spec.Env = []corev1.EnvVar{{Name: "PERSES_SECURITY_ENCRYPTION_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "key"}, Key: "value"}}}}
		}, true, false},
		{"required missing data key", func(p *v1alpha2.Perses) {
			p.Spec.Env = []corev1.EnvVar{{Name: "PERSES_SECURITY_ENCRYPTION_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "key"}, Key: "missing"}}}}
		}, false, true},
		{"optional absent key", func(p *v1alpha2.Perses) {
			p.Spec.Env = []corev1.EnvVar{{Name: "PERSES_SECURITY_ENCRYPTION_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "key"}, Key: "missing", Optional: ptr.To(true)}}}}
		}, false, false},
		{"optional env retains import", func(p *v1alpha2.Perses) {
			p.Spec.EnvFrom = []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "key"}}}}
			p.Spec.Env = []corev1.EnvVar{{Name: "PERSES_SECURITY_ENCRYPTION_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "missing"}, Key: "missing", Optional: ptr.To(true)}}}}
		}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Namespace: "test"}}
			tc.setup(p)
			got, err := HasExplicitEncryptionKey(context.Background(), reader, p)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestGetEncryptionKeyName(t *testing.T) {
	require.Equal(t, "example-encryption-key", GetEncryptionKeyName("example"))
	a := GetEncryptionKeyName(strings.Repeat("a", 240) + "a")
	b := GetEncryptionKeyName(strings.Repeat("a", 240) + "b")
	require.NotEqual(t, a, b)
	require.Empty(t, validation.IsDNS1123Subdomain(a))
	require.LessOrEqual(t, len(a), 253)
	require.Equal(t, a, GetEncryptionKeyName(strings.Repeat("a", 240)+"a"))
}

func TestWithManagedEncryptionKey(t *testing.T) {
	p := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "example"}}
	before := p.DeepCopy()
	effective, err := WithManagedEncryptionKey(p, "example-encryption-key")
	require.NoError(t, err)
	require.Equal(t, before, p)
	require.Equal(t, "/etc/perses/encryption/encryption-key", effective.Spec.Config.Security.EncryptionKeyFile)
	require.Empty(t, effective.Spec.Config.Security.EncryptionKey)
	var volume *corev1.Volume
	for _, v := range GetVolumes(effective) {
		if v.Name == "encryption-key" {
			volume = &v
		}
	}
	require.NotNil(t, volume)
	require.Equal(t, "example-encryption-key", volume.Secret.SecretName)
	require.EqualValues(t, 0440, *volume.Secret.DefaultMode)
	require.Contains(t, GetVolumeMounts(effective), corev1.VolumeMount{Name: "encryption-key", MountPath: "/etc/perses/encryption", ReadOnly: true})
	for _, mount := range []string{"/etc", "/etc/perses", "/etc/perses/encryption", "/etc/perses/encryption/child", "/etc/perses/../perses/encryption"} {
		t.Run(mount, func(t *testing.T) {
			p := before.DeepCopy()
			p.Spec.VolumeMounts = []corev1.VolumeMount{{Name: "custom", MountPath: mount}}
			_, err := WithManagedEncryptionKey(p, "key")
			require.Error(t, err)
		})
	}
	p.Spec.Volumes = []corev1.Volume{{Name: "encryption-key"}}
	_, err = WithManagedEncryptionKey(p, "key")
	require.Error(t, err)
}

func TestManagedEncryptionSecurityContext(t *testing.T) {
	for _, tc := range []struct {
		name    string
		context *corev1.PodSecurityContext
		group   int64
	}{
		{"default", nil, 65534},
		{"custom without group", &corev1.PodSecurityContext{RunAsUser: ptr.To[int64](10001), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}, 65534},
		{"explicit group", &corev1.PodSecurityContext{FSGroup: ptr.To[int64](2000)}, 2000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &v1alpha2.Perses{}
			p.Spec.PodSecurityContext = tc.context
			before := p.DeepCopy()
			effective, err := WithManagedEncryptionKey(p, "key")
			require.NoError(t, err)
			security := GetPodSecurityContext(effective)
			require.NotNil(t, security.FSGroup, "0440 key files require a volume group for non-root operands")
			require.Equal(t, tc.group, *security.FSGroup)
			require.Equal(t, before, p)
			if tc.context != nil {
				require.Equal(t, tc.context.RunAsUser, security.RunAsUser)
				require.Equal(t, tc.context.SeccompProfile, security.SeccompProfile)
			}
		})
	}
}

func TestCredentialSyncInterval(t *testing.T) {
	credentials := &v1alpha2.Client{BasicAuth: &v1alpha2.BasicAuth{}}
	require.Equal(t, 5*time.Minute, CredentialSyncInterval(0, credentials))
	require.Equal(t, 5*time.Minute, CredentialSyncInterval(-time.Second, credentials))
	require.Equal(t, time.Minute, CredentialSyncInterval(time.Minute, credentials))
	require.Zero(t, CredentialSyncInterval(0, nil))
}
