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

package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/perses/pkg/model/api/v1/dashboard"
	"github.com/perses/perses/pkg/model/api/v1/variable"
)

func makePromQLQuery(expr string) persesv1.Query {
	return persesv1.Query{
		Kind: "TimeSeriesQuery",
		Spec: persesv1.QuerySpec{
			Plugin: common.Plugin{
				Kind: "PrometheusTimeSeriesQuery",
				Spec: map[string]any{
					"query": expr,
				},
			},
		},
	}
}

func makeNonPromQLQuery() persesv1.Query {
	return persesv1.Query{
		Kind: "TraceQuery",
		Spec: persesv1.QuerySpec{
			Plugin: common.Plugin{
				Kind: "TempoTraceQuery",
				Spec: map[string]any{
					"query": `{service.name="frontend"}`,
				},
			},
		},
	}
}

func TestValidatePromQLInDashboard(t *testing.T) {
	tests := []struct {
		name        string
		panels      map[string]*persesv1.Panel
		wantErrCnt  int
		wantErrExpr string
	}{
		{
			name: "valid PromQL expression",
			panels: map[string]*persesv1.Panel{
				"cpu": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery("up"),
						},
					},
				},
			},
			wantErrCnt: 0,
		},
		{
			name: "valid complex PromQL expression",
			panels: map[string]*persesv1.Panel{
				"memory": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery(`rate(container_memory_rss{namespace="default"}[5m])`),
						},
					},
				},
			},
			wantErrCnt: 0,
		},
		{
			name: "invalid PromQL expression",
			panels: map[string]*persesv1.Panel{
				"broken": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery("rate(up[)"),
						},
					},
				},
			},
			wantErrCnt:  1,
			wantErrExpr: "rate(up[)",
		},
		{
			name: "query with variable references is skipped",
			panels: map[string]*persesv1.Panel{
				"variable": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery(`up{job=~"$job"}`),
						},
					},
				},
			},
			wantErrCnt: 0,
		},
		{
			name: "query with braced variable references is skipped",
			panels: map[string]*persesv1.Panel{
				"variable": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery(`rate(caddy_http_response_duration_seconds_sum[${interval}])`),
						},
					},
				},
			},
			wantErrCnt: 0,
		},
		{
			name: "non-PromQL query is skipped",
			panels: map[string]*persesv1.Panel{
				"traces": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makeNonPromQLQuery(),
						},
					},
				},
			},
			wantErrCnt: 0,
		},
		{
			name: "empty query expression is skipped",
			panels: map[string]*persesv1.Panel{
				"empty": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery(""),
						},
					},
				},
			},
			wantErrCnt: 0,
		},
		{
			name: "multiple panels with mixed validity",
			panels: map[string]*persesv1.Panel{
				"good": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery("up"),
						},
					},
				},
				"bad": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery("rate("),
						},
					},
				},
				"skipped": {
					Kind: "Panel",
					Spec: persesv1.PanelSpec{
						Queries: []persesv1.Query{
							makePromQLQuery(`up{job="$job"}`),
						},
					},
				},
			},
			wantErrCnt: 1,
		},
		{
			name:       "nil panel is skipped",
			panels:     map[string]*persesv1.Panel{"nilPanel": nil},
			wantErrCnt: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := persesv1.DashboardSpec{
				Panels: tt.panels,
			}
			errs := ValidatePromQLInDashboard(spec)
			assert.Len(t, errs, tt.wantErrCnt)
			if tt.wantErrExpr != "" && len(errs) > 0 {
				assert.Equal(t, tt.wantErrExpr, errs[0].Expr)
			}
		})
	}
}

func TestExtractPromQLFromQuery(t *testing.T) {
	t.Run("extracts from PrometheusTimeSeriesQuery", func(t *testing.T) {
		q := makePromQLQuery("up")
		expr, ok := extractPromQLFromQuery(q)
		require.True(t, ok)
		assert.Equal(t, "up", expr)
	})

	t.Run("returns false for non-Prometheus query", func(t *testing.T) {
		q := makeNonPromQLQuery()
		_, ok := extractPromQLFromQuery(q)
		assert.False(t, ok)
	})

	t.Run("returns false when spec is not a map", func(t *testing.T) {
		q := persesv1.Query{
			Kind: "TimeSeriesQuery",
			Spec: persesv1.QuerySpec{
				Plugin: common.Plugin{
					Kind: "PrometheusTimeSeriesQuery",
					Spec: "not-a-map",
				},
			},
		}
		_, ok := extractPromQLFromQuery(q)
		assert.False(t, ok)
	})

	t.Run("returns false when query field is not a string", func(t *testing.T) {
		q := persesv1.Query{
			Kind: "TimeSeriesQuery",
			Spec: persesv1.QuerySpec{
				Plugin: common.Plugin{
					Kind: "PrometheusTimeSeriesQuery",
					Spec: map[string]any{
						"query": 42,
					},
				},
			},
		}
		_, ok := extractPromQLFromQuery(q)
		assert.False(t, ok)
	})
}

func TestContainsVariableRef(t *testing.T) {
	tests := []struct {
		expr string
		want bool
	}{
		{"up", false},
		{`rate(metric[5m])`, false},
		{`up{job="$job"}`, true},
		{`up{job="${job}"}`, true},
		{`rate(metric[$interval])`, true},
		{`rate(metric[${interval}])`, true},
		{`sum by (namespace) (rate(container_cpu_usage_seconds_total{namespace="$namespace"}[5m]))`, true},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			assert.Equal(t, tt.want, containsVariableRef(tt.expr))
		})
	}
}

func makePromQLVariable(name, expr string) dashboard.Variable {
	return dashboard.Variable{
		Kind: "ListVariable",
		Spec: &dashboard.ListVariableSpec{
			ListSpec: variable.ListSpec{
				Plugin: common.Plugin{
					Kind: "PrometheusPromQLVariable",
					Spec: map[string]any{
						"expr":      expr,
						"labelName": "job",
					},
				},
			},
			Name: name,
		},
	}
}

func makeStaticVariable(name string) dashboard.Variable {
	return dashboard.Variable{
		Kind: "ListVariable",
		Spec: &dashboard.ListVariableSpec{
			ListSpec: variable.ListSpec{
				Plugin: common.Plugin{
					Kind: "StaticListVariable",
					Spec: map[string]any{
						"values": []string{"1m", "5m"},
					},
				},
			},
			Name: name,
		},
	}
}

func TestValidateVariables(t *testing.T) {
	tests := []struct {
		name        string
		variables   []dashboard.Variable
		wantErrCnt  int
		wantErrExpr string
	}{
		{
			name: "valid PromQL variable",
			variables: []dashboard.Variable{
				makePromQLVariable("job", `group by (job) (up)`),
			},
			wantErrCnt: 0,
		},
		{
			name: "invalid PromQL variable",
			variables: []dashboard.Variable{
				makePromQLVariable("broken", `group by (`),
			},
			wantErrCnt:  1,
			wantErrExpr: "group by (",
		},
		{
			name: "variable with variable reference is skipped",
			variables: []dashboard.Variable{
				makePromQLVariable("instance", `up{job="$job"}`),
			},
			wantErrCnt: 0,
		},
		{
			name: "non-PromQL variable is skipped",
			variables: []dashboard.Variable{
				makeStaticVariable("interval"),
			},
			wantErrCnt: 0,
		},
		{
			name: "empty expr is skipped",
			variables: []dashboard.Variable{
				makePromQLVariable("empty", ""),
			},
			wantErrCnt: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateVariables(tt.variables)
			assert.Len(t, errs, tt.wantErrCnt)
			if tt.wantErrExpr != "" && len(errs) > 0 {
				assert.Equal(t, tt.wantErrExpr, errs[0].Expr)
				assert.Equal(t, "broken", errs[0].Location.VariableName)
			}
		})
	}
}

func TestValidatePromQLInDashboard_WithVariables(t *testing.T) {
	spec := persesv1.DashboardSpec{
		Panels: map[string]*persesv1.Panel{
			"good": {
				Kind: "Panel",
				Spec: persesv1.PanelSpec{
					Queries: []persesv1.Query{
						makePromQLQuery("up"),
					},
				},
			},
		},
		Variables: []dashboard.Variable{
			makePromQLVariable("badvar", "rate("),
		},
	}
	errs := ValidatePromQLInDashboard(spec)
	require.Len(t, errs, 1)
	assert.Equal(t, "badvar", errs[0].Location.VariableName)
}

func TestPromQLLocation_String(t *testing.T) {
	t.Run("panel with query name", func(t *testing.T) {
		loc := PromQLLocation{PanelName: "cpu", QueryIndex: 0, QueryName: "cpu_usage"}
		assert.Equal(t, `panel "cpu" query "cpu_usage" (index 0)`, loc.String())
	})

	t.Run("panel without query name", func(t *testing.T) {
		loc := PromQLLocation{PanelName: "cpu", QueryIndex: 1}
		assert.Equal(t, `panel "cpu" query index 1`, loc.String())
	})

	t.Run("variable", func(t *testing.T) {
		loc := PromQLLocation{VariableName: "job"}
		assert.Equal(t, `variable "job"`, loc.String())
	})
}

func TestParsePromQLValidationMode(t *testing.T) {
	tests := []struct {
		input    string
		expected PromQLValidationMode
		wantErr  bool
	}{
		{"enforce", PromQLValidationEnforce, false},
		{"ENFORCE", PromQLValidationEnforce, false},
		{"warn", PromQLValidationWarn, false},
		{"WARN", PromQLValidationWarn, false},
		{"disabled", PromQLValidationDisabled, false},
		{"DISABLED", PromQLValidationDisabled, false},
		{"", PromQLValidationEnforce, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode, err := ParsePromQLValidationMode(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, mode)
			}
		})
	}
}

func TestDashboardValidator_Modes(t *testing.T) {
	invalidDashboard := &persesv1alpha1.PersesDashboard{}
	invalidDashboard.Name = "test-dashboard"
	invalidDashboard.Spec.DashboardSpec = persesv1.DashboardSpec{
		Panels: map[string]*persesv1.Panel{
			"broken": {
				Kind: "Panel",
				Spec: persesv1.PanelSpec{
					Queries: []persesv1.Query{
						makePromQLQuery("rate(up[)"),
					},
				},
			},
		},
	}

	validDashboard := &persesv1alpha1.PersesDashboard{}
	validDashboard.Name = "valid-dashboard"
	validDashboard.Spec.DashboardSpec = persesv1.DashboardSpec{
		Panels: map[string]*persesv1.Panel{
			"good": {
				Kind: "Panel",
				Spec: persesv1.PanelSpec{
					Queries: []persesv1.Query{
						makePromQLQuery("up"),
					},
				},
			},
		},
	}

	t.Run("enforce mode rejects invalid PromQL", func(t *testing.T) {
		v := &DashboardValidator{Mode: PromQLValidationEnforce}
		warnings, err := v.ValidateCreate(context.Background(), invalidDashboard)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PromQL")
		assert.Nil(t, warnings)
	})

	t.Run("enforce mode allows valid PromQL", func(t *testing.T) {
		v := &DashboardValidator{Mode: PromQLValidationEnforce}
		warnings, err := v.ValidateCreate(context.Background(), validDashboard)
		assert.NoError(t, err)
		assert.Nil(t, warnings)
	})

	t.Run("warn mode returns warnings for invalid PromQL", func(t *testing.T) {
		v := &DashboardValidator{Mode: PromQLValidationWarn}
		warnings, err := v.ValidateCreate(context.Background(), invalidDashboard)
		assert.NoError(t, err)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "invalid PromQL")
	})

	t.Run("warn mode no warnings for valid PromQL", func(t *testing.T) {
		v := &DashboardValidator{Mode: PromQLValidationWarn}
		warnings, err := v.ValidateCreate(context.Background(), validDashboard)
		assert.NoError(t, err)
		assert.Nil(t, warnings)
	})

	t.Run("disabled mode skips all validation", func(t *testing.T) {
		v := &DashboardValidator{Mode: PromQLValidationDisabled}
		warnings, err := v.ValidateCreate(context.Background(), invalidDashboard)
		assert.NoError(t, err)
		assert.Nil(t, warnings)
	})

	t.Run("update validates the new object", func(t *testing.T) {
		v := &DashboardValidator{Mode: PromQLValidationEnforce}
		warnings, err := v.ValidateUpdate(context.Background(), validDashboard, invalidDashboard)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PromQL")
		assert.Nil(t, warnings)
	})

	t.Run("delete always succeeds", func(t *testing.T) {
		v := &DashboardValidator{Mode: PromQLValidationEnforce}
		warnings, err := v.ValidateDelete(context.Background(), invalidDashboard)
		assert.NoError(t, err)
		assert.Nil(t, warnings)
	})
}
