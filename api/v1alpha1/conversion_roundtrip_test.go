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

package v1alpha1

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conv "sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/perses/perses-operator/api/v1alpha2"
)

// controller-runtime only serves a kind on /convert if the spoke is Convertible
// and the storage version is a Hub.
var (
	_ conv.Convertible = &Perses{}
	_ conv.Convertible = &PersesDashboard{}
	_ conv.Convertible = &PersesDatasource{}

	_ conv.Hub = &v1alpha2.Perses{}
	_ conv.Hub = &v1alpha2.PersesDashboard{}
	_ conv.Hub = &v1alpha2.PersesDatasource{}
)

// clientJSON uses the v1alpha1 wire format, including the snake_case
// "password_path" field that was renamed in v1alpha2.
const clientJSON = `{
  "basicAuth": {"type": "secret", "name": "basic-auth", "namespace": "monitoring", "username": "admin", "password_path": "password"},
  "oauth": {
    "type": "secret", "name": "oauth", "clientIDPath": "client-id", "clientSecretPath": "client-secret",
    "tokenURL": "https://auth.example.com/token", "scopes": ["read", "write"],
    "endpointParams": {"audience": ["perses"]}, "authStyle": 2
  },
  "tls": {
    "enable": true,
    "caCert": {"type": "configmap", "name": "ca", "certPath": "ca.crt"},
    "userCert": {"type": "secret", "name": "client-cert", "certPath": "tls.crt", "privateKeyPath": "tls.key"},
    "insecureSkipVerify": true
  },
  "kubernetesAuth": {"enable": true}
}`

const statusJSON = `{
  "conditions": [
    {"type": "Available", "status": "True", "reason": "Reconciled", "message": "ok", "lastTransitionTime": "2026-01-01T00:00:00Z", "observedGeneration": 3}
  ]
}`

const objectMetaJSON = `{"name": "test", "namespace": "monitoring", "labels": {"app": "perses"}, "annotations": {"note": "kept"}, "generation": 3}`

func mustUnmarshal(t *testing.T, data string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(data), into); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
}

func assertSemanticEqual(t *testing.T, want, got any) {
	t.Helper()
	if equality.Semantic.DeepEqual(want, got) {
		return
	}
	w, _ := json.MarshalIndent(want, "", "  ")
	g, _ := json.MarshalIndent(got, "", "  ")
	t.Errorf("objects differ after round trip\nwant:\n%s\ngot:\n%s", w, g)
}

// assertHubClient checks fields whose shape or JSON name differs between versions.
func assertHubClient(t *testing.T, client *v1alpha2.Client) {
	t.Helper()
	if client == nil {
		t.Fatal("expected spec.client to be converted, got nil")
	}
	if client.BasicAuth == nil || client.BasicAuth.PasswordPath != "password" {
		t.Errorf("expected basicAuth.passwordPath=password, got %+v", client.BasicAuth)
	}
	if client.OAuth == nil || client.OAuth.AuthStyle == nil || *client.OAuth.AuthStyle != 2 {
		t.Errorf("expected oauth.authStyle=2, got %+v", client.OAuth)
	}
	if client.TLS == nil || client.TLS.Enable == nil || !*client.TLS.Enable {
		t.Errorf("expected tls.enable=true, got %+v", client.TLS)
	}
	if client.KubernetesAuth == nil || client.KubernetesAuth.Enable == nil || !*client.KubernetesAuth.Enable {
		t.Errorf("expected kubernetesAuth.enable=true, got %+v", client.KubernetesAuth)
	}
}

func TestPersesConversionRoundTrip(t *testing.T) {
	original := &Perses{}
	mustUnmarshal(t, `{
	  "apiVersion": "perses.dev/v1alpha1",
	  "kind": "Perses",
	  "metadata": `+objectMetaJSON+`,
	  "spec": {
	    "metadata": {"labels": {"instance": "perses"}, "annotations": {"team": "observability"}},
	    "client": `+clientJSON+`,
	    "config": {"database": {"file": {"folder": "/perses", "extension": "yaml"}}},
	    "args": ["--log.level=debug"],
	    "containerPort": 8080,
	    "replicas": 2,
	    "nodeSelector": {"kubernetes.io/os": "linux"},
	    "tolerations": [{"key": "dedicated", "operator": "Equal", "value": "monitoring", "effect": "NoSchedule"}],
	    "image": "docker.io/persesdev/perses:v0.55.1",
	    "service": {"name": "perses-svc", "annotations": {"svc": "annotation"}},
	    "livenessProbe": {"httpGet": {"path": "/api/health", "port": 8080}, "periodSeconds": 10},
	    "readinessProbe": {"httpGet": {"path": "/api/health", "port": 8080}, "periodSeconds": 5},
	    "tls": {"enable": true, "userCert": {"type": "secret", "name": "server-cert", "certPath": "tls.crt", "privateKeyPath": "tls.key"}},
	    "storage": {"storageClass": "fast-ssd", "size": "10Gi"},
	    "serviceAccountName": "perses-sa"
	  },
	  "status": `+statusJSON+`
	}`, original)

	hub := &v1alpha2.Perses{}
	if err := original.DeepCopy().ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}

	assertHubClient(t, hub.Spec.Client)
	if hub.Spec.ContainerPort == nil || *hub.Spec.ContainerPort != 8080 {
		t.Errorf("expected containerPort=8080, got %v", hub.Spec.ContainerPort)
	}
	if hub.Spec.Storage == nil || hub.Spec.Storage.PersistentVolumeClaimTemplate == nil {
		t.Fatalf("expected storage to be migrated to persistentVolumeClaimTemplate, got %+v", hub.Spec.Storage)
	}
	pvc := hub.Spec.Storage.PersistentVolumeClaimTemplate
	if pvc.StorageClassName == nil || *pvc.StorageClassName != "fast-ssd" {
		t.Errorf("expected storageClassName=fast-ssd, got %v", pvc.StorageClassName)
	}
	if size := pvc.Resources.Requests[corev1.ResourceStorage]; !size.Equal(resource.MustParse("10Gi")) {
		t.Errorf("expected storage request=10Gi, got %s", size.String())
	}

	roundTripped := &Perses{}
	if err := roundTripped.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}

	// TypeMeta is set by the conversion webhook, not by the conversion functions.
	want := original.DeepCopy()
	want.TypeMeta = metav1.TypeMeta{}
	assertSemanticEqual(t, want, roundTripped)
}

func TestPersesDatasourceConversionRoundTrip(t *testing.T) {
	original := &PersesDatasource{}
	mustUnmarshal(t, `{
	  "apiVersion": "perses.dev/v1alpha1",
	  "kind": "PersesDatasource",
	  "metadata": `+objectMetaJSON+`,
	  "spec": {
	    "config": {
	      "display": {"name": "Prometheus", "description": "Main datasource"},
	      "default": true,
	      "plugin": {"kind": "PrometheusDatasource", "spec": {"directUrl": "https://prometheus.example.com"}}
	    },
	    "client": `+clientJSON+`
	  },
	  "status": `+statusJSON+`
	}`, original)

	hub := &v1alpha2.PersesDatasource{}
	if err := original.DeepCopy().ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}

	assertHubClient(t, hub.Spec.Client)
	if hub.Spec.Config.Display == nil || hub.Spec.Config.Display.Name != "Prometheus" {
		t.Errorf("expected config.display.name=Prometheus, got %+v", hub.Spec.Config.Display)
	}
	if hub.Spec.Config.Plugin.Kind != "PrometheusDatasource" {
		t.Errorf("expected config.plugin.kind=PrometheusDatasource, got %q", hub.Spec.Config.Plugin.Kind)
	}

	roundTripped := &PersesDatasource{}
	if err := roundTripped.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}

	want := original.DeepCopy()
	want.TypeMeta = metav1.TypeMeta{}
	assertSemanticEqual(t, want, roundTripped)
}

func TestPersesDatasourceConversionFromHub(t *testing.T) {
	hub := &v1alpha2.PersesDatasource{}
	mustUnmarshal(t, `{
	  "apiVersion": "perses.dev/v1alpha2",
	  "kind": "PersesDatasource",
	  "metadata": `+objectMetaJSON+`,
	  "spec": {
	    "config": {"display": {"name": "Prometheus"}, "default": false, "plugin": {"kind": "PrometheusDatasource", "spec": {"directUrl": "https://prometheus.example.com"}}},
	    "client": {"basicAuth": {"type": "secret", "name": "basic-auth", "namespace": "monitoring", "username": "admin", "passwordPath": "password"}},
	    "instanceSelector": {"matchLabels": {"app.kubernetes.io/instance": "perses"}}
	  }
	}`, hub)

	spoke := &PersesDatasource{}
	if err := spoke.ConvertFrom(hub.DeepCopy()); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if spoke.Spec.Client == nil || spoke.Spec.Client.BasicAuth == nil || spoke.Spec.Client.BasicAuth.PasswordPath != "password" {
		t.Fatalf("expected spec.client.basicAuth.password_path=password, got %+v", spoke.Spec.Client)
	}

	roundTripped := &v1alpha2.PersesDatasource{}
	if err := spoke.ConvertTo(roundTripped); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}

	// instanceSelector has no v1alpha1 equivalent and is expected to be dropped.
	want := hub.DeepCopy()
	want.TypeMeta = metav1.TypeMeta{}
	want.Spec.InstanceSelector = nil
	assertSemanticEqual(t, want, roundTripped)
}

func TestPersesDashboardConversionRoundTrip(t *testing.T) {
	original := &PersesDashboard{}
	mustUnmarshal(t, `{
	  "apiVersion": "perses.dev/v1alpha1",
	  "kind": "PersesDashboard",
	  "metadata": `+objectMetaJSON+`,
	  "spec": {
	    "display": {"name": "Overview"},
	    "duration": "1h",
	    "refreshInterval": "30s",
	    "panels": {
	      "cpu": {"kind": "Panel", "spec": {"display": {"name": "CPU"}, "plugin": {"kind": "TimeSeriesChart", "spec": {}}}}
	    },
	    "layouts": [
	      {"kind": "Grid", "spec": {"items": [{"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/cpu"}}]}}
	    ]
	  },
	  "status": `+statusJSON+`
	}`, original)

	hub := &v1alpha2.PersesDashboard{}
	if err := original.DeepCopy().ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}

	// v1alpha1 inlines the dashboard spec; v1alpha2 nests it under spec.config.
	if hub.Spec.Config.Display == nil || hub.Spec.Config.Display.Name != "Overview" {
		t.Errorf("expected config.display.name=Overview, got %+v", hub.Spec.Config.Display)
	}
	if hub.Spec.Config.Duration != "1h" {
		t.Errorf("expected config.duration=1h, got %q", hub.Spec.Config.Duration)
	}
	if _, ok := hub.Spec.Config.Panels["cpu"]; !ok {
		t.Errorf("expected config.panels to contain cpu, got %v", hub.Spec.Config.Panels)
	}

	roundTripped := &PersesDashboard{}
	if err := roundTripped.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}

	want := original.DeepCopy()
	want.TypeMeta = metav1.TypeMeta{}
	assertSemanticEqual(t, want, roundTripped)
}
