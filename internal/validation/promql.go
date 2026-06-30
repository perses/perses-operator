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
	"fmt"
	"regexp"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"

	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/dashboard"
)

const (
	prometheusTimeSeriesQueryKind = "PrometheusTimeSeriesQuery"
	prometheusPromQLVariableKind  = "PrometheusPromQLVariable"
)

// variablePattern matches Perses variable references like $variable or ${variable}
var variablePattern = regexp.MustCompile(`\$\{?[a-zA-Z_][a-zA-Z0-9_]*\}?`)

// PromQLLocation identifies where a PromQL expression was found in a dashboard.
type PromQLLocation struct {
	// Panel-level query location
	PanelName  string
	QueryIndex int
	QueryName  string
	// Variable-level location
	VariableName string
}

func (l PromQLLocation) String() string {
	if l.VariableName != "" {
		return fmt.Sprintf("variable %q", l.VariableName)
	}
	if l.QueryName != "" {
		return fmt.Sprintf("panel %q query %q (index %d)", l.PanelName, l.QueryName, l.QueryIndex)
	}
	return fmt.Sprintf("panel %q query index %d", l.PanelName, l.QueryIndex)
}

// PromQLError represents a PromQL validation error with location context.
type PromQLError struct {
	Location PromQLLocation
	Expr     string
	Err      error
}

func (e *PromQLError) Error() string {
	return fmt.Sprintf("%s: invalid PromQL expression %q: %v", e.Location, e.Expr, e.Err)
}

func (e *PromQLError) Unwrap() error {
	return e.Err
}

// ValidatePromQLInDashboard validates all PromQL expressions found in dashboard
// panels and variables. It returns a list of validation errors for any invalid
// PromQL expressions. Queries containing variable references ($var or ${var})
// are skipped since they cannot be parsed without variable substitution.
func ValidatePromQLInDashboard(spec persesv1.DashboardSpec) []*PromQLError {
	var errs []*PromQLError

	for panelName, panel := range spec.Panels {
		if panel == nil {
			continue
		}
		panelErrs := validatePanelQueries(panelName, panel.Spec.Queries)
		errs = append(errs, panelErrs...)
	}

	varErrs := validateVariables(spec.Variables)
	errs = append(errs, varErrs...)

	return errs
}

func validatePanelQueries(panelName string, queries []persesv1.Query) []*PromQLError {
	var errs []*PromQLError

	for i, q := range queries {
		expr, ok := extractPromQLFromQuery(q)
		if !ok {
			continue
		}

		if expr == "" {
			continue
		}

		// Skip queries that contain variable references — they won't parse correctly
		if containsVariableRef(expr) {
			continue
		}

		if err := validatePromQLExpr(expr); err != nil {
			errs = append(errs, &PromQLError{
				Location: PromQLLocation{
					PanelName:  panelName,
					QueryIndex: i,
					QueryName:  q.Spec.Name,
				},
				Expr: expr,
				Err:  err,
			})
		}
	}

	return errs
}

func validateVariables(variables []dashboard.Variable) []*PromQLError {
	var errs []*PromQLError

	for _, v := range variables {
		listSpec, ok := v.Spec.(*dashboard.ListVariableSpec)
		if !ok {
			continue
		}

		if listSpec.Plugin.Kind != prometheusPromQLVariableKind {
			continue
		}

		specMap, ok := listSpec.Plugin.Spec.(map[string]any)
		if !ok {
			continue
		}

		expr, ok := specMap["expr"].(string)
		if !ok || expr == "" {
			continue
		}

		expr = strings.TrimSpace(expr)

		if containsVariableRef(expr) {
			continue
		}

		if err := validatePromQLExpr(expr); err != nil {
			errs = append(errs, &PromQLError{
				Location: PromQLLocation{
					VariableName: listSpec.Name,
				},
				Expr: expr,
				Err:  err,
			})
		}
	}

	return errs
}

// extractPromQLFromQuery extracts the PromQL expression from a query if it's a
// PrometheusTimeSeriesQuery. Returns the expression and true if found.
func extractPromQLFromQuery(q persesv1.Query) (string, bool) {
	if q.Spec.Plugin.Kind != prometheusTimeSeriesQueryKind {
		return "", false
	}

	specMap, ok := q.Spec.Plugin.Spec.(map[string]any)
	if !ok {
		return "", false
	}

	queryStr, ok := specMap["query"].(string)
	if !ok {
		return "", false
	}

	return strings.TrimSpace(queryStr), true
}

// containsVariableRef checks whether an expression contains Perses variable references.
func containsVariableRef(expr string) bool {
	return variablePattern.MatchString(expr)
}

// validatePromQLExpr parses and validates a PromQL expression.
func validatePromQLExpr(expr string) error {
	p := parser.NewParser(parser.Options{})
	_, err := p.ParseExpr(expr)
	return err
}
