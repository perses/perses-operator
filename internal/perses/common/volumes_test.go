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

package common

import (
	"github.com/perses/perses-operator/api/v1alpha2"
	persesconfig "github.com/perses/perses/pkg/model/api/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetVolumes", func() {
	DescribeTable("when getting volumes for a Perses instance",
		func(perses *v1alpha2.Perses, verify func(volumes []corev1.Volume)) {
			volumes := GetVolumes(perses)
			verify(volumes)
		},
		Entry("SQL database includes config and plugins volumes only",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec:       v1alpha2.PersesSpec{},
			},
			func(volumes []corev1.Volume) {
				Expect(volumes).To(HaveLen(2))
				Expect(volumes[0].Name).To(Equal(configVolumeName))
				Expect(volumes[1].Name).To(Equal(pluginsVolumeName))
				Expect(volumes[1].VolumeSource.EmptyDir).NotTo(BeNil())
			},
		),
		Entry("file database includes config, plugins, and storage volumes",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					Config: v1alpha2.PersesConfig{
						Config: persesconfig.Config{
							Database: persesconfig.Database{
								File: &persesconfig.File{Folder: "/perses"},
							},
						},
					},
					Storage: &v1alpha2.StorageConfiguration{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			func(volumes []corev1.Volume) {
				Expect(volumes).To(HaveLen(3))
				Expect(volumes[0].Name).To(Equal(configVolumeName))
				Expect(volumes[1].Name).To(Equal(pluginsVolumeName))
				Expect(volumes[2].Name).To(Equal(StorageVolumeName))
				Expect(volumes[2].VolumeSource.EmptyDir).NotTo(BeNil())
			},
		),
		Entry("user-defined volumes are appended",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					Volumes: []corev1.Volume{
						{
							Name: "extra-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
								},
							},
						},
					},
				},
			},
			func(volumes []corev1.Volume) {
				Expect(volumes).To(HaveLen(3))
				Expect(volumes[0].Name).To(Equal(configVolumeName))
				Expect(volumes[1].Name).To(Equal(pluginsVolumeName))
				Expect(volumes[2].Name).To(Equal("extra-config"))
				Expect(volumes[2].VolumeSource.ConfigMap).NotTo(BeNil())
				Expect(volumes[2].VolumeSource.ConfigMap.Name).To(Equal("my-config"))
			},
		),
	)
})

var _ = Describe("GetVolumeMounts", func() {
	DescribeTable("when getting volume mounts for a Perses instance",
		func(perses *v1alpha2.Perses, verify func(mounts []corev1.VolumeMount)) {
			mounts := GetVolumeMounts(perses)
			verify(mounts)
		},
		Entry("SQL database includes config and plugins mounts only",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec:       v1alpha2.PersesSpec{},
			},
			func(mounts []corev1.VolumeMount) {
				Expect(mounts).To(HaveLen(2))
				Expect(mounts[0].Name).To(Equal(configVolumeName))
				Expect(mounts[0].MountPath).To(Equal(configMountPath))
				Expect(mounts[0].ReadOnly).To(BeTrue())
				Expect(mounts[1].Name).To(Equal(pluginsVolumeName))
				Expect(mounts[1].MountPath).To(Equal(pluginsMountPath))
				Expect(mounts[1].ReadOnly).To(BeFalse())
			},
		),
		Entry("file database includes config, plugins, and storage mounts",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					Config: v1alpha2.PersesConfig{
						Config: persesconfig.Config{
							Database: persesconfig.Database{
								File: &persesconfig.File{Folder: "/perses"},
							},
						},
					},
				},
			},
			func(mounts []corev1.VolumeMount) {
				Expect(mounts).To(HaveLen(3))
				Expect(mounts[0].Name).To(Equal(configVolumeName))
				Expect(mounts[1].Name).To(Equal(pluginsVolumeName))
				Expect(mounts[2].Name).To(Equal(StorageVolumeName))
				Expect(mounts[2].MountPath).To(Equal(storageMountPath))
				Expect(mounts[2].ReadOnly).To(BeFalse())
			},
		),
		Entry("user-defined volume mounts are appended",
			&v1alpha2.Perses{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha2.PersesSpec{
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "extra-config",
							MountPath: "/etc/perses/extra",
							ReadOnly:  true,
						},
					},
				},
			},
			func(mounts []corev1.VolumeMount) {
				Expect(mounts).To(HaveLen(3))
				Expect(mounts[0].Name).To(Equal(configVolumeName))
				Expect(mounts[1].Name).To(Equal(pluginsVolumeName))
				Expect(mounts[2].Name).To(Equal("extra-config"))
				Expect(mounts[2].MountPath).To(Equal("/etc/perses/extra"))
				Expect(mounts[2].ReadOnly).To(BeTrue())
			},
		),
	)
})
