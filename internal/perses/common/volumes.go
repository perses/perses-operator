package common

import (
	"github.com/perses/perses-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func GetVolumes(name string, tls *v1alpha1.TLS) []corev1.Volume {
	configName := GetConfigName(name)

	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configName,
					},
					DefaultMode: ptr.To[int32](420),
				},
			},
		},
		{
			Name: "storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if tls != nil && tls.Enable {
		switch tls.CaCert.Type {
		case v1alpha1.CertificateTypeSecret:
			volumes = append(volumes, corev1.Volume{
				Name: "ca",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tls.CaCert.Name,
						DefaultMode: &[]int32{420}[0],
					},
				},
			})
		case v1alpha1.CertificateTypeConfigMap:
			volumes = append(volumes, corev1.Volume{
				Name: "ca",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: tls.CaCert.Name,
						},
					},
				},
			})
		}

		if tls.UserCert != nil {
			switch tls.UserCert.Type {
			case v1alpha1.CertificateTypeSecret:
				volumes = append(volumes, corev1.Volume{
					Name: "tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  tls.UserCert.Name,
							DefaultMode: &[]int32{420}[0],
						},
					},
				})
			case v1alpha1.CertificateTypeConfigMap:
				volumes = append(volumes, corev1.Volume{
					Name: "tls",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: tls.UserCert.Name,
							},
							DefaultMode: &[]int32{420}[0],
						},
					},
				})
			}
		}
	}

	return volumes
}

func GetVolumeMounts(tls *v1alpha1.TLS) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "config",
			ReadOnly:  true,
			MountPath: "/perses/config",
		},
		{
			Name:      "storage",
			ReadOnly:  false,
			MountPath: "/etc/perses/storage",
		},
	}

	if tls != nil && tls.Enable {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "ca",
			ReadOnly:  true,
			MountPath: "/ca",
		})

		if tls.UserCert != nil {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      "tls",
				ReadOnly:  true,
				MountPath: "/tls",
			})
		}
	}

	return volumeMounts
}
