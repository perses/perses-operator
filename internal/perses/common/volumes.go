package common

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/perses/perses-operator/api/v1alpha2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// GetVolumes returns the volumes needed for the Perses container
func GetVolumes(perses *v1alpha2.Perses) []corev1.Volume {
	configName := GetConfigName(perses.Name)

	volumes := []corev1.Volume{
		{
			Name: configVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configName,
					},
					DefaultMode: ptr.To[int32](defaultFileMode),
				},
			},
		},
	}

	if perses.Spec.Config.Database.File != nil {
		volumes = append(volumes, corev1.Volume{
			Name: StorageVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: GetStorageName(perses.Name),
				},
			},
		})
	} else {
		volumes = append(volumes, corev1.Volume{
			Name: StorageVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	// Add TLS volumes if enabled
	if isTLSEnabled(perses) {
		tls := perses.Spec.TLS

		// Add CA certificate volume if provided
		if tls.CaCert != nil {
			volumes = append(volumes, createCertVolume(caVolumeName, *tls.CaCert))
		}

		// Add user certificate volume if provided
		if tls.UserCert != nil {
			volumes = append(volumes, createCertVolume(tlsVolumeName, *tls.UserCert))
		}
	}

	// add provisioning secrets
	if perses.Spec.Provisioning != nil {
		for _, secret := range perses.Spec.Provisioning.SecretRefs {
			volumes = append(volumes, corev1.Volume{
				Name: secret.GetSecretVolumeName(),
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  secret.Key,
								Path: secret.String(),
							},
						},
					},
				},
			})
		}
	}

	return volumes
}

// createCertVolume creates a volume for a certificate based on its type (Secret or ConfigMap)
func createCertVolume(name string, cert v1alpha2.Certificate) corev1.Volume {
	volume := corev1.Volume{
		Name: name,
	}

	switch cert.Type {
	case v1alpha2.SecretSourceTypeSecret:
		volume.VolumeSource = corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  cert.Name,
				DefaultMode: ptr.To[int32](defaultFileMode),
			},
		}
	case v1alpha2.SecretSourceTypeConfigMap:
		volume.VolumeSource = corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cert.Name,
				},
				DefaultMode: ptr.To[int32](defaultFileMode),
			},
		}
	}

	return volume
}

// GetVolumeMounts returns the volume mounts needed for the Perses container
func GetVolumeMounts(perses *v1alpha2.Perses) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      configVolumeName,
			ReadOnly:  true,
			MountPath: configMountPath,
		},
		{
			Name:      StorageVolumeName,
			ReadOnly:  false,
			MountPath: storageMountPath,
		},
	}

	// Add TLS volume mounts if enabled
	if isTLSEnabled(perses) {
		if perses.Spec.TLS.CaCert != nil {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      caVolumeName,
				ReadOnly:  true,
				MountPath: caMountPath,
			})
		}

		if perses.Spec.TLS.UserCert != nil {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      tlsVolumeName,
				ReadOnly:  true,
				MountPath: tlsCertMountPath,
			})
		}
	}

	// add provisioning secrets
	if perses.Spec.Provisioning != nil {
		for _, secret := range perses.Spec.Provisioning.SecretRefs {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      secret.GetSecretVolumeName(),
				ReadOnly:  true,
				MountPath: filepath.Join(secretsMountPath, secret.String()),
				SubPath:   secret.String(),
			})
		}
	}

	return volumeMounts
}

// GetProvisioningHash generates a hash of the provisioning status data
func GetProvisioningHash(perses *v1alpha2.Perses) (string, error) {
	if perses.Status.Provisioning == nil {
		return "", nil
	}

	data, err := json.Marshal(perses.Status.Provisioning)
	if err != nil {
		return "", err
	}

	return rand.SafeEncodeString(fmt.Sprint(sha256.Sum256(data))), nil
}
