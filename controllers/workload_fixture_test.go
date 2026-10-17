// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0

package controllers

import (
	"context"
	"fmt"

	"github.com/perses/perses-operator/api/v1alpha2"
	persesconfig "github.com/perses/perses/pkg/model/api/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// envtest has no workload controllers, so readiness needs explicit status fixtures.
func readyDatasourceWorkload(ctx context.Context, p *v1alpha2.Perses) error {
	if p.Labels == nil {
		p.Labels = map[string]string{}
	}
	if p.Labels["test.perses.dev/datasource"] != p.Name || p.Spec.Config.Database.File == nil {
		p.Labels["test.perses.dev/datasource"] = p.Name
		p.Spec.Config.Database.File = &persesconfig.File{Folder: "/perses"}
		if err := k8sClient.Update(ctx, p); err != nil {
			return err
		}
	}
	sts := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(p), sts); err != nil {
		return err
	}
	replicas := ptr.Deref(sts.Spec.Replicas, 1)
	if replicas != 1 {
		return fmt.Errorf("fixture expects one replica, got %d", replicas)
	}
	sts.Status = appsv1.StatefulSetStatus{ObservedGeneration: sts.Generation, Replicas: 1, CurrentReplicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, CurrentRevision: "fixture-revision", UpdateRevision: "fixture-revision"}
	if err := k8sClient.Status().Update(ctx, sts); err != nil {
		return err
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: p.Name + "-0", Namespace: p.Namespace}}
	_, err := ctrlCreateOrUpdatePod(ctx, pod, sts)
	return err
}

func ctrlCreateOrUpdatePod(ctx context.Context, pod *corev1.Pod, sts *appsv1.StatefulSet) (bool, error) {
	existing := &corev1.Pod{}
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), existing)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	if err != nil {
		pod.Labels = map[string]string{}
		for key, value := range sts.Spec.Selector.MatchLabels {
			pod.Labels[key] = value
		}
		pod.Labels[appsv1.ControllerRevisionHashLabelKey] = sts.Status.UpdateRevision
		pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "StatefulSet", Name: sts.Name, UID: sts.UID, Controller: ptr.To(true)}}
		pod.Spec = *sts.Spec.Template.Spec.DeepCopy()
		if err := k8sClient.Create(ctx, pod); err != nil {
			return false, err
		}
	} else {
		pod = existing
	}
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
	return true, k8sClient.Status().Update(ctx, pod)
}
