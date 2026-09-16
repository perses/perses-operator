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

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
)

func TestMergeReason(t *testing.T) {
	tests := []struct {
		name     string
		current  ConditionStatusReason
		incoming ConditionStatusReason
		want     ConditionStatusReason
	}{
		{"empty current yields to incoming", "", ReasonBackendError, ReasonBackendError},
		{"validation outranks backend", ReasonBackendError, ReasonValidationFailed, ReasonValidationFailed},
		{"config outranks backend", ReasonBackendError, ReasonInvalidConfiguration, ReasonInvalidConfiguration},
		{"connection outranks backend", ReasonBackendError, ReasonConnectionFailed, ReasonConnectionFailed},
		{"backend does not clobber validation", ReasonValidationFailed, ReasonBackendError, ReasonValidationFailed},
		{"backend does not clobber config", ReasonInvalidConfiguration, ReasonBackendError, ReasonInvalidConfiguration},
		{"validation outranks connection", ReasonConnectionFailed, ReasonValidationFailed, ReasonValidationFailed},
		{"equal severity keeps current", ReasonValidationFailed, ReasonInvalidConfiguration, ReasonValidationFailed},
		{"same reason is stable", ReasonBackendError, ReasonBackendError, ReasonBackendError},
		{"empty incoming does not lower current", ReasonValidationFailed, "", ReasonValidationFailed},
		{"known incoming outranks unknown current", "SomethingElse", ReasonBackendError, ReasonBackendError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MergeReason(tt.current, tt.incoming))
		})
	}
}

func TestResourceTargetsInstance(t *testing.T) {
	instanceLabels := map[string]string{
		"app.kubernetes.io/instance": "perses-1",
		"env":                        "prod",
	}

	tests := []struct {
		name     string
		selector *metav1.LabelSelector
		want     bool
		wantErr  bool
	}{
		{"nil selector matches everything", nil, true, false},
		{"empty selector matches everything", &metav1.LabelSelector{}, true, false},
		{"matching label", &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}}, true, false},
		{"non-matching label", &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}}, false, false},
		{"missing label", &metav1.LabelSelector{MatchLabels: map[string]string{"team": "obs"}}, false, false},
		{
			"invalid selector operator",
			&metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "env",
				Operator: "Bogus",
			}}},
			false,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResourceTargetsInstance(tt.selector, instanceLabels)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPersesInstanceForPod(t *testing.T) {
	ctx := context.Background()
	scheme := newScheme()
	require.NoError(t, persesv1alpha2.AddToScheme(scheme))

	instance := &persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "perses-1", Namespace: "default"},
	}
	// The pod's instance label is the sanitized instance name (see LabelsForPerses).
	instanceLabel := LabelsForPerses(instance.Name, instance)["app.kubernetes.io/instance"]

	podFor := func(labels map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Labels: labels}}
	}

	t.Run("resolves the owning instance from the sanitized label", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
		pod := podFor(map[string]string{
			"app.kubernetes.io/managed-by": "perses-operator",
			"app.kubernetes.io/instance":   instanceLabel,
		})
		got, err := PersesInstanceForPod(ctx, reader, pod)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "perses-1", got.Name)
	})

	t.Run("ignores pods not managed by the operator", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
		pod := podFor(map[string]string{"app.kubernetes.io/instance": instanceLabel})
		got, err := PersesInstanceForPod(ctx, reader, pod)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("returns nil when no instance matches", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
		pod := podFor(map[string]string{
			"app.kubernetes.io/managed-by": "perses-operator",
			"app.kubernetes.io/instance":   "other-instance",
		})
		got, err := PersesInstanceForPod(ctx, reader, pod)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestPodReadinessPredicateCreate(t *testing.T) {
	pred := PodReadinessPredicate()

	readyPod := &corev1.Pod{Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
		Type: corev1.PodReady, Status: corev1.ConditionTrue,
	}}}}
	notReadyPod := &corev1.Pod{Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
		Type: corev1.PodReady, Status: corev1.ConditionFalse,
	}}}}

	// A pod that is already Ready on its initial-list Create event must resync,
	// otherwise pods already running when the operator (re)starts are skipped.
	assert.True(t, pred.Create(event.CreateEvent{Object: readyPod}))
	assert.False(t, pred.Create(event.CreateEvent{Object: notReadyPod}))
	assert.False(t, pred.Create(event.CreateEvent{Object: &corev1.Service{}}))
}
