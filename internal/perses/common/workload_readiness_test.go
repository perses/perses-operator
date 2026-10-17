// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0

package common

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rhobs/perses-operator/api/v1alpha2"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPersesWorkloadReady(t *testing.T) {
	for _, stateful := range []bool{false, true} {
		kind := "deployment"
		if stateful {
			kind = "statefulset"
		}
		for _, tc := range []struct {
			name   string
			mutate func(client.Object, *corev1.Pod)
			extra  bool
			want   bool
		}{
			{"ready", func(o client.Object, p *corev1.Pod) {}, false, true},
			{"stale observation", func(o client.Object, p *corev1.Pod) { o.SetGeneration(3) }, false, false},
			{"wrong owner", func(o client.Object, p *corev1.Pod) { o.SetOwnerReferences(nil) }, false, false},
			{"pod unready", func(o client.Object, p *corev1.Pod) { p.Status.Conditions = nil }, false, false},
			{"terminating pod", func(o client.Object, p *corev1.Pod) {
				now := metav1.Now()
				p.DeletionTimestamp = &now
				p.Finalizers = []string{"test"}
			}, false, false},
			{"old serving pod", func(o client.Object, p *corev1.Pod) {}, true, false},
			{"zero replicas", func(o client.Object, p *corev1.Pod) {
				switch w := o.(type) {
				case *appsv1.Deployment:
					w.Spec.Replicas = ptr.To[int32](0)
				case *appsv1.StatefulSet:
					w.Spec.Replicas = ptr.To[int32](0)
				}
			}, false, false},
			{"old revision", func(o client.Object, p *corev1.Pod) {
				switch w := o.(type) {
				case *appsv1.Deployment:
					w.Status.UpdatedReplicas = 0
				case *appsv1.StatefulSet:
					w.Status.CurrentRevision = "old"
				}
			}, false, false},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				scheme := runtime.NewScheme()
				require.NoError(t, appsv1.AddToScheme(scheme))
				require.NoError(t, corev1.AddToScheme(scheme))
				p := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test", UID: "perses"}}
				require.NoError(t, json.Unmarshal([]byte(`{"database":{"file":{"folder":"/perses","extension":"yaml"}}}`), &p.Spec.Config))
				if !stateful {
					p.Spec.Storage = &v1alpha2.StorageConfiguration{EmptyDir: &corev1.EmptyDirVolumeSource{}}
				}
				owner := []metav1.OwnerReference{{APIVersion: v1alpha2.GroupVersion.String(), Kind: "Perses", Name: p.Name, UID: p.UID, Controller: ptr.To(true)}}
				metadata := metav1.ObjectMeta{Name: p.Name, Namespace: p.Namespace, UID: "workload", Generation: 2, OwnerReferences: owner}
				selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "example"}}
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "example-0", Namespace: "test", Labels: map[string]string{"app": "example", "pod-template-hash": "new", "controller-revision-hash": "new"}, OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", UID: "new-rs", Controller: ptr.To(true)}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
				var workload client.Object
				if stateful {
					pod.OwnerReferences = []metav1.OwnerReference{{Kind: "StatefulSet", UID: "workload", Controller: ptr.To(true)}}
					workload = &appsv1.StatefulSet{ObjectMeta: metadata, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To[int32](1), Selector: selector}, Status: appsv1.StatefulSetStatus{ObservedGeneration: 2, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, CurrentRevision: "new", UpdateRevision: "new"}}
				} else {
					workload = &appsv1.Deployment{ObjectMeta: metadata, Spec: appsv1.DeploymentSpec{Replicas: ptr.To[int32](1), Selector: selector}, Status: appsv1.DeploymentStatus{ObservedGeneration: 2, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}}
				}
				tc.mutate(workload, pod)
				objects := []client.Object{workload, pod}
				if tc.extra {
					old := pod.DeepCopy()
					old.Name = "old"
					old.Labels["pod-template-hash"] = "old"
					old.Labels["controller-revision-hash"] = "old"
					objects = append(objects, old)
				}
				reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
				ready, err := PersesWorkloadReady(context.Background(), reader, p)
				require.NoError(t, err)
				require.Equal(t, tc.want, ready)
			})
		}
	}
	t.Run("missing workload", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, appsv1.AddToScheme(scheme))
		p := &v1alpha2.Perses{}
		require.NoError(t, json.Unmarshal([]byte(`{"database":{"sql":{}}}`), &p.Spec.Config))
		ready, err := PersesWorkloadReady(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(), p)
		require.NoError(t, err)
		require.False(t, ready)
	})
	t.Run("no managed workload", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, appsv1.AddToScheme(scheme))
		// No database configured, so the operator manages neither a Deployment
		// nor a StatefulSet; there is no rollout to wait for.
		p := &v1alpha2.Perses{}
		ready, err := PersesWorkloadReady(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(), p)
		require.NoError(t, err)
		require.True(t, ready)
	})
	t.Run("converged zero replicas", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, appsv1.AddToScheme(scheme))
		require.NoError(t, corev1.AddToScheme(scheme))
		p := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test", UID: "perses"}}
		require.NoError(t, json.Unmarshal([]byte(`{"database":{"sql":{}}}`), &p.Spec.Config))
		owner := []metav1.OwnerReference{{APIVersion: v1alpha2.GroupVersion.String(), Kind: "Perses", Name: p.Name, UID: p.UID, Controller: ptr.To(true)}}
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: p.Name, Namespace: p.Namespace, UID: "workload", Generation: 2, OwnerReferences: owner},
			Spec:       appsv1.DeploymentSpec{Replicas: ptr.To[int32](0), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "example"}}},
			Status:     appsv1.DeploymentStatus{ObservedGeneration: 2},
		}
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep).Build()
		ready, err := PersesWorkloadReady(context.Background(), reader, p)
		require.NoError(t, err)
		require.True(t, ready)
	})
}
