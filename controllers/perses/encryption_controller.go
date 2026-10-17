// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0

package perses

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type encryptionRuntime struct {
	Perses  *v1alpha2.Perses
	Managed bool
}

const encryptionRuntimeContextKey persesContextKey = "encryption-runtime"

func withEncryptionRuntime(ctx context.Context, p *v1alpha2.Perses) context.Context {
	return context.WithValue(ctx, encryptionRuntimeContextKey, &encryptionRuntime{Perses: p})
}

func runtimePersesFromContext(ctx context.Context) (*v1alpha2.Perses, bool) {
	if state, ok := ctx.Value(encryptionRuntimeContextKey).(*encryptionRuntime); ok {
		return state.Perses, true
	}
	return persesFromContext(ctx)
}

func (r *PersesReconciler) reconcileEncryptionKey(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	state, ok := ctx.Value(encryptionRuntimeContextKey).(*encryptionRuntime)
	if !ok {
		return subreconciler.RequeueWithError(fmt.Errorf("encryption runtime context missing"))
	}
	p := state.Perses
	key, err := r.ensureEncryptionKey(ctx, p)
	if err == nil && key != nil {
		// Only overwrite state.Perses on success, so a failure leaves it valid.
		managed, managedErr := common.WithManagedEncryptionKey(p, key.Name)
		if managedErr != nil {
			err = managedErr
		} else {
			state.Perses = managed
			state.Managed = true
		}
	}
	if err != nil {
		_, statusErr := r.updatePersesStatus(ctx, req, func(current *v1alpha2.Perses) {
			meta.SetStatusCondition(&current.Status.Conditions, metav1.Condition{Type: common.TypeAvailablePerses, Status: metav1.ConditionFalse, Reason: "EncryptionKeyUnavailable", Message: err.Error()})
			meta.SetStatusCondition(&current.Status.Conditions, metav1.Condition{Type: common.TypeDegradedPerses, Status: metav1.ConditionTrue, Reason: "EncryptionKeyUnavailable", Message: err.Error()})
		})
		if statusErr != nil {
			return subreconciler.RequeueWithError(statusErr)
		}
		return subreconciler.RequeueWithError(err)
	}
	return subreconciler.ContinueReconciling()
}

func generateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate encryption key: %w", err)
	}
	return key, nil
}

func (r *PersesReconciler) ensureEncryptionKey(ctx context.Context, p *v1alpha2.Perses) (*corev1.Secret, error) {
	// Bypass the cache to avoid regenerating an initialized key.
	fresh := &v1alpha2.Perses{}
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(p), fresh); err != nil {
		return nil, err
	}
	if fresh.UID != p.UID || !fresh.DeletionTimestamp.IsZero() {
		return nil, fmt.Errorf("perses instance changed or is being deleted during encryption key initialization")
	}
	explicit, err := common.HasExplicitEncryptionKey(ctx, r.APIReader, fresh)
	if err != nil {
		return nil, err
	}
	if explicit {
		return nil, nil
	}
	initialized := fresh.Annotations[common.EncryptionKeyInitializedAnnotation]
	key := &corev1.Secret{}
	name := client.ObjectKey{Namespace: p.Namespace, Name: common.GetEncryptionKeyName(p.Name)}
	err = r.APIReader.Get(ctx, name, key)
	if apierrors.IsNotFound(err) {
		if initialized != "" {
			return nil, fmt.Errorf("initialized encryption key Secret %s is missing; restore the matching key backup", name)
		}
		data, err := generateEncryptionKey()
		if err != nil {
			return nil, err
		}
		labels := common.LabelsForPerses(p.Name, p)
		labels[common.PersesWatchLabel] = common.PersesWatchLabelValue
		labels[common.EncryptionKeyPurposeLabel] = common.EncryptionKeyPurposeValue
		key = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name.Name, Namespace: name.Namespace, Labels: labels},
			Immutable:  ptr.To(true),
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{common.EncryptionKeyDataKey: data},
		}
		if err := ctrl.SetControllerReference(p, key, r.Scheme); err != nil {
			return nil, err
		}
		if err := r.Create(ctx, key); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return nil, fmt.Errorf("create encryption key Secret %s: %w", name, err)
			}
			if err := r.APIReader.Get(ctx, name, key); err != nil {
				return nil, err
			}
		}
	} else if err != nil {
		return nil, fmt.Errorf("read encryption key Secret %s: %w", name, err)
	}
	owner := metav1.GetControllerOf(key)
	// Typed objects carry empty TypeMeta, so match the known GroupVersion.
	if owner == nil || owner.UID != p.UID || owner.Kind != "Perses" || owner.APIVersion != v1alpha2.GroupVersion.String() {
		return nil, fmt.Errorf("encryption key Secret %s is not controlled by this Perses instance", name)
	}
	if key.Labels[common.EncryptionKeyPurposeLabel] != common.EncryptionKeyPurposeValue || !ptr.Deref(key.Immutable, false) || len(key.Data[common.EncryptionKeyDataKey]) != 32 || key.UID == "" || !key.DeletionTimestamp.IsZero() {
		return nil, fmt.Errorf("encryption key Secret %s must be an existing immutable managed Secret with a 32-byte key", name)
	}
	if initialized != "" && initialized != string(key.UID) {
		return nil, fmt.Errorf("initialized encryption key Secret %s has been replaced; restore the matching key backup and explicitly authorize its UID", name)
	}
	if initialized == string(key.UID) {
		// Already recorded; skip the annotation patch.
		return key, nil
	}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha2.Perses{}
		if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(p), current); err != nil {
			return err
		}
		if current.UID != p.UID || !current.DeletionTimestamp.IsZero() {
			return fmt.Errorf("perses instance changed during encryption key initialization")
		}
		previous := current.Annotations[common.EncryptionKeyInitializedAnnotation]
		if previous == string(key.UID) {
			return nil
		}
		if previous != "" {
			return fmt.Errorf("initialized encryption key uid changed during reconciliation")
		}
		before := current.DeepCopy()
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations[common.EncryptionKeyInitializedAnnotation] = string(key.UID)
		return r.Patch(ctx, current, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
	})
	if err != nil {
		return nil, fmt.Errorf("record encryption key initialization: %w", err)
	}
	return key, nil
}
