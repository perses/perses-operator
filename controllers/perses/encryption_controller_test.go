// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0

package perses

import (
	"context"
	"fmt"
	"testing"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type encryptionTestClient struct {
	client.Client
	failPatch     bool
	createRace    *corev1.Secret
	conflictPatch bool
}

func (c *encryptionTestClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*corev1.Secret); ok && c.createRace != nil {
		winner := c.createRace
		c.createRace = nil
		if err := c.Client.Create(ctx, winner); err != nil {
			return err
		}
		return apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, obj.GetName())
	}
	if obj.GetUID() == "" {
		obj.SetUID(types.UID("uid-" + obj.GetName()))
	}
	return c.Client.Create(ctx, obj, opts...)
}
func (c *encryptionTestClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.Client.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	if s, ok := obj.(*corev1.Secret); ok {
		s.Data = nil
	}
	return nil
}
func (c *encryptionTestClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if c.conflictPatch {
		c.conflictPatch = false
		fresh := &v1alpha2.Perses{}
		if err := c.Client.Get(ctx, client.ObjectKeyFromObject(obj), fresh); err != nil {
			return err
		}
		fresh.Annotations = map[string]string{"unrelated": "preserve"}
		if err := c.Update(ctx, fresh); err != nil {
			return err
		}
		return apierrors.NewConflict(schema.GroupResource{Group: "perses.dev", Resource: "perses"}, obj.GetName(), fmt.Errorf("concurrent metadata update"))
	}
	if c.failPatch {
		c.failPatch = false
		return fmt.Errorf("injected patch failure")
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}
func encryptionReconciler(t *testing.T) (*PersesReconciler, *v1alpha2.Perses, *encryptionTestClient) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	p := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test", UID: "perses-uid"}}
	base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(p).WithObjects(p).Build()
	c := &encryptionTestClient{Client: base}
	return &PersesReconciler{Client: c, APIReader: base, Scheme: scheme}, p, c
}
func TestManagedEncryptionRuntime(t *testing.T) {
	r, p, _ := encryptionReconciler(t)
	r.Config.PersesImage = "docker.io/persesdev/perses:v0.55.0-rc.1"
	ctx := withEncryptionRuntime(withPerses(context.Background(), p), p)
	result, err := r.reconcileEncryptionKey(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(p)})
	require.NoError(t, err)
	require.Nil(t, result)
	effective, ok := runtimePersesFromContext(ctx)
	require.True(t, ok)
	cm, err := r.createPersesConfigMap(effective)
	require.NoError(t, err)
	require.Contains(t, cm.Data["config.yaml"], "/etc/perses/encryption/encryption-key")
	key := &corev1.Secret{}
	require.NoError(t, r.APIReader.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: "example-encryption-key"}, key))
	require.NotContains(t, cm.Data["config.yaml"], string(key.Data["encryption-key"]))
	dep, err := r.createPersesDeployment(effective)
	require.NoError(t, err)
	sts, err := r.createPersesStatefulSet(effective)
	require.NoError(t, err)
	for _, pod := range []corev1.PodSpec{dep.Spec.Template.Spec, sts.Spec.Template.Spec} {
		require.Contains(t, pod.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "encryption-key", MountPath: "/etc/perses/encryption", ReadOnly: true})
		require.Contains(t, pod.Volumes, corev1.Volume{Name: "encryption-key", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "example-encryption-key", DefaultMode: ptr.To[int32](0440), Items: []corev1.KeyToPath{{Key: "encryption-key", Path: "encryption-key"}}}}})
	}
	fresh := &v1alpha2.Perses{}
	require.NoError(t, r.APIReader.Get(ctx, client.ObjectKeyFromObject(p), fresh))
	require.Empty(t, fresh.Spec.Config.Security.EncryptionKeyFile)
	require.Empty(t, fresh.Spec.Volumes)
}

func TestManagedEncryptionRuntimePreservedOnError(t *testing.T) {
	r, p, _ := encryptionReconciler(t)
	// A user volume colliding with the managed encryption volume makes
	// WithManagedEncryptionKey fail after the key has been generated.
	p.Spec.Volumes = []corev1.Volume{{Name: common.EncryptionKeyVolumeName}}
	ctx := withEncryptionRuntime(withPerses(context.Background(), p), p)
	result, err := r.reconcileEncryptionKey(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(p)})
	require.Error(t, err)
	require.NotNil(t, result)
	// The error branch must leave the runtime Perses intact, not nil it out.
	runtime, ok := runtimePersesFromContext(ctx)
	require.True(t, ok)
	require.Same(t, p, runtime)
	state, ok := ctx.Value(encryptionRuntimeContextKey).(*encryptionRuntime)
	require.True(t, ok)
	require.False(t, state.Managed)
}

func TestGenerateEncryptionKey(t *testing.T) {
	a, err := generateEncryptionKey()
	require.NoError(t, err)
	b, err := generateEncryptionKey()
	require.NoError(t, err)
	require.Len(t, a, 32)
	require.NotEqual(t, a, b)
}
func TestEnsureEncryptionKey(t *testing.T) {
	ctx := context.Background()
	t.Run("reuses persisted key through uncached reader", func(t *testing.T) {
		r, p, _ := encryptionReconciler(t)
		first, err := r.ensureEncryptionKey(ctx, p)
		require.NoError(t, err)
		require.Len(t, first.Data[common.EncryptionKeyDataKey], 32)
		require.True(t, *first.Immutable)
		require.Equal(t, p.UID, metav1.GetControllerOf(first).UID)
		require.Equal(t, "encryption-key", first.Labels["perses.dev/purpose"])
		require.Equal(t, "true", first.Labels["perses.dev/watch"])
		fresh := &v1alpha2.Perses{}
		require.NoError(t, r.APIReader.Get(ctx, client.ObjectKeyFromObject(p), fresh))
		require.Equal(t, string(first.UID), fresh.Annotations[common.EncryptionKeyInitializedAnnotation])
		require.Empty(t, fresh.Spec.Config.Security.EncryptionKeyFile)
		restarted := &PersesReconciler{Client: r.Client, APIReader: r.APIReader, Scheme: r.Scheme}
		second, err := restarted.ensureEncryptionKey(ctx, p)
		require.NoError(t, err)
		require.Equal(t, first.Data, second.Data)
	})
	t.Run("create race validates winner and conflict patch preserves metadata", func(t *testing.T) {
		r, p, c := encryptionReconciler(t)
		winner := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.GetEncryptionKeyName(p.Name), Namespace: p.Namespace, UID: "winner", Labels: map[string]string{common.EncryptionKeyPurposeLabel: common.EncryptionKeyPurposeValue}}, Immutable: ptr.To(true), Data: map[string][]byte{common.EncryptionKeyDataKey: []byte("12345678901234567890123456789012")}}
		require.NoError(t, ctrl.SetControllerReference(p, winner, r.Scheme))
		c.createRace = winner
		c.conflictPatch = true
		key, err := r.ensureEncryptionKey(ctx, p)
		require.NoError(t, err)
		require.Equal(t, winner.Data, key.Data)
		require.Equal(t, winner.UID, key.UID)
		fresh := &v1alpha2.Perses{}
		require.NoError(t, r.APIReader.Get(ctx, client.ObjectKeyFromObject(p), fresh))
		require.Equal(t, "preserve", fresh.Annotations["unrelated"])
		require.Equal(t, "winner", fresh.Annotations[common.EncryptionKeyInitializedAnnotation])
	})
	t.Run("retains key after interrupted annotation patch", func(t *testing.T) {
		r, p, c := encryptionReconciler(t)
		c.failPatch = true
		_, err := r.ensureEncryptionKey(ctx, p)
		require.Error(t, err)
		existing := &corev1.Secret{}
		require.NoError(t, r.APIReader.Get(ctx, client.ObjectKey{Name: "example-encryption-key", Namespace: "test"}, existing))
		key, err := r.ensureEncryptionKey(ctx, p)
		require.NoError(t, err)
		require.Equal(t, existing.Data, key.Data)
	})
	t.Run("never regenerates missing initialized key", func(t *testing.T) {
		r, p, _ := encryptionReconciler(t)
		key, err := r.ensureEncryptionKey(ctx, p)
		require.NoError(t, err)
		require.NoError(t, r.Delete(ctx, key))
		_, err = r.ensureEncryptionKey(ctx, p)
		require.Error(t, err)
		require.Contains(t, err.Error(), "initialized")
		keys := &corev1.SecretList{}
		require.NoError(t, r.APIReader.List(ctx, keys))
		require.Empty(t, keys.Items)
	})
	t.Run("rejects replacement UID", func(t *testing.T) {
		r, p, c := encryptionReconciler(t)
		key, err := r.ensureEncryptionKey(ctx, p)
		require.NoError(t, err)
		require.NoError(t, r.Delete(ctx, key))
		key.ResourceVersion = ""
		key.UID = "replacement"
		require.NoError(t, c.Client.Create(ctx, key))
		_, err = r.ensureEncryptionKey(ctx, p)
		require.Error(t, err)
	})
	t.Run("rejects unrelated name collision", func(t *testing.T) {
		r, p, c := encryptionReconciler(t)
		require.NoError(t, c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "example-encryption-key", Namespace: "test"}}))
		_, err := r.ensureEncryptionKey(ctx, p)
		require.Error(t, err)
	})
	t.Run("preserves explicit settings and old generated key", func(t *testing.T) {
		r, p, c := encryptionReconciler(t)
		key, err := r.ensureEncryptionKey(ctx, p)
		require.NoError(t, err)
		fresh := &v1alpha2.Perses{}
		require.NoError(t, r.APIReader.Get(ctx, client.ObjectKeyFromObject(p), fresh))
		fresh.Spec.Config.Security.EncryptionKeyFile = "/external/key"
		require.NoError(t, c.Update(ctx, fresh))
		selected, err := r.ensureEncryptionKey(ctx, fresh)
		require.NoError(t, err)
		require.Nil(t, selected)
		persisted := &corev1.Secret{}
		require.NoError(t, r.APIReader.Get(ctx, client.ObjectKeyFromObject(key), persisted))
		require.Equal(t, key.Data, persisted.Data)
	})
}
