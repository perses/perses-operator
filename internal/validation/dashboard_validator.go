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
	"fmt"
	"strings"

	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PromQLValidationMode controls how PromQL validation errors are reported.
type PromQLValidationMode string

const (
	// PromQLValidationEnforce rejects dashboards with invalid PromQL at admission time.
	PromQLValidationEnforce PromQLValidationMode = "enforce"
	// PromQLValidationWarn allows dashboards through but returns validation errors as warnings.
	PromQLValidationWarn PromQLValidationMode = "warn"
	// PromQLValidationDisabled skips PromQL validation entirely.
	PromQLValidationDisabled PromQLValidationMode = "disabled"
)

// ParsePromQLValidationMode converts a string to a PromQLValidationMode, returning
// an error if the value is not recognized.
func ParsePromQLValidationMode(s string) (PromQLValidationMode, error) {
	switch strings.ToLower(s) {
	case string(PromQLValidationEnforce), "":
		return PromQLValidationEnforce, nil
	case string(PromQLValidationWarn):
		return PromQLValidationWarn, nil
	case string(PromQLValidationDisabled):
		return PromQLValidationDisabled, nil
	default:
		return "", fmt.Errorf("invalid promql-validation-mode %q: must be one of enforce, warn, disabled", s)
	}
}

// DashboardValidator validates PersesDashboard resources, including PromQL syntax.
type DashboardValidator struct {
	Mode PromQLValidationMode
}

var _ admission.Validator[*persesv1alpha1.PersesDashboard] = &DashboardValidator{}

func (v *DashboardValidator) ValidateCreate(_ context.Context, obj *persesv1alpha1.PersesDashboard) (admission.Warnings, error) {
	return v.validateDashboard(obj)
}

func (v *DashboardValidator) ValidateUpdate(_ context.Context, _, newObj *persesv1alpha1.PersesDashboard) (admission.Warnings, error) {
	return v.validateDashboard(newObj)
}

func (v *DashboardValidator) ValidateDelete(_ context.Context, _ *persesv1alpha1.PersesDashboard) (admission.Warnings, error) {
	return nil, nil
}

func (v *DashboardValidator) validateDashboard(dashboard *persesv1alpha1.PersesDashboard) (admission.Warnings, error) {
	if v.Mode == PromQLValidationDisabled {
		return nil, nil
	}

	promqlErrs := ValidatePromQLInDashboard(dashboard.Spec.DashboardSpec)
	if len(promqlErrs) == 0 {
		return nil, nil
	}

	var messages []string
	for _, e := range promqlErrs {
		messages = append(messages, e.Error())
	}

	summary := fmt.Sprintf("dashboard %q has invalid PromQL expressions:\n%s",
		dashboard.Name, strings.Join(messages, "\n"))

	if v.Mode == PromQLValidationWarn {
		return admission.Warnings{summary}, nil
	}

	return nil, fmt.Errorf("%s", summary)
}
