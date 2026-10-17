// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0

package common

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/operator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	EncryptionKeyDataKey               = "encryption-key"
	EncryptionKeyInitializedAnnotation = "perses.dev/encryption-key-initialized"
	EncryptionKeyVolumeName            = "encryption-key"
	EncryptionKeyMountPath             = "/etc/perses/encryption"
	EncryptionKeyFilePath              = EncryptionKeyMountPath + "/encryption-key"
	EncryptionKeyPurposeLabel          = "perses.dev/purpose"
	EncryptionKeyPurposeValue          = "encryption-key"
	// DefaultEncryptionKeyFSGroup lets non-root containers read the 0440 key file.
	DefaultEncryptionKeyFSGroup = 65534
)

// HasExplicitEncryptionKey reports configured key sources, including empty environment values.
func HasExplicitEncryptionKey(ctx context.Context, reader client.Reader, p *v1alpha2.Perses) (bool, error) {
	// A key set directly in the spec is conclusive; skip the env-source reads.
	if len(p.Spec.Config.Security.EncryptionKey) > 0 || p.Spec.Config.Security.EncryptionKeyFile != "" {
		return true, nil
	}
	explicit := false
	for _, source := range p.Spec.EnvFrom {
		var keys []string
		switch {
		case source.SecretRef != nil:
			obj := &corev1.Secret{}
			if err := reader.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: source.SecretRef.Name}, obj); err != nil {
				if apierrors.IsNotFound(err) && ptr.Deref(source.SecretRef.Optional, false) {
					continue
				}
				return false, fmt.Errorf("read encryption environment Secret %s: %w", source.SecretRef.Name, err)
			}
			for key := range obj.Data {
				keys = append(keys, key)
			}
		case source.ConfigMapRef != nil:
			obj := &corev1.ConfigMap{}
			if err := reader.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: source.ConfigMapRef.Name}, obj); err != nil {
				if apierrors.IsNotFound(err) && ptr.Deref(source.ConfigMapRef.Optional, false) {
					continue
				}
				return false, fmt.Errorf("read encryption environment ConfigMap %s: %w", source.ConfigMapRef.Name, err)
			}
			for key := range obj.Data {
				keys = append(keys, key)
			}
		}
		for _, key := range keys {
			if isEncryptionEnvironment(source.Prefix + key) {
				explicit = true
			}
		}
	}
	for _, env := range p.Spec.Env {
		if !isEncryptionEnvironment(env.Name) {
			continue
		}
		if env.ValueFrom == nil {
			explicit = true
			continue
		}
		var exists bool
		var err error
		switch {
		case env.ValueFrom.SecretKeyRef != nil:
			ref := env.ValueFrom.SecretKeyRef
			obj := &corev1.Secret{}
			err = reader.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: ref.Name}, obj)
			if apierrors.IsNotFound(err) && ptr.Deref(ref.Optional, false) {
				continue
			}
			if err == nil {
				_, exists = obj.Data[ref.Key]
				if !exists && !ptr.Deref(ref.Optional, false) {
					err = fmt.Errorf("required encryption environment key %s missing in Secret %s", ref.Key, ref.Name)
				}
			}
		case env.ValueFrom.ConfigMapKeyRef != nil:
			ref := env.ValueFrom.ConfigMapKeyRef
			obj := &corev1.ConfigMap{}
			err = reader.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: ref.Name}, obj)
			if apierrors.IsNotFound(err) && ptr.Deref(ref.Optional, false) {
				continue
			}
			if err == nil {
				_, exists = obj.Data[ref.Key]
				if !exists && !ptr.Deref(ref.Optional, false) {
					err = fmt.Errorf("required encryption environment key %s missing in ConfigMap %s", ref.Key, ref.Name)
				}
			}
		default:
			// Kubelet resolves downward-API values.
			exists = true
		}
		if err != nil {
			return false, fmt.Errorf("resolve %s: %w", env.Name, err)
		}
		explicit = explicit || exists
	}
	return explicit, nil
}

func isEncryptionEnvironment(name string) bool {
	return name == "PERSES_SECURITY_ENCRYPTION_KEY" || name == "PERSES_SECURITY_ENCRYPTION_KEY_FILE"
}

func GetEncryptionKeyName(persesName string) string {
	const suffix = "-encryption-key"
	if len(persesName)+len(suffix) <= 253 {
		return persesName + suffix
	}
	digest := sha256.Sum256([]byte(persesName))
	suffixWithHash := fmt.Sprintf("-%x%s", digest[:6], suffix)
	return strings.TrimRight(persesName[:253-len(suffixWithHash)], "-.") + suffixWithHash
}

// WithManagedEncryptionKey returns a runtime copy that must not be persisted.
func WithManagedEncryptionKey(p *v1alpha2.Perses, secretName string) (*v1alpha2.Perses, error) {
	if len(p.Spec.Config.Security.EncryptionKey) > 0 || p.Spec.Config.Security.EncryptionKeyFile != "" {
		return nil, fmt.Errorf("cannot replace explicitly configured encryption key")
	}
	for _, volume := range GetVolumes(p) {
		if volume.Name == EncryptionKeyVolumeName {
			return nil, fmt.Errorf("volume %q conflicts with managed encryption key", volume.Name)
		}
	}
	for _, mount := range GetVolumeMounts(p) {
		cleaned := path.Clean(mount.MountPath)
		if cleaned == "/" || cleaned == EncryptionKeyMountPath || strings.HasPrefix(cleaned, EncryptionKeyMountPath+"/") || strings.HasPrefix(EncryptionKeyMountPath, cleaned+"/") {
			return nil, fmt.Errorf("volume mount %q overlaps managed encryption key path", mount.MountPath)
		}
	}
	effective := p.DeepCopy()
	// Supply a volume group so non-root containers can read the 0440 key file.
	if effective.Spec.PodSecurityContext != nil && effective.Spec.PodSecurityContext.FSGroup == nil {
		effective.Spec.PodSecurityContext.FSGroup = ptr.To[int64](DefaultEncryptionKeyFSGroup)
	}
	effective.Spec.Config.Security.EncryptionKeyFile = EncryptionKeyFilePath
	effective.Spec.Volumes = append(effective.Spec.Volumes, corev1.Volume{
		Name: EncryptionKeyVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretName,
				DefaultMode: ptr.To[int32](0440),
				Items:       []corev1.KeyToPath{{Key: EncryptionKeyDataKey, Path: EncryptionKeyDataKey}},
			},
		},
	})
	effective.Spec.VolumeMounts = append(effective.Spec.VolumeMounts, corev1.VolumeMount{
		Name:      EncryptionKeyVolumeName,
		MountPath: EncryptionKeyMountPath,
		ReadOnly:  true,
	})
	return effective, nil
}

func CredentialSyncInterval(configured time.Duration, c *v1alpha2.Client) time.Duration {
	if configured <= 0 && HasSecretConfig(c) {
		return operator.DefaultResourceSyncInterval
	}
	return configured
}
