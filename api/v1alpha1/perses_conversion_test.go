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
