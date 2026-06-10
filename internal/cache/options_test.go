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

package cache

import (
	"fmt"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	persesCommon "github.com/perses/spec/go/common"
	persesDashboard "github.com/perses/spec/go/dashboard"
	persesDatasource "github.com/perses/spec/go/datasource"
	persesv1alpha2 "github.com/rhobs/perses-operator/api/v1alpha2"
	"github.com/rhobs/perses-operator/internal/perses/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestParseSecretLabelSelector(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string // string representation of the parsed selector
		expectNil bool
		expectErr bool
	}{
		{
			name:      "empty string returns nil",
			input:     "",
			expectNil: true,
		},
		{
			name:     "single equality selector",
			input:    "perses.dev/watch=true",
			expected: "perses.dev/watch=true",
		},
		{
			name:     "multiple selectors",
			input:    "perses.dev/watch=true,team=platform",
			expected: "perses.dev/watch=true,team=platform",
		},
		{
			name:     "not-equal selector",
			input:    "env!=production",
			expected: "env!=production",
		},
		{
			name:     "set-based selector",
			input:    "tier in (frontend,backend)",
			expected: "tier in (backend,frontend)",
		},
		{
			name:     "existence selector",
			input:    "perses.dev/watch",
			expected: "perses.dev/watch",
		},
		{
			name:      "invalid selector",
			input:     "invalid label!@#",
			expectErr: true,
		},
		{
			name:     "notin selector",
			input:    "env notin (staging,dev)",
			expected: "env notin (dev,staging)",
		},
		{
			name:     "double-equals selector",
			input:    "key==value",
			expected: "key==value",
		},
		{
			name:      "invalid key with spaces",
			input:     "invalid key=value",
			expectErr: true,
		},
		{
			name:     "empty parens in set selector matches nothing",
			input:    "key in ()",
			expected: "key in ()",
		},
		{
			name:      "unclosed parens in set selector",
			input:     "key in (value",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSecretLabelSelector(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result.String() != tt.expected {
				t.Errorf("expected selector %q, got %q", tt.expected, result.String())
			}
		})
	}
}

func findByObjectEntry(byObject map[client.Object]ctrlcache.ByObject, target client.Object) *ctrlcache.ByObject {
	for obj, entry := range byObject {
		if fmt.Sprintf("%T", obj) == fmt.Sprintf("%T", target) {
			return &entry
		}
	}
	return nil
}

func TestBuildCacheByObject_DefaultLabels(t *testing.T) {
	byObject := buildCacheByObject(nil, false, false)

	managedBySelector := labels.SelectorFromSet(labels.Set{
		common.PersesManagedByLabel: common.PersesManagedByValue,
	})

	// Check operator-managed resource types have managed-by selector
	managedTypes := []client.Object{
		&appsv1.Deployment{},
		&appsv1.StatefulSet{},
		&corev1.ConfigMap{},
		&corev1.Service{},
	}

	for _, obj := range managedTypes {
		entry := findByObjectEntry(byObject, obj)
		if entry == nil {
			t.Errorf("expected ByObject entry for %T", obj)
			continue
		}
		if entry.Label.String() != managedBySelector.String() {
			t.Errorf("expected label selector %q for %T, got %q", managedBySelector, obj, entry.Label)
		}
	}

	// Check secret has default watch label
	secretEntry := findByObjectEntry(byObject, &corev1.Secret{})
	if secretEntry == nil {
		t.Fatal("expected ByObject entry for Secret")
	}

	expectedSecretSelector := labels.SelectorFromSet(labels.Set{
		common.PersesWatchLabel: common.PersesWatchLabelValue,
	})
	if secretEntry.Label.String() != expectedSecretSelector.String() {
		t.Errorf("expected secret label selector %q, got %q", expectedSecretSelector, secretEntry.Label)
	}

	if secretEntry.Transform == nil {
		t.Error("expected Transform to be set on Secret entry")
	}
}

func TestBuildCacheByObject_CustomSecretSelector(t *testing.T) {
	customSelector, err := labels.Parse("custom=label")
	if err != nil {
		t.Fatalf("failed to parse selector: %v", err)
	}
	byObject := buildCacheByObject(customSelector, false, false)

	secretEntry := findByObjectEntry(byObject, &corev1.Secret{})
	if secretEntry == nil {
		t.Fatal("expected ByObject entry for Secret")
	}

	if secretEntry.Label.String() != customSelector.String() {
		t.Errorf("expected secret label selector %q, got %q", customSelector, secretEntry.Label)
	}

	if secretEntry.Transform == nil {
		t.Error("expected Transform to be set on Secret entry")
	}
}

func TestBuildCacheByObject_WatchAllSecrets(t *testing.T) {
	byObject := buildCacheByObject(nil, true, false)

	secretEntry := findByObjectEntry(byObject, &corev1.Secret{})
	if secretEntry == nil {
		t.Fatal("expected ByObject entry for Secret")
	}

	if secretEntry.Label != nil {
		t.Errorf("expected nil label selector for watch-all-secrets, got %q", secretEntry.Label)
	}

	if secretEntry.Transform == nil {
		t.Error("expected Transform to be set on Secret entry even with watch-all-secrets")
	}
}

func TestBuildCacheByObject_WatchAllSecretsOverridesSelector(t *testing.T) {
	customSelector, err := labels.Parse("custom=label")
	if err != nil {
		t.Fatalf("failed to parse selector: %v", err)
	}
	byObject := buildCacheByObject(customSelector, true, false)

	secretEntry := findByObjectEntry(byObject, &corev1.Secret{})
	if secretEntry == nil {
		t.Fatal("expected ByObject entry for Secret")
	}

	if secretEntry.Label != nil {
		t.Errorf("expected nil label selector when watch-all-secrets is true, got %q", secretEntry.Label)
	}
}

func getSecretTransform(t *testing.T) func(obj any) (any, error) {
	t.Helper()
	byObject := buildCacheByObject(nil, false, false)
	secretEntry := findByObjectEntry(byObject, &corev1.Secret{})
	if secretEntry == nil {
		t.Fatal("expected ByObject entry for Secret")
	}
	if secretEntry.Transform == nil {
		t.Fatal("expected Transform to be set on Secret entry")
	}
	return secretEntry.Transform
}

func TestBuildCacheByObject_SecretTransformStripsData(t *testing.T) {
	transform := getSecretTransform(t)

	secret := &corev1.Secret{
		Data:       map[string][]byte{"key": []byte("value")},
		StringData: map[string]string{"key": "value"},
	}

	result, err := transform(secret)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	transformed := result.(*corev1.Secret)
	if transformed.Data != nil {
		t.Error("expected Data to be nil after Transform")
	}
	if transformed.StringData != nil {
		t.Error("expected StringData to be nil after Transform")
	}
}

func TestBuildCacheByObject_SecretTransformNonSecretObject(t *testing.T) {
	transform := getSecretTransform(t)

	// A non-Secret object should pass through unchanged
	configMap := &corev1.ConfigMap{
		Data: map[string]string{"key": "value"},
	}

	result, err := transform(configMap)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	cm := result.(*corev1.ConfigMap)
	if cm.Data == nil || cm.Data["key"] != "value" {
		t.Error("expected ConfigMap data to be unchanged after Transform")
	}
}

func TestBuildCacheByObject_SecretTransformNilObject(t *testing.T) {
	transform := getSecretTransform(t)

	result, err := transform(nil)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestBuildCacheByObject_SecretTransformStringObject(t *testing.T) {
	transform := getSecretTransform(t)

	// An unexpected type should pass through unchanged
	result, err := transform("not a secret")
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	str, ok := result.(string)
	if !ok || str != "not a secret" {
		t.Errorf("expected string to pass through unchanged, got %v", result)
	}
}

func TestBuildCacheByObject_SecretTransformEmptySecret(t *testing.T) {
	transform := getSecretTransform(t)

	// A secret with no data should not error
	secret := &corev1.Secret{}

	result, err := transform(secret)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	transformed := result.(*corev1.Secret)
	if transformed.Data != nil {
		t.Error("expected Data to remain nil")
	}
	if transformed.StringData != nil {
		t.Error("expected StringData to remain nil")
	}
}

func TestBuildCacheByObject_TLSClusterProfile(t *testing.T) {
	byObject := buildCacheByObject(nil, false, true)

	apiServerEntry := findByObjectEntry(byObject, &configv1.APIServer{})
	if apiServerEntry == nil {
		t.Fatal("expected ByObject entry for APIServer when tlsClusterProfile is true")
	}

	if apiServerEntry.Label.String() != labels.Everything().String() {
		t.Errorf("expected Everything selector for APIServer, got %q", apiServerEntry.Label)
	}
}

func TestBuildCacheByObject_NoTLSClusterProfile(t *testing.T) {
	byObject := buildCacheByObject(nil, false, false)

	apiServerEntry := findByObjectEntry(byObject, &configv1.APIServer{})
	if apiServerEntry != nil {
		t.Error("expected no ByObject entry for APIServer when tlsClusterProfile is false")
	}
}

func getCRDTransform(t *testing.T, target client.Object) func(obj any) (any, error) {
	t.Helper()
	byObject := buildCacheByObject(nil, false, false)
	entry := findByObjectEntry(byObject, target)
	if entry == nil {
		t.Fatalf("expected ByObject entry for %T", target)
	}
	if entry.Transform == nil {
		t.Fatalf("expected Transform on %T entry", target)
	}
	return entry.Transform
}

func TestBuildCacheOptions_SetsDefaultTransform(t *testing.T) {
	opts := BuildCacheOptions(nil, false, false)
	if opts.DefaultTransform == nil {
		t.Fatal("expected DefaultTransform to be set")
	}

	// Verify it strips ManagedFields from an arbitrary object
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
			},
		},
	}
	result, err := opts.DefaultTransform(cm)
	if err != nil {
		t.Fatalf("DefaultTransform returned error: %v", err)
	}
	if len(result.(*corev1.ConfigMap).ManagedFields) != 0 {
		t.Error("expected ManagedFields to be stripped by DefaultTransform")
	}
}

func TestBuildCacheOptions_ContainsByObject(t *testing.T) {
	opts := BuildCacheOptions(nil, false, false)
	if opts.ByObject == nil {
		t.Fatal("expected ByObject to be set")
	}
	entry := findByObjectEntry(opts.ByObject, &corev1.Secret{})
	if entry == nil {
		t.Fatal("expected Secret entry in ByObject")
	}
}

func TestBuildCacheByObject_DashboardTransformStripsManagedFields(t *testing.T) {
	transform := getCRDTransform(t, &persesv1alpha2.PersesDashboard{})

	dashboard := &persesv1alpha2.PersesDashboard{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dash",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
			},
		},
	}

	result, err := transform(dashboard)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	if result.(*persesv1alpha2.PersesDashboard).ManagedFields != nil {
		t.Error("expected ManagedFields to be nil after Transform")
	}
}

func TestBuildCacheByObject_SecretTransformStripsManagedFields(t *testing.T) {
	transform := getSecretTransform(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
			},
		},
	}

	result, err := transform(secret)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	if result.(*corev1.Secret).ManagedFields != nil {
		t.Error("expected ManagedFields to be nil after Transform")
	}
}

func TestBuildCacheByObject_DashboardTransformStripsSpec(t *testing.T) {
	transform := getCRDTransform(t, &persesv1alpha2.PersesDashboard{})

	dashboard := &persesv1alpha2.PersesDashboard{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-dash",
			Namespace:       "test-ns",
			ResourceVersion: "12345",
			Labels:          map[string]string{"app": "test"},
			Annotations:     map[string]string{"perses.dev/tags": "oncall"},
		},
		Spec: persesv1alpha2.PersesDashboardSpec{
			Config: persesv1alpha2.Dashboard{
				Spec: persesDashboard.Spec{
					Display:  &persesCommon.Display{Name: "My Dashboard"},
					Duration: "5m",
				},
			},
			InstanceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "prod"},
			},
		},
		Status: persesv1alpha2.PersesDashboardStatus{
			Conditions: []metav1.Condition{{
				Type:   common.TypeAvailablePerses,
				Status: metav1.ConditionTrue,
				Reason: "Reconciled",
			}},
		},
	}

	result, err := transform(dashboard)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	d := result.(*persesv1alpha2.PersesDashboard)

	if d.Spec.Config.Display != nil || d.Spec.Config.Duration != "" {
		t.Error("expected Spec.Config to be zeroed")
	}
	if d.Spec.InstanceSelector != nil {
		t.Error("expected Spec.InstanceSelector to be nil")
	}

	if d.Name != "test-dash" || d.Namespace != "test-ns" || d.ResourceVersion != "12345" {
		t.Error("expected ObjectMeta to be preserved")
	}
	if d.Labels["app"] != "test" || d.Annotations["perses.dev/tags"] != "oncall" {
		t.Error("expected Labels and Annotations to be preserved")
	}
	if len(d.Status.Conditions) != 1 || d.Status.Conditions[0].Type != common.TypeAvailablePerses {
		t.Error("expected Status.Conditions to be preserved")
	}
}

func TestBuildCacheByObject_DashboardTransformEmptyObject(t *testing.T) {
	transform := getCRDTransform(t, &persesv1alpha2.PersesDashboard{})

	dashboard := &persesv1alpha2.PersesDashboard{}
	result, err := transform(dashboard)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	if result.(*persesv1alpha2.PersesDashboard) == nil {
		t.Error("expected non-nil result")
	}
}

func TestBuildCacheByObject_DatasourceTransformStripsSpec(t *testing.T) {
	transform := getCRDTransform(t, &persesv1alpha2.PersesDatasource{})

	datasource := &persesv1alpha2.PersesDatasource{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-ds",
			Namespace:       "test-ns",
			ResourceVersion: "67890",
		},
		Spec: persesv1alpha2.DatasourceSpec{
			Config: persesv1alpha2.Datasource{
				Spec: persesDatasource.Spec{
					Default: true,
					Plugin:  persesCommon.Plugin{Kind: "PrometheusDatasource"},
				},
			},
			Client: &persesv1alpha2.Client{
				BasicAuth: &persesv1alpha2.BasicAuth{Username: "admin"},
			},
			InstanceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "prod"},
			},
		},
		Status: persesv1alpha2.PersesDatasourceStatus{
			Conditions: []metav1.Condition{{
				Type:   common.TypeAvailablePerses,
				Status: metav1.ConditionTrue,
			}},
		},
	}

	result, err := transform(datasource)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	d := result.(*persesv1alpha2.PersesDatasource)

	if d.Spec.Config.Default || d.Spec.Config.Plugin.Kind != "" {
		t.Error("expected Spec.Config to be zeroed")
	}
	if d.Spec.Client != nil {
		t.Error("expected Spec.Client to be nil")
	}
	if d.Spec.InstanceSelector != nil {
		t.Error("expected Spec.InstanceSelector to be nil")
	}

	if d.Name != "test-ds" || d.ResourceVersion != "67890" {
		t.Error("expected ObjectMeta to be preserved")
	}
	if len(d.Status.Conditions) != 1 {
		t.Error("expected Status.Conditions to be preserved")
	}
}

func TestBuildCacheByObject_GlobalDatasourceTransformStripsSpec(t *testing.T) {
	transform := getCRDTransform(t, &persesv1alpha2.PersesGlobalDatasource{})

	gds := &persesv1alpha2.PersesGlobalDatasource{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-gds",
			ResourceVersion: "11111",
		},
		Spec: persesv1alpha2.DatasourceSpec{
			Config: persesv1alpha2.Datasource{
				Spec: persesDatasource.Spec{
					Default: true,
					Plugin:  persesCommon.Plugin{Kind: "PrometheusDatasource"},
				},
			},
			Client: &persesv1alpha2.Client{
				BasicAuth: &persesv1alpha2.BasicAuth{Username: "admin"},
			},
			InstanceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "prod"},
			},
		},
		Status: persesv1alpha2.PersesGlobalDatasourceStatus{
			Conditions: []metav1.Condition{{
				Type:   common.TypeAvailablePerses,
				Status: metav1.ConditionTrue,
			}},
		},
	}

	result, err := transform(gds)
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	d := result.(*persesv1alpha2.PersesGlobalDatasource)

	if d.Spec.Config.Default || d.Spec.Config.Plugin.Kind != "" {
		t.Error("expected Spec.Config to be zeroed")
	}
	if d.Spec.Client != nil {
		t.Error("expected Spec.Client to be nil")
	}
	if d.Spec.InstanceSelector != nil {
		t.Error("expected Spec.InstanceSelector to be nil")
	}

	if d.Name != "test-gds" || d.ResourceVersion != "11111" {
		t.Error("expected ObjectMeta to be preserved")
	}
	if len(d.Status.Conditions) != 1 {
		t.Error("expected Status.Conditions to be preserved")
	}
}
