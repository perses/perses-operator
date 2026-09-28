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
	"reflect"
	"testing"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

func newTestReconciler(t *testing.T) *PersesReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha2 to scheme: %v", err)
	}
	return &PersesReconciler{Scheme: scheme}
}

func TestCreatePersesService_CustomPort(t *testing.T) {
	perses := &v1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.PersesSpec{
			ContainerPort: ptr.To[int32](9000),
		},
	}

	svc, err := newTestReconciler(t).createPersesService("test", perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.Spec.Ports[0].Port != 9000 {
		t.Errorf("service port = %d, want 9000", svc.Spec.Ports[0].Port)
	}
	if svc.Spec.Ports[0].TargetPort.IntVal != 9000 {
		t.Errorf("service targetPort = %d, want 9000", svc.Spec.Ports[0].TargetPort.IntVal)
	}
}

func TestCreatePersesService_SelectorUsesStableLabelsOnly(t *testing.T) {
	perses := &v1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.PersesSpec{
			Metadata: &v1alpha2.Metadata{
				Labels: map[string]string{"custom-label": "custom-value"},
			},
		},
	}

	svc, err := newTestReconciler(t).createPersesService("test", perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User labels can change over time while old pods still carry the previous
	// values, so they must not be part of the Service selector.
	if _, ok := svc.Spec.Selector["custom-label"]; ok {
		t.Errorf("service selector contains user label: %v", svc.Spec.Selector)
	}
	if !reflect.DeepEqual(svc.Spec.Selector, common.SelectorLabelsForPerses(perses.Name, perses)) {
		t.Errorf("service selector = %v, want %v", svc.Spec.Selector, common.SelectorLabelsForPerses(perses.Name, perses))
	}
	// The Service's own labels still carry the user labels.
	if svc.Labels["custom-label"] != "custom-value" {
		t.Errorf("service labels = %v, want custom-label=custom-value", svc.Labels)
	}
}

func TestServiceNeedsUpdate_NoChange(t *testing.T) {
	perses := &v1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.PersesSpec{
			Metadata: &v1alpha2.Metadata{
				Labels: map[string]string{"custom-label": "custom-value"},
			},
		},
	}

	r := newTestReconciler(t)
	existing, err := r.createPersesService("test", perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	desired, err := r.createPersesService("test", perses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The selector is narrower than the label set; that alone must not be
	// reported as a change, or the Service would be rewritten on every reconcile.
	if serviceNeedsUpdate(existing, desired, perses.Name, perses) {
		t.Error("serviceNeedsUpdate = true for identical services, want false")
	}

	existing.Spec.Selector["custom-label"] = "custom-value"
	if !serviceNeedsUpdate(existing, desired, perses.Name, perses) {
		t.Error("serviceNeedsUpdate = false for a stale selector, want true")
	}
}
