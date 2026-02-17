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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"github.com/perses/perses-operator/api/v1alpha2"
)

func TestBasicAuthConversion(t *testing.T) {
	v1alpha1JSON := `{"type":"secret","name":"test","username":"user","password_path":"/etc/pass"}`

	// Unmarshal v1alpha1 JSON (snake_case)
	var v1Auth BasicAuth
	if err := json.Unmarshal([]byte(v1alpha1JSON), &v1Auth); err != nil {
		t.Fatalf("unmarshal v1alpha1: %v", err)
	}
	if v1Auth.PasswordPath != "/etc/pass" {
		t.Errorf("expected PasswordPath='/etc/pass', got '%s'", v1Auth.PasswordPath)
	}

	// Convert to v1alpha2
	var v2Auth v1alpha2.BasicAuth
	if err := Convert_v1alpha1_BasicAuth_To_v1alpha2_BasicAuth(&v1Auth, &v2Auth, nil); err != nil {
		t.Fatalf("conversion v1->v2: %v", err)
	}
	if v2Auth.PasswordPath != v1Auth.PasswordPath {
		t.Errorf("conversion failed: expected '%s', got '%s'", v1Auth.PasswordPath, v2Auth.PasswordPath)
	}

	// Marshal v1alpha2 (should use camelCase)
	v2JSON, err := json.Marshal(v2Auth)
	if err != nil {
		t.Fatalf("marshal v1alpha2: %v", err)
	}
	if !strings.Contains(string(v2JSON), `"passwordPath"`) {
		t.Errorf("v1alpha2 should use 'passwordPath', got: %s", string(v2JSON))
	}
	if strings.Contains(string(v2JSON), `"password_path"`) {
		t.Errorf("v1alpha2 should not use 'password_path', got: %s", string(v2JSON))
	}

	// Reverse conversion
	var v1AuthReverse BasicAuth
	if err := Convert_v1alpha2_BasicAuth_To_v1alpha1_BasicAuth(&v2Auth, &v1AuthReverse, nil); err != nil {
		t.Fatalf("conversion v2->v1: %v", err)
	}

	// Marshal v1alpha1 (should use snake_case)
	v1JSONReverse, err := json.Marshal(v1AuthReverse)
	if err != nil {
		t.Fatalf("marshal v1alpha1: %v", err)
	}
	if !strings.Contains(string(v1JSONReverse), `"password_path"`) {
		t.Errorf("v1alpha1 should use 'password_path', got: %s", string(v1JSONReverse))
	}
	if strings.Contains(string(v1JSONReverse), `"passwordPath"`) {
		t.Errorf("v1alpha1 should not use 'passwordPath', got: %s", string(v1JSONReverse))
	}
}

func TestStorageConfigurationConversion(t *testing.T) {
	storageClass := "fast-ssd"
	size := resource.MustParse("10Gi")

	// Test v1alpha1 -> v1alpha2 conversion
	v1Storage := &StorageConfiguration{
		StorageClass: &storageClass,
		Size:         size,
	}

	var v2Storage v1alpha2.StorageConfiguration
	if err := Convert_v1alpha1_StorageConfiguration_To_v1alpha2_StorageConfiguration(v1Storage, &v2Storage, nil); err != nil {
		t.Fatalf("conversion v1->v2: %v", err)
	}

	// Verify PVC template was created with correct values
	if v2Storage.PersistentVolumeClaimTemplate == nil {
		t.Fatal("expected PersistentVolumeClaimTemplate to be set")
	}
	if *v2Storage.PersistentVolumeClaimTemplate.StorageClassName != storageClass {
		t.Errorf("expected storageClassName=%s, got %s", storageClass, *v2Storage.PersistentVolumeClaimTemplate.StorageClassName)
	}

	storageReq := v2Storage.PersistentVolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage]
	if !storageReq.Equal(size) {
		t.Errorf("expected size=%s, got %s", size.String(), storageReq.String())
	}

	// Test v1alpha2 -> v1alpha1 conversion (round trip)
	var v1StorageReverse StorageConfiguration
	if err := Convert_v1alpha2_StorageConfiguration_To_v1alpha1_StorageConfiguration(&v2Storage, &v1StorageReverse, nil); err != nil {
		t.Fatalf("conversion v2->v1: %v", err)
	}

	if v1StorageReverse.StorageClass == nil || *v1StorageReverse.StorageClass != storageClass {
		t.Errorf("expected storageClass=%s after round trip", storageClass)
	}
	if !v1StorageReverse.Size.Equal(size) {
		t.Errorf("expected size=%s after round trip, got %s", size.String(), v1StorageReverse.Size.String())
	}

	// Test v1alpha2 with EmptyDir (should drop EmptyDir when converting to v1alpha1)
	v2StorageWithEmptyDir := v1alpha2.StorageConfiguration{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			SizeLimit: ptr.To(resource.MustParse("1Gi")),
		},
	}

	var v1StorageDropped StorageConfiguration
	if err := Convert_v1alpha2_StorageConfiguration_To_v1alpha1_StorageConfiguration(&v2StorageWithEmptyDir, &v1StorageDropped, nil); err != nil {
		t.Fatalf("conversion v2->v1 with emptyDir: %v", err)
	}

	// v1alpha1 should have no storage config since EmptyDir isn't supported
	if v1StorageDropped.StorageClass != nil || !v1StorageDropped.Size.IsZero() {
		t.Error("expected empty storage config when converting emptyDir from v2 to v1")
	}
}
