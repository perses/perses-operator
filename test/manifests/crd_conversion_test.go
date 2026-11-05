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

// Package manifests verifies that every CRD serving more than one version is
// wired to the conversion webhook in both the kustomize sources and the
// generated install artifacts. Without it, the API server falls back to the
// "None" strategy, which silently drops fields whose shape differs between
// versions instead of failing.
package manifests

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"gopkg.in/yaml.v2"
)

const repoRoot = "../.."

type crd struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Versions []struct {
			Name   string `yaml:"name"`
			Served bool   `yaml:"served"`
		} `yaml:"versions"`
		Conversion *struct {
			Strategy string `yaml:"strategy"`
			Webhook  *struct {
				ClientConfig struct {
					Service *struct {
						Name string `yaml:"name"`
						Path string `yaml:"path"`
					} `yaml:"service"`
				} `yaml:"clientConfig"`
				ConversionReviewVersions []string `yaml:"conversionReviewVersions"`
			} `yaml:"webhook"`
		} `yaml:"conversion"`
	} `yaml:"spec"`
}

type csv struct {
	Kind string `yaml:"kind"`
	Spec struct {
		WebhookDefinitions []struct {
			Type           string   `yaml:"type"`
			ConversionCRDs []string `yaml:"conversionCRDs"`
		} `yaml:"webhookdefinitions"`
	} `yaml:"spec"`
}

// decodeAll decodes every YAML document of the given kind in path.
func decodeAll[T any](t *testing.T, path, kind string) []T {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	var out []T
	dec := yaml.NewDecoder(f)
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return out
			}
			t.Fatalf("decode %s: %v", path, err)
		}
		if raw == nil || raw["kind"] != kind {
			continue
		}
		// Re-encode so documents of other kinds never have to match T.
		b, err := yaml.Marshal(raw)
		if err != nil {
			t.Fatalf("re-marshal %s: %v", path, err)
		}
		var doc T
		if err := yaml.Unmarshal(b, &doc); err != nil {
			t.Fatalf("decode %s in %s: %v", kind, path, err)
		}
		out = append(out, doc)
	}
}

func glob(t *testing.T, pattern string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot, pattern))
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no files match %s", pattern)
	}
	return files
}

// multiVersionCRDs returns the names of CRDs in config/crd/bases that serve
// more than one version and therefore need the conversion webhook.
func multiVersionCRDs(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, path := range glob(t, "config/crd/bases/*.yaml") {
		for _, c := range decodeAll[crd](t, path, "CustomResourceDefinition") {
			served := 0
			for _, v := range c.Spec.Versions {
				if v.Served {
					served++
				}
			}
			if served > 1 {
				names = append(names, c.Metadata.Name)
			}
		}
	}
	if len(names) == 0 {
		t.Fatal("expected at least one multi-version CRD in config/crd/bases")
	}
	sort.Strings(names)
	return names
}

func assertWebhookConversion(t *testing.T, source string, c crd) {
	t.Helper()
	conv := c.Spec.Conversion
	if conv == nil || conv.Strategy != "Webhook" {
		t.Errorf("%s: CRD %s serves multiple versions but spec.conversion.strategy is not Webhook", source, c.Metadata.Name)
		return
	}
	if conv.Webhook == nil || conv.Webhook.ClientConfig.Service == nil || conv.Webhook.ClientConfig.Service.Path != "/convert" {
		t.Errorf("%s: CRD %s must point spec.conversion.webhook.clientConfig.service.path at /convert", source, c.Metadata.Name)
		return
	}
	if !slices.Contains(conv.Webhook.ConversionReviewVersions, "v1") {
		t.Errorf("%s: CRD %s must list v1 in spec.conversion.webhook.conversionReviewVersions", source, c.Metadata.Name)
	}
}

// TestConversionWebhookPatches guards the kustomize patches themselves, so a
// patch that targets the wrong CRD fails with a pointer to the offending file.
func TestConversionWebhookPatches(t *testing.T) {
	want := multiVersionCRDs(t)

	patchesByCRD := map[string][]string{}
	for _, path := range glob(t, "config/crd/patches/webhook_in_*.yaml") {
		rel, _ := filepath.Rel(repoRoot, path)
		for _, c := range decodeAll[crd](t, path, "CustomResourceDefinition") {
			patchesByCRD[c.Metadata.Name] = append(patchesByCRD[c.Metadata.Name], rel)
			assertWebhookConversion(t, rel, c)
		}
	}

	for name, files := range patchesByCRD {
		if len(files) > 1 {
			t.Errorf("CRD %s is targeted by multiple conversion webhook patches: %v", name, files)
		}
	}
	for _, name := range want {
		if len(patchesByCRD[name]) == 0 {
			t.Errorf("CRD %s serves multiple versions but no config/crd/patches/webhook_in_*.yaml targets it", name)
		}
	}
}

// TestGeneratedCRDsUseConversionWebhook checks the committed install artifacts,
// which CI keeps in sync with config/ via installer-check and bundle-check.
func TestGeneratedCRDsUseConversionWebhook(t *testing.T) {
	want := multiVersionCRDs(t)

	sources := map[string][]string{
		"bundle.yaml":      {filepath.Join(repoRoot, "bundle.yaml")},
		"bundle/manifests": glob(t, "bundle/manifests/perses.dev_*.yaml"),
	}
	for source, files := range sources {
		t.Run(source, func(t *testing.T) {
			found := map[string]crd{}
			for _, path := range files {
				for _, c := range decodeAll[crd](t, path, "CustomResourceDefinition") {
					found[c.Metadata.Name] = c
				}
			}
			for _, name := range want {
				c, ok := found[name]
				if !ok {
					t.Errorf("%s: CRD %s not found", source, name)
					continue
				}
				assertWebhookConversion(t, source, c)
			}
		})
	}
}

func TestCSVConversionCRDs(t *testing.T) {
	want := multiVersionCRDs(t)

	path := filepath.Join(repoRoot, "bundle/manifests/perses-operator.clusterserviceversion.yaml")
	csvs := decodeAll[csv](t, path, "ClusterServiceVersion")
	if len(csvs) != 1 {
		t.Fatalf("expected exactly one ClusterServiceVersion in %s, got %d", path, len(csvs))
	}

	var conversionCRDs []string
	for _, def := range csvs[0].Spec.WebhookDefinitions {
		if def.Type == "ConversionWebhook" {
			conversionCRDs = append(conversionCRDs, def.ConversionCRDs...)
		}
	}
	for _, name := range want {
		if !slices.Contains(conversionCRDs, name) {
			t.Errorf("CSV ConversionWebhook definition does not list %s in conversionCRDs (got %v)", name, conversionCRDs)
		}
	}
}
