// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0

package common

import (
	"context"
	"time"

	"github.com/perses/perses-operator/api/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const WorkloadRetryInterval = 5 * time.Second

// PersesWorkloadReady checks rollout convergence. Pass an uncached reader.
func PersesWorkloadReady(ctx context.Context, reader client.Reader, p *v1alpha2.Perses) (bool, error) {
	name := client.ObjectKeyFromObject(p)
	var selector *metav1.LabelSelector
	var desired int32
	var workload client.Object
	var revision string
	if p.RequiresDeployment() {
		dep := &appsv1.Deployment{}
		if err := reader.Get(ctx, name, dep); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		desired = ptr.Deref(dep.Spec.Replicas, 1)
		if !deploymentRolledOut(dep, desired) {
			return false, nil
		}
		selector = dep.Spec.Selector
		workload = dep
	} else if p.RequiresStatefulSet() {
		sts := &appsv1.StatefulSet{}
		if err := reader.Get(ctx, name, sts); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		desired = ptr.Deref(sts.Spec.Replicas, 1)
		if !statefulSetRolledOut(sts, desired) {
			return false, nil
		}
		selector = sts.Spec.Selector
		workload = sts
		revision = sts.Status.UpdateRevision
	} else {
		// No managed Deployment or StatefulSet (e.g. no database), so nothing to await.
		return true, nil
	}
	owner := metav1.GetControllerOf(workload)
	if selector == nil || owner == nil || owner.Kind != "Perses" || owner.UID != p.UID || !workload.GetDeletionTimestamp().IsZero() {
		return false, nil
	}
	labels, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false, err
	}
	pods := &corev1.PodList{}
	if err := reader.List(ctx, pods, client.InNamespace(p.Namespace), client.MatchingLabelsSelector{Selector: labels}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	var count int32
	var replicaSetUID types.UID
	var hash string
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		if !pod.DeletionTimestamp.IsZero() {
			return false, nil
		}
		count++
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				ready = true
			}
		}
		podOwner := metav1.GetControllerOf(&pod)
		if !ready || podOwner == nil || podOwner.UID == "" {
			return false, nil
		}
		if revision != "" {
			if podOwner.Kind != "StatefulSet" || podOwner.UID != workload.GetUID() || pod.Labels[appsv1.ControllerRevisionHashLabelKey] != revision {
				return false, nil
			}
		} else {
			currentHash := pod.Labels[appsv1.DefaultDeploymentUniqueLabelKey]
			if podOwner.Kind != "ReplicaSet" || currentHash == "" {
				return false, nil
			}
			if replicaSetUID != "" && (replicaSetUID != podOwner.UID || hash != currentHash) {
				return false, nil
			}
			replicaSetUID = podOwner.UID
			hash = currentHash
		}
	}
	return count == desired, nil
}

// deploymentRolledOut reports whether all desired replicas are updated, ready,
// and available at the latest observed generation.
func deploymentRolledOut(dep *appsv1.Deployment, desired int32) bool {
	return dep.Status.ObservedGeneration >= dep.Generation &&
		dep.Status.Replicas == desired &&
		dep.Status.UpdatedReplicas == desired &&
		dep.Status.ReadyReplicas == desired &&
		dep.Status.AvailableReplicas == desired
}

// statefulSetRolledOut reports whether all desired replicas are updated and
// ready at the current revision and latest observed generation.
func statefulSetRolledOut(sts *appsv1.StatefulSet, desired int32) bool {
	return sts.Status.ObservedGeneration >= sts.Generation &&
		sts.Status.Replicas == desired &&
		sts.Status.UpdatedReplicas == desired &&
		sts.Status.ReadyReplicas == desired &&
		sts.Status.UpdateRevision != "" &&
		sts.Status.CurrentRevision == sts.Status.UpdateRevision
}
