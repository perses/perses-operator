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
