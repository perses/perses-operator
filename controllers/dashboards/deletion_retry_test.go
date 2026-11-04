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

package dashboards

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/perses/perses/pkg/model/api/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
)

// deletionFixture builds a file-database Perses instance whose pods are dialed
// at a real httptest backend, so the real client factory is exercised.
type deletionFixture struct {
	kube     client.Client
	reader   client.Reader
	podReady *corev1.Pod

	mu       sync.Mutex
	requests []string
}

func newDeletionFixture(t *testing.T, withPod bool) *deletionFixture {
	t.Helper()
	f := &deletionFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/test"):
			_, _ = w.Write([]byte(`{"kind":"Project","metadata":{"name":"test"}}`))
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/projects/test/dashboards/example"):
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected API request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	perses := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "test"}}
	perses.Spec.Config.Database = config.Database{File: &config.File{Folder: "/perses"}}
	perses.Spec.ContainerPort = ptr.To(int32(server.Listener.Addr().(*net.TCPAddr).Port))
	f.podReady = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-0", Namespace: "test", Labels: common.SelectorLabelsForPerses(perses.Name, perses)},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			PodIP:      "127.0.0.1",
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
		},
	}
	objects := []client.Object{perses}
	if withPod {
		objects = append(objects, f.podReady)
	}
	f.kube = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	f.reader = f.kube
	return f
}

func (f *deletionFixture) requestLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *deletionFixture) reconcile() (ctrl.Result, error) {
	r := &PersesDashboardReconciler{Client: f.kube, APIReader: f.reader, ClientFactory: common.NewWithConfig()}
	return r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "test", Name: "example"}})
}

func TestDeletionRetriesUntilPodsReady(t *testing.T) {
	for _, tc := range []struct {
		name    string
		withPod bool
	}{
		{"no pods (scaled to zero)", false},
		{"only a not-ready pod", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDeletionFixture(t, tc.withPod)

			result, err := f.reconcile()
			require.NoError(t, err, "no ready pods is a wait, not a failure")
			require.Equal(t, 30*time.Second, result.RequeueAfter, "deletion must be retried until a live process can be cleaned")
			require.Empty(t, f.requestLog(), "nothing can be contacted without a ready pod")

			ready := f.podReady.DeepCopy()
			if tc.withPod {
				require.NoError(t, f.kube.Get(context.Background(), client.ObjectKeyFromObject(ready), ready))
				ready.Status.Conditions[0].Status = corev1.ConditionTrue
				require.NoError(t, f.kube.Status().Update(context.Background(), ready))
			} else {
				ready.Status.Conditions[0].Status = corev1.ConditionTrue
				require.NoError(t, f.kube.Create(context.Background(), ready))
			}

			result, err = f.reconcile()
			require.NoError(t, err)
			require.Zero(t, result.RequeueAfter, "cleanup is finished once a ready pod was reached")
			require.Contains(t, f.requestLog(), "DELETE /api/v1/projects/test/dashboards/example")
		})
	}
}

type failingPodList struct{ client.Reader }

func (f failingPodList) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*corev1.PodList); ok {
		return errors.New("pod list unavailable")
	}
	return f.Reader.List(ctx, list, opts...)
}

func TestDeletionStillSurfacesOtherClientFactoryErrors(t *testing.T) {
	f := newDeletionFixture(t, true)
	f.reader = failingPodList{Reader: f.kube}

	_, err := f.reconcile()
	require.ErrorContains(t, err, "pod list unavailable")
	require.NotErrorIs(t, err, common.ErrNoReadyPods)
	require.Empty(t, f.requestLog())
}
