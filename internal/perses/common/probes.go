// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the \"License\");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an \"AS IS\" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"github.com/perses/perses-operator/api/v1alpha2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetProbes(perses *v1alpha2.Perses) (*v1.Probe, *v1.Probe) {
	var livenessProbe, readinessProbe *v1.Probe

	if perses.Spec.LivenessProbe != nil {
		livenessProbe = &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   perses.Spec.Config.APIPrefix + "/metrics",
					Port:   intstr.FromInt32(8080),
					Scheme: v1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: perses.Spec.LivenessProbe.InitialDelaySeconds,
			TimeoutSeconds:      perses.Spec.LivenessProbe.TimeoutSeconds,
			PeriodSeconds:       perses.Spec.LivenessProbe.PeriodSeconds,
			SuccessThreshold:    perses.Spec.LivenessProbe.SuccessThreshold,
			FailureThreshold:    perses.Spec.LivenessProbe.FailureThreshold,
		}
	}
	if perses.Spec.ReadinessProbe != nil {
		readinessProbe = &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   perses.Spec.Config.APIPrefix + "/metrics",
					Port:   intstr.FromInt32(8080),
					Scheme: v1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: perses.Spec.ReadinessProbe.InitialDelaySeconds,
			TimeoutSeconds:      perses.Spec.ReadinessProbe.TimeoutSeconds,
			PeriodSeconds:       perses.Spec.ReadinessProbe.PeriodSeconds,
			SuccessThreshold:    perses.Spec.ReadinessProbe.SuccessThreshold,
			FailureThreshold:    perses.Spec.ReadinessProbe.FailureThreshold,
		}
	}

	return livenessProbe, readinessProbe
}
