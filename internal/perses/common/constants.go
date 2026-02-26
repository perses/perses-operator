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
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	PersesNamespaceDomain     = "perses.dev"
	PersesFinalizer           = PersesNamespaceDomain + "/finalizer"
	PersesProvisioningVersion = PersesNamespaceDomain + "/provisioning-version"
	TypeAvailablePerses       = "Available"
	TypeDegradedPerses        = "Degraded"

	// Flags
	PersesServerURLFlag = "perses-server-url"

	// Volume names
	configVolumeName  = "config"
	StorageVolumeName = "storage"
	pluginsVolumeName = "plugins"

	// TLS volume names
	caVolumeName     = "ca"
	caMountPath      = "/ca"
	tlsVolumeName    = "tls"
	tlsCertMountPath = "/tls"

	// Mount paths
	storageMountPath  = "/perses"
	secretsMountPath  = "/etc/perses/provisioning/secrets"
	configMountPath   = "/etc/perses/config"
	pluginsMountPath  = "/etc/perses/plugins"
	defaultConfigPath = configMountPath + "/config.yaml"

	defaultFileMode = 420
)

type ConditionStatusReason string

const (
	// Failure to be used when unable to locate any perses backends
	ReasonMissingPerses        ConditionStatusReason = "PersesMissing"
	ReasonConnectionFailed     ConditionStatusReason = "PersesConnectionFailed"
	ReasonInvalidConfiguration ConditionStatusReason = "InvalidConfiguration"
	// Failure to be used when resource that started the reconciliation is unable to be found
	ReasonMissingResource ConditionStatusReason = "MissingResource"
	// Generic failure for when the reason is due to the backend returning an error
	ReasonBackendError ConditionStatusReason = "PersesBackendError"
)

// isTLSEnabled checks if TLS is enabled in the Perses configuration
func isTLSEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.TLS != nil &&
		perses.Spec.TLS.Enable != nil &&
		*perses.Spec.TLS.Enable
}

// hasTLSConfiguration checks if valid TLS configuration is present
func hasTLSConfiguration(perses *v1alpha2.Perses) bool {
	return isTLSEnabled(perses) &&
		perses.Spec.TLS.UserCert != nil &&
		perses.Spec.TLS.UserCert.CertPath != "" &&
		perses.Spec.TLS.UserCert.PrivateKeyPath != nil &&
		*perses.Spec.TLS.UserCert.PrivateKeyPath != ""
}

// isClientTLSEnabled checks if TLS is enabled in the Perses client configuration
func isClientTLSEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.Client != nil &&
		perses.Spec.Client.TLS != nil &&
		perses.Spec.Client.TLS.Enable != nil &&
		*perses.Spec.Client.TLS.Enable
}

func isKubernetesAuthEnabled(perses *v1alpha2.Perses) bool {
	return perses != nil &&
		perses.Spec.Client != nil &&
		perses.Spec.Client.KubernetesAuth != nil &&
		perses.Spec.Client.KubernetesAuth.Enable != nil &&
		*perses.Spec.Client.KubernetesAuth.Enable
}

// PersesBecameAvailable returns true when a Perses instance transitions
// to Available status, indicating its API is ready to accept requests.
func PersesBecameAvailable(oldObj, newObj client.Object) bool {
	oldPerses, ok := oldObj.(*v1alpha2.Perses)
	if !ok {
		return false
	}
	newPerses, ok := newObj.(*v1alpha2.Perses)
	if !ok {
		return false
	}
	wasAvailable := meta.IsStatusConditionTrue(oldPerses.Status.Conditions, TypeAvailablePerses)
	isAvailable := meta.IsStatusConditionTrue(newPerses.Status.Conditions, TypeAvailablePerses)
	return !wasAvailable && isAvailable
}

// PersesAvailabilityPredicate returns a predicate that triggers reconciliation
// when a Perses instance becomes available or is deleted. This is used by
// child resource controllers (Dashboard, Datasource, GlobalDatasource) to
// reconcile their resources when the parent Perses instance becomes ready.
func PersesAvailabilityPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return PersesBecameAvailable(e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
