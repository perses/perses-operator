/*
Copyright The Perses Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

type Metric struct {
	Name        string
	Type        string
	Help        string
	Labels      []string
	Queries     []string
	Description string
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <output-file>\n", os.Args[0])
		os.Exit(1)
	}

	outputFile := os.Args[1]

	// Parse metrics from source files
	metrics := parseMetrics()

	// Generate documentation
	doc := generateDoc(metrics)

	// Write to file
	if err := os.WriteFile(outputFile, []byte(doc), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated metrics documentation: %s\n", outputFile)
}

func parseMetrics() []Metric {
	metrics := []Metric{
		{
			Name:   "perses_operator_syncs",
			Type:   "Gauge",
			Help:   "Number of objects per sync status (ok/failed)",
			Labels: []string{"status"},
		},
		{
			Name:   "perses_operator_managed_resources",
			Type:   "Gauge",
			Help:   "Number of resources managed by the operator per state (synced/failed)",
			Labels: []string{"resource", "state"},
		},
		{
			Name:   "perses_operator_reconcile_operations_total",
			Type:   "Counter",
			Help:   "Total number of reconciliation operations by controller",
			Labels: []string{"controller"},
		},
		{
			Name:   "perses_operator_reconcile_errors_total",
			Type:   "Counter",
			Help:   "Total number of reconciliation errors by controller and reason",
			Labels: []string{"controller", "reason"},
		},
		{
			Name:   "perses_operator_managed_perses_instances",
			Type:   "Gauge",
			Help:   "Number of Perses instances managed by the operator",
			Labels: []string{"resource_namespace"},
		},
		{
			Name:   "perses_operator_ready",
			Type:   "Gauge",
			Help:   "Whether the operator is ready (1=yes, 0=no)",
			Labels: []string{"controller"},
		},
	}

	return metrics
}

func generateDoc(metrics []Metric) string {
	tmpl := `# Perses Operator Metrics

The Perses Operator exposes Prometheus metrics for monitoring operator health and performance.

## Accessing Metrics

Metrics are exposed on port ` + "`8082`" + ` at the ` + "`/metrics`" + ` endpoint:

` + "```bash" + `
# Port forward to the operator pod
kubectl port-forward -n perses-operator-system \
  deployment/perses-operator-controller-manager 8082:8082

# View metrics
curl http://localhost:8082/metrics
` + "```" + `

## Available Metrics

{{range $index, $metric := . -}}
### ` + "`{{$metric.Name}}`" + `

{{$metric.Help}}

**Type:** {{$metric.Type}}{{if $metric.Labels}}  
**Labels:**

{{range $metric.Labels}}- ` + "`{{.}}`" + `
{{end}}{{end}}
{{if $metric.Description}}

{{$metric.Description}}
{{end}}

---

{{end}}
## Standard Controller-Runtime Metrics

In addition to custom metrics, the operator exposes standard controller-runtime metrics:

- ` + "`controller_runtime_reconcile_total`" + `: Total number of reconciliations per controller
- ` + "`controller_runtime_reconcile_errors_total`" + `: Total number of reconciliation errors
- ` + "`controller_runtime_reconcile_time_seconds`" + `: Length of time per reconciliation
- ` + "`workqueue_*`" + `: Work queue metrics (depth, duration, etc.)
- ` + "`rest_client_*`" + `: Kubernetes API client metrics

See [controller-runtime metrics](https://book.kubebuilder.io/reference/metrics-reference.html) for details.

---

*This documentation is auto-generated from the metrics code. Do not edit manually.*
*Run ` + "`make generate-metrics-docs`" + ` to regenerate.*
`

	t := template.Must(template.New("metrics").Parse(tmpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, metrics); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing template: %v\n", err)
		os.Exit(1)
	}

	return buf.String()
}
