// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0

package datasources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/perses/perses-operator/api/v1alpha2"
	"github.com/perses/perses-operator/internal/perses/common"
	v1 "github.com/perses/perses/pkg/client/api/v1"
	clientconfig "github.com/perses/perses/pkg/client/config"
	model "github.com/perses/perses/pkg/model/api/v1"
	speccommon "github.com/perses/spec/go/common"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagedSecretReplay(t *testing.T) {
	for _, sourceType := range []v1alpha2.SecretSourceType{v1alpha2.SecretSourceTypeSecret, v1alpha2.SecretSourceTypeConfigMap} {
		t.Run(string(sourceType), func(t *testing.T) { testManagedSecretReplay(t, sourceType) })
	}
}

func testManagedSecretReplay(t *testing.T, sourceType v1alpha2.SecretSourceType) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	source := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "test"}, Data: map[string][]byte{"password": []byte("original-password")}}
	ds := &v1alpha2.PersesDatasource{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test"}}
	require.NoError(t, json.Unmarshal([]byte(`{"plugin":{"kind":"Prometheus","spec":{"directUrl":"http://example.com"}}}`), &ds.Spec.Config.Spec))
	ds.Spec.Client = &v1alpha2.Client{BasicAuth: &v1alpha2.BasicAuth{SecretSource: v1alpha2.SecretSource{Type: v1alpha2.SecretSourceTypeSecret, Name: ptr.To("credentials"), Namespace: ptr.To("test")}, Username: "user", PasswordPath: "password"}}
	stored := &model.Datasource{Kind: model.KindDatasource, Metadata: model.ProjectMetadata{Metadata: model.Metadata{Name: ds.Name}}, Spec: ds.Spec.Config.Spec}
	var passwords []string
	var tlsKeys []string
	datasourceWrites := 0
	missing := false
	reject := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/projects/test"):
			_ = json.NewEncoder(w).Encode(&model.Project{Kind: "Project", Metadata: model.Metadata{Name: "test"}})
		case strings.HasSuffix(r.URL.Path, "/datasources/example"):
			if r.Method != "GET" {
				datasourceWrites++
			}
			_ = json.NewEncoder(w).Encode(stored)
		case strings.HasSuffix(r.URL.Path, "/secrets/example-secret") || strings.HasSuffix(r.URL.Path, "/secrets"):
			if r.Method == "GET" {
				if missing {
					w.WriteHeader(404)
					return
				}
				_, _ = w.Write([]byte(`{"kind":"Secret","metadata":{"name":"example-secret"},"spec":{}}`))
				return
			}
			if r.Method != "POST" && r.Method != "PUT" {
				t.Errorf("unexpected secret method %s", r.Method)
				w.WriteHeader(500)
				return
			}
			if reject {
				w.WriteHeader(401)
				return
			}
			var payload struct {
				Spec struct {
					BasicAuth struct {
						Password string `json:"password"`
					} `json:"basicAuth"`
					TLSConfig struct {
						Cert string `json:"cert"`
						Key  string `json:"key"`
					} `json:"tlsConfig"`
				} `json:"spec"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			passwords = append(passwords, payload.Spec.BasicAuth.Password)
			tlsKeys = append(tlsKeys, payload.Spec.TLSConfig.Key)
			missing = false
			_, _ = w.Write([]byte(`{"kind":"Secret","metadata":{"name":"example-secret"},"spec":{}}`))
		default:
			t.Errorf("unexpected API request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	url, err := speccommon.ParseURL(server.URL)
	require.NoError(t, err)
	rest, err := clientconfig.NewRESTClient(clientconfig.RestConfigClient{URL: url})
	require.NoError(t, err)
	var credentialSource client.Object = source
	if sourceType == v1alpha2.SecretSourceTypeConfigMap {
		credentialSource = &corev1.ConfigMap{ObjectMeta: source.ObjectMeta, Data: map[string]string{"password": "original-password"}}
	}
	ds.Spec.Client.BasicAuth.Type = sourceType
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(credentialSource).Build()
	r := &PersesDatasourceReconciler{Client: kube, APIReader: kube, ClientFactory: common.NewWithClient(v1.NewWithClient(rest))}
	sync := func() error { _, _, err := r.syncPersesDatasource(ctx, v1alpha2.Perses{}, ds); return err }
	require.NoError(t, sync())
	require.Equal(t, []string{"original-password"}, passwords)
	require.Zero(t, datasourceWrites)
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(credentialSource), credentialSource))
	switch obj := credentialSource.(type) {
	case *corev1.Secret:
		obj.Data["password"] = []byte("rotated-password")
	case *corev1.ConfigMap:
		obj.Data["password"] = "rotated-password"
	}
	require.NoError(t, kube.Update(ctx, credentialSource))
	require.NoError(t, sync())
	require.Equal(t, []string{"original-password", "rotated-password"}, passwords)
	require.Zero(t, datasourceWrites)
	missing = true
	require.NoError(t, sync())
	require.Len(t, passwords, 3)
	require.Equal(t, "rotated-password", passwords[2])
	reject = true
	require.Error(t, sync())
	require.Len(t, passwords, 3)
	reject = false
	require.NoError(t, sync())
	require.Len(t, passwords, 4)
	// Client changes must not require datasource updates.
	ds.Spec.Client.TLS = &v1alpha2.TLS{Enable: ptr.To(true), UserCert: &v1alpha2.Certificate{SecretSource: v1alpha2.SecretSource{Type: sourceType, Name: ptr.To("credentials"), Namespace: ptr.To("test")}, CertPath: "cert", PrivateKeyPath: ptr.To("key")}}
	switch obj := credentialSource.(type) {
	case *corev1.Secret:
		obj.Data["cert"] = []byte("source-cert")
		obj.Data["key"] = []byte("source-key")
	case *corev1.ConfigMap:
		obj.Data["cert"] = "source-cert"
		obj.Data["key"] = "source-key"
	}
	require.NoError(t, kube.Update(ctx, credentialSource))
	require.NoError(t, sync())
	require.Equal(t, "source-key", tlsKeys[len(tlsKeys)-1])
	require.Zero(t, datasourceWrites)
	for _, empty := range []bool{false, true} {
		switch obj := credentialSource.(type) {
		case *corev1.Secret:
			delete(obj.Data, "key")
			if empty {
				obj.Data["key"] = nil
			}
		case *corev1.ConfigMap:
			delete(obj.Data, "key")
			if empty {
				obj.Data["key"] = ""
			}
		}
		require.NoError(t, kube.Update(ctx, credentialSource))
		require.Error(t, sync(), "missing requested mTLS key must not overwrite a working API secret")
		require.Len(t, passwords, 5)
	}
	require.NoError(t, kube.Delete(ctx, credentialSource))
	require.Error(t, sync())
	require.Len(t, passwords, 5)
}

func TestManagedSecretReplayInterval(t *testing.T) {
	for _, tc := range []struct {
		name        string
		credentials bool
		interval    time.Duration
		want        time.Duration
	}{
		{"credentials retain recovery at zero", true, 0, 5 * time.Minute},
		{"no credentials retain zero semantics", false, 0, 0},
		{"custom positive interval", true, 17 * time.Second, 17 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha2.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			p := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "test", UID: "backend", Labels: map[string]string{"target": "yes"}}}
			require.NoError(t, json.Unmarshal([]byte(`{"database":{"file":{"folder":"/perses"}}}`), &p.Spec.Config))
			p.Status.Conditions = []metav1.Condition{{Type: common.TypeAvailablePerses, Status: metav1.ConditionTrue}}
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: p.Name, Namespace: p.Namespace, UID: "workload", Generation: 2, OwnerReferences: []metav1.OwnerReference{{Kind: "Perses", UID: p.UID, Controller: ptr.To(true)}}}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To[int32](1), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend"}}}, Status: appsv1.StatefulSetStatus{ObservedGeneration: 2, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, CurrentRevision: "new", UpdateRevision: "new"}}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "backend-0", Namespace: p.Namespace, Labels: map[string]string{"app": "backend", "controller-revision-hash": "new"}, OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", UID: sts.UID, Controller: ptr.To(true)}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
			ds := &v1alpha2.PersesDatasource{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test"}}
			require.NoError(t, json.Unmarshal([]byte(`{"plugin":{"kind":"Prometheus","spec":{"directUrl":"http://example.com"}}}`), &ds.Spec.Config.Spec))
			ds.Spec.InstanceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"target": "yes"}}
			if tc.credentials {
				ds.Spec.Client = &v1alpha2.Client{BasicAuth: &v1alpha2.BasicAuth{SecretSource: v1alpha2.SecretSource{Type: v1alpha2.SecretSourceTypeSecret, Name: ptr.To("credentials")}, Username: "user", PasswordPath: "password"}}
			}
			source := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "test"}, Data: map[string][]byte{"password": []byte("source-password")}}
			secretWrites := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/projects/test"):
					_, _ = w.Write([]byte(`{"kind":"Project","metadata":{"name":"test"}}`))
				case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/datasources/example"):
					_ = json.NewEncoder(w).Encode(&model.Datasource{Kind: model.KindDatasource, Metadata: model.ProjectMetadata{Metadata: model.Metadata{Name: ds.Name}}, Spec: ds.Spec.Config.Spec})
				case strings.HasSuffix(req.URL.Path, "/secrets/example-secret"):
					if req.Method == "PUT" {
						secretWrites++
					} else {
						require.Equal(t, "GET", req.Method)
					}
					_, _ = w.Write([]byte(`{"kind":"Secret","metadata":{"name":"example-secret"},"spec":{}}`))
				default:
					t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			url, err := speccommon.ParseURL(server.URL)
			require.NoError(t, err)
			rest, err := clientconfig.NewRESTClient(clientconfig.RestConfigClient{URL: url})
			require.NoError(t, err)
			unselected := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "unselected", Namespace: "test"}}
			kube := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ds).WithObjects(p, sts, pod, ds, source, unselected).Build()
			r := &PersesDatasourceReconciler{Client: kube, APIReader: kube, ClientFactory: common.NewWithClient(v1.NewWithClient(rest)), SyncInterval: tc.interval}
			result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(ds)})
			require.NoError(t, err)
			require.Equal(t, tc.want, result.RequeueAfter)
			if tc.credentials {
				require.Equal(t, 1, secretWrites)
			} else {
				require.Zero(t, secretWrites)
			}
			fresh := &v1alpha2.PersesDatasource{}
			require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(ds), fresh))
			available := false
			for _, condition := range fresh.Status.Conditions {
				if condition.Type == common.TypeAvailablePerses && condition.Status == metav1.ConditionTrue {
					available = true
				}
			}
			require.True(t, available)
		})
	}
}

func TestManagedSecretReplayReadinessAndRetry(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	p := &v1alpha2.Perses{ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "test", UID: "backend"}}
	ds := &v1alpha2.PersesDatasource{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "test"}}
	require.NoError(t, json.Unmarshal([]byte(`{"plugin":{"kind":"Prometheus","spec":{"directUrl":"http://example.com"}}}`), &ds.Spec.Config.Spec))
	ds.Spec.Client = &v1alpha2.Client{BasicAuth: &v1alpha2.BasicAuth{}}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ds).WithObjects(p, ds).Build()
	r := &PersesDatasourceReconciler{Client: kube, APIReader: kube, SyncInterval: 0}
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(ds)})
	require.NoError(t, err)
	require.Equal(t, 5*time.Second, result.RequeueAfter)
	fresh := &v1alpha2.PersesDatasource{}
	require.NoError(t, kube.Get(context.Background(), client.ObjectKeyFromObject(ds), fresh))
	for _, condition := range fresh.Status.Conditions {
		require.False(t, condition.Status == metav1.ConditionTrue && condition.Reason == "Reconciled")
	}
}
