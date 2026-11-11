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

package manifests

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// The operator pods serve the conversion webhook, so with a single replica
// conversion fails during pod restarts. HA is opt-in: default install artifacts
// must stay single-replica without a PodDisruptionBudget, while the opt-in HA
// manifests must be highly available with selectors matching the operator pods.

const hostnameTopologyKey = "kubernetes.io/hostname"

type labelSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

type deploymentSpec struct {
	Replicas *int          `yaml:"replicas"`
	Selector labelSelector `yaml:"selector"`
	Template struct {
		Metadata struct {
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
		Spec struct {
			Affinity *struct {
				PodAntiAffinity *struct {
					Preferred []struct {
						PodAffinityTerm struct {
							LabelSelector labelSelector `yaml:"labelSelector"`
							TopologyKey   string        `yaml:"topologyKey"`
						} `yaml:"podAffinityTerm"`
					} `yaml:"preferredDuringSchedulingIgnoredDuringExecution"`
				} `yaml:"podAntiAffinity"`
			} `yaml:"affinity"`
		} `yaml:"spec"`
	} `yaml:"template"`
}

type deployment struct {
	Metadata struct {
		Name   string            `yaml:"name"`
		Labels map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Spec deploymentSpec `yaml:"spec"`
}

type kustomizeComponent struct {
	Kind      string   `yaml:"kind"`
	Resources []string `yaml:"resources"`
	Patches   []struct {
		Path   string `yaml:"path"`
		Target *struct {
			Kind          string `yaml:"kind"`
			LabelSelector string `yaml:"labelSelector"`
		} `yaml:"target"`
	} `yaml:"patches"`
}

type podDisruptionBudget struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		MinAvailable   any           `yaml:"minAvailable"`
		MaxUnavailable any           `yaml:"maxUnavailable"`
		Selector       labelSelector `yaml:"selector"`
	} `yaml:"spec"`
}

type service struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Selector map[string]string `yaml:"selector"`
	} `yaml:"spec"`
}

type csvInstall struct {
	Spec struct {
		Install struct {
			Spec struct {
				Deployments []struct {
					Name string         `yaml:"name"`
					Spec deploymentSpec `yaml:"spec"`
				} `yaml:"deployments"`
			} `yaml:"spec"`
		} `yaml:"install"`
		WebhookDefinitions []struct {
			Type           string `yaml:"type"`
			DeploymentName string `yaml:"deploymentName"`
		} `yaml:"webhookdefinitions"`
	} `yaml:"spec"`
}

// selects reports whether a non-empty matchLabels selector matches labels.
func selects(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// assertHighlyAvailable checks that the deployment named source/name runs
// multiple replicas, is covered by a PodDisruptionBudget that still allows
// voluntary disruptions, and prefers spreading its pods across nodes.
func assertHighlyAvailable(t *testing.T, source, name string, spec deploymentSpec, pdbs []podDisruptionBudget) {
	t.Helper()
	podLabels := spec.Template.Metadata.Labels

	replicas := 1
	if spec.Replicas != nil {
		replicas = *spec.Replicas
	}
	if replicas < 2 {
		t.Errorf("%s: deployment %s runs %d replica(s); at least 2 are needed to keep the conversion webhook available during pod restarts", source, name, replicas)
	}
	if !selects(spec.Selector.MatchLabels, podLabels) {
		t.Errorf("%s: deployment %s selector %v does not match its pod labels %v", source, name, spec.Selector.MatchLabels, podLabels)
	}

	covered := false
	for _, pdb := range pdbs {
		if !selects(pdb.Spec.Selector.MatchLabels, podLabels) {
			continue
		}
		covered = true
		if err := checkDisruptionBudget(pdb, replicas); err != nil {
			t.Errorf("%s: PodDisruptionBudget %s for deployment %s: %v", source, pdb.Metadata.Name, name, err)
		}
	}
	if !covered {
		t.Errorf("%s: no PodDisruptionBudget selects the pods of deployment %s (labels %v)", source, name, podLabels)
	}

	// Anti-affinity must stay preferred: a required rule would leave the second
	// replica Pending on single-node clusters such as the kind e2e cluster.
	spread := false
	if a := spec.Template.Spec.Affinity; a != nil && a.PodAntiAffinity != nil {
		for _, term := range a.PodAntiAffinity.Preferred {
			if term.PodAffinityTerm.TopologyKey == hostnameTopologyKey && selects(term.PodAffinityTerm.LabelSelector.MatchLabels, podLabels) {
				spread = true
			}
		}
	}
	if !spread {
		t.Errorf("%s: deployment %s has no preferred pod anti-affinity on %s selecting its own pods (labels %v)", source, name, hostnameTopologyKey, podLabels)
	}
}

// assertSingleReplica checks that the deployment runs one replica and that no
// PodDisruptionBudget selects its pods.
func assertSingleReplica(t *testing.T, source, name string, spec deploymentSpec, pdbs []podDisruptionBudget) {
	t.Helper()
	podLabels := spec.Template.Metadata.Labels

	if spec.Replicas != nil && *spec.Replicas != 1 {
		t.Errorf("%s: deployment %s runs %d replicas; high availability must be opt-in, the default is 1 replica", source, name, *spec.Replicas)
	}
	for _, pdb := range pdbs {
		if selects(pdb.Spec.Selector.MatchLabels, podLabels) {
			t.Errorf("%s: PodDisruptionBudget %s selects the pods of deployment %s; it belongs to the opt-in HA manifests only", source, pdb.Metadata.Name, name)
		}
	}
}

// checkDisruptionBudget requires an integer budget that keeps at least one pod
// running and still allows at least one voluntary disruption, so node drains
// are never blocked indefinitely.
func checkDisruptionBudget(pdb podDisruptionBudget, replicas int) error {
	switch {
	case pdb.Spec.MinAvailable != nil:
		v, ok := pdb.Spec.MinAvailable.(int)
		if !ok {
			return fmt.Errorf("minAvailable must be an integer, got %v", pdb.Spec.MinAvailable)
		}
		if v < 1 || v >= replicas {
			return fmt.Errorf("minAvailable %d must be at least 1 and lower than replicas (%d)", v, replicas)
		}
	case pdb.Spec.MaxUnavailable != nil:
		v, ok := pdb.Spec.MaxUnavailable.(int)
		if !ok {
			return fmt.Errorf("maxUnavailable must be an integer, got %v", pdb.Spec.MaxUnavailable)
		}
		if v < 1 || v >= replicas {
			return fmt.Errorf("maxUnavailable %d must be at least 1 and lower than replicas (%d)", v, replicas)
		}
	default:
		return fmt.Errorf("neither minAvailable nor maxUnavailable is set")
	}
	return nil
}

// TestInstallerWebhookDefaults checks that the deployment behind the
// conversion webhook service in bundle.yaml runs a single replica.
func TestInstallerWebhookDefaults(t *testing.T) {
	path := filepath.Join(repoRoot, "bundle.yaml")
	want := multiVersionCRDs(t)

	serviceNames := map[string]bool{}
	for _, c := range decodeAll[crd](t, path, "CustomResourceDefinition") {
		for _, name := range want {
			if c.Metadata.Name == name && c.Spec.Conversion != nil && c.Spec.Conversion.Webhook != nil && c.Spec.Conversion.Webhook.ClientConfig.Service != nil {
				serviceNames[c.Spec.Conversion.Webhook.ClientConfig.Service.Name] = true
			}
		}
	}
	if len(serviceNames) == 0 {
		t.Fatalf("bundle.yaml: no conversion webhook service referenced by the multi-version CRDs %v", want)
	}

	services := decodeAll[service](t, path, "Service")
	deployments := decodeAll[deployment](t, path, "Deployment")
	pdbs := decodeAll[podDisruptionBudget](t, path, "PodDisruptionBudget")

	for svcName := range serviceNames {
		var svc *service
		for i := range services {
			if services[i].Metadata.Name == svcName {
				svc = &services[i]
			}
		}
		if svc == nil {
			t.Errorf("bundle.yaml: conversion webhook service %s not found", svcName)
			continue
		}

		backed := false
		for _, d := range deployments {
			if !selects(svc.Spec.Selector, d.Spec.Template.Metadata.Labels) {
				continue
			}
			backed = true
			assertSingleReplica(t, "bundle.yaml", d.Metadata.Name, d.Spec, pdbs)
		}
		if !backed {
			t.Errorf("bundle.yaml: no deployment backs conversion webhook service %s (selector %v)", svcName, svc.Spec.Selector)
		}
	}
}

// TestBundleWebhookDefaults checks that the CSV deployment serving the
// conversion webhook runs a single replica.
func TestBundleWebhookDefaults(t *testing.T) {
	csvPath := filepath.Join(repoRoot, "bundle/manifests/perses-operator.clusterserviceversion.yaml")
	csvs := decodeAll[csvInstall](t, csvPath, "ClusterServiceVersion")
	if len(csvs) != 1 {
		t.Fatalf("expected exactly one ClusterServiceVersion in %s, got %d", csvPath, len(csvs))
	}

	var pdbs []podDisruptionBudget
	for _, path := range glob(t, "bundle/manifests/*.yaml") {
		pdbs = append(pdbs, decodeAll[podDisruptionBudget](t, path, "PodDisruptionBudget")...)
	}

	checked := 0
	for _, def := range csvs[0].Spec.WebhookDefinitions {
		if def.Type != "ConversionWebhook" {
			continue
		}
		checked++
		found := false
		for _, d := range csvs[0].Spec.Install.Spec.Deployments {
			if d.Name == def.DeploymentName {
				found = true
				assertSingleReplica(t, "bundle/manifests", d.Name, d.Spec, pdbs)
			}
		}
		if !found {
			t.Errorf("bundle/manifests: ConversionWebhook deploymentName %q is not a CSV install deployment", def.DeploymentName)
		}
	}
	if checked == 0 {
		t.Fatal("bundle/manifests: CSV has no ConversionWebhook definition")
	}
}

// TestHAComponent checks config/components/ha applied on top of
// config/manager/manager.yaml, which provides the pod labels and selector.
func TestHAComponent(t *testing.T) {
	const dir = "config/components/ha"
	components := decodeAll[kustomizeComponent](t, filepath.Join(repoRoot, dir, "kustomization.yaml"), "Component")
	if len(components) != 1 {
		t.Fatalf("%s: expected kustomization.yaml of kind Component, got %d", dir, len(components))
	}
	component := components[0]

	managers := decodeAll[deployment](t, filepath.Join(repoRoot, "config/manager/manager.yaml"), "Deployment")
	if len(managers) != 1 {
		t.Fatalf("config/manager/manager.yaml: expected exactly one Deployment, got %d", len(managers))
	}
	manager := managers[0]

	var pdbs []podDisruptionBudget
	for _, res := range component.Resources {
		pdbs = append(pdbs, decodeAll[podDisruptionBudget](t, filepath.Join(repoRoot, dir, res), "PodDisruptionBudget")...)
	}

	patched := 0
	for _, p := range component.Patches {
		patches := decodeAll[deployment](t, filepath.Join(repoRoot, dir, p.Path), "Deployment")
		if len(patches) == 0 {
			continue
		}
		patched++

		// Names are already prefixed by config/default, so target by label.
		if p.Target == nil || p.Target.Kind != "Deployment" || p.Target.LabelSelector == "" {
			t.Errorf("%s: patch %s must target kind Deployment by labelSelector", dir, p.Path)
		} else if !selects(parseLabelSelector(t, p.Target.LabelSelector), manager.Metadata.Labels) {
			t.Errorf("%s: patch %s target %q does not match the manager Deployment labels %v", dir, p.Path, p.Target.LabelSelector, manager.Metadata.Labels)
		}

		spec := patches[0].Spec
		spec.Selector = manager.Spec.Selector
		spec.Template.Metadata.Labels = manager.Spec.Template.Metadata.Labels
		assertHighlyAvailable(t, dir, manager.Metadata.Name, spec, pdbs)
	}
	if patched != 1 {
		t.Fatalf("%s: expected exactly one Deployment patch, got %d", dir, patched)
	}
}

// parseLabelSelector parses an equality-based selector such as "a=b,c=d".
func parseLabelSelector(t *testing.T, selector string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, req := range strings.Split(selector, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(req), "=")
		if !ok || strings.ContainsAny(v, "=!") {
			t.Fatalf("unsupported label selector requirement %q", req)
		}
		out[k] = v
	}
	return out
}

// TestJsonnetOperatorAvailability checks the jsonnet output, which uses its
// own labels and selectors.
func TestJsonnetOperatorAvailability(t *testing.T) {
	for dir, ha := range map[string]bool{
		"jsonnet/examples":    false,
		"jsonnet/examples-ha": true,
	} {
		t.Run(dir, func(t *testing.T) {
			var deployments []deployment
			var pdbs []podDisruptionBudget
			for _, path := range glob(t, dir+"/*.yaml") {
				deployments = append(deployments, decodeAll[deployment](t, path, "Deployment")...)
				pdbs = append(pdbs, decodeAll[podDisruptionBudget](t, path, "PodDisruptionBudget")...)
			}
			if len(deployments) != 1 {
				t.Fatalf("%s: expected exactly one Deployment, got %d", dir, len(deployments))
			}
			d := deployments[0]
			if !selects(d.Spec.Selector.MatchLabels, d.Spec.Template.Metadata.Labels) {
				t.Errorf("%s: deployment %s selector %v does not match its pod labels %v", dir, d.Metadata.Name, d.Spec.Selector.MatchLabels, d.Spec.Template.Metadata.Labels)
			}
			if ha {
				assertHighlyAvailable(t, dir, d.Metadata.Name, d.Spec, pdbs)
			} else {
				assertSingleReplica(t, dir, d.Metadata.Name, d.Spec, pdbs)
			}
		})
	}
}
