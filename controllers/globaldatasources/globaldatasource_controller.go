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

package globaldatasources

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	v1 "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/perseshttp"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/secret"
	logger "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persescommon "github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

var gdlog = logger.WithField("module", "globaldatasource_controller")

func (r *PersesGlobalDatasourceReconciler) reconcileGlobalDatasourcesInAllInstances(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	globaldatasource, ok := globalDatasourceFromContext(ctx)
	if !ok {
		gdlog.Error("globaldatasource not found in context")
		res, err := subreconciler.RequeueWithError(fmt.Errorf("globaldatasource not found in context"))
		return r.setStatusToDegraded(ctx, req, res, persescommon.ReasonMissingResource, err)
	}

	var labelSelector labels.Selector
	if globaldatasource.Spec.InstanceSelector == nil {
		labelSelector = labels.Everything()
	} else {
		var err error
		labelSelector, err = metav1.LabelSelectorAsSelector(globaldatasource.Spec.InstanceSelector)
		if err != nil {
			return subreconciler.RequeueWithError(err)
		}
	}

	persesInstances := &persesv1alpha2.PersesList{}
	opts := &client.ListOptions{
		LabelSelector: labelSelector,
	}
	err := r.List(ctx, persesInstances, opts)
	if err != nil {
		gdlog.WithError(err).Error("Failed to get perses instances")
		res, err := subreconciler.RequeueWithError(err)
		return r.setStatusToDegraded(ctx, req, res, persescommon.ReasonMissingPerses, err)
	}

	if len(persesInstances.Items) == 0 {
		gdlog.Info("No Perses instances found, requeue in 1 minute")
		res, err := subreconciler.RequeueWithDelay(time.Minute)
		return r.setStatusToDegraded(ctx, req, res, persescommon.ReasonMissingPerses, err)
	}

	for _, persesInstance := range persesInstances.Items {
		if !meta.IsStatusConditionTrue(persesInstance.Status.Conditions, persescommon.TypeAvailablePerses) {
			gdlog.Infof("Skipping Perses instance %s/%s (not yet available)", persesInstance.Namespace, persesInstance.Name)
			continue
		}
		if res, reason, err := r.syncPersesGlobalDatasource(ctx, persesInstance, globaldatasource); subreconciler.ShouldHaltOrRequeue(res, err) {
			return r.setStatusToDegraded(ctx, req, res, reason, err)
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesGlobalDatasourceReconciler) syncPersesGlobalDatasource(ctx context.Context, perses persesv1alpha2.Perses, globaldatasource *persesv1alpha2.PersesGlobalDatasource) (*ctrl.Result, persescommon.ConditionStatusReason, error) {
	persesClient, err := r.ClientFactory.CreateClient(ctx, r.Client, perses)

	if err != nil {
		gdlog.WithError(err).Error("Failed to create perses rest client")
		return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonConnectionFailed)

	}

	// create a secret holding the secret configuration so the globaldatasource can reference it
	if persescommon.HasSecretConfig(globaldatasource.Spec.Client) {
		_, reason, err := r.syncPersesGlobalSecret(ctx, persesClient, globaldatasource)
		if err != nil {
			gdlog.WithError(err).Errorf("Failed to create globaldatasource secret: %s", globaldatasource.Name)
			return subreconciler.RequeueWithErrorAndReason(err, reason)
		}
	}

	_, err = persesClient.GlobalDatasource().Get(globaldatasource.Name)

	globalDatasourceWithName := &persesv1.GlobalDatasource{
		Kind: persesv1.KindGlobalDatasource,
		Metadata: persesv1.Metadata{
			Name: globaldatasource.Name,
			Tags: persescommon.ParseTags(globaldatasource.Annotations),
		},
		Spec: globaldatasource.Spec.Config.DatasourceSpec,
	}

	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err = persesClient.GlobalDatasource().Create(globalDatasourceWithName)

			if err != nil {
				gdlog.WithError(err).Errorf("Failed to create globaldatasource: %s", globaldatasource.Name)
				return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonBackendError)

			}

			gdlog.Infof("GlobalDatasource created: %s", globaldatasource.Name)

			res, err := subreconciler.ContinueReconciling()
			return res, "", err
		}

		res, err := subreconciler.RequeueWithError(err)
		return res, persescommon.ReasonBackendError, err
	} else {
		_, err = persesClient.GlobalDatasource().Update(globalDatasourceWithName)

		if err != nil {
			gdlog.WithError(err).Errorf("Failed to update globaldatasource: %s", globaldatasource.Name)
			return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonBackendError)
		}

		gdlog.Infof("GlobalDatasource updated: %s", globaldatasource.Name)
	}

	res, err := subreconciler.ContinueReconciling()
	return res, "", err
}

// creates/updates a Perses Global Secret with configuration,
// retrieving cert/key data from Secrets, ConfigMaps, or files specified in the PersesGlobalDatasource.
func (r *PersesGlobalDatasourceReconciler) syncPersesGlobalSecret(ctx context.Context, persesClient v1.ClientInterface, datasource *persesv1alpha2.PersesGlobalDatasource) (*ctrl.Result, persescommon.ConditionStatusReason, error) {
	datasourceName := datasource.Name
	secretName := datasourceName + persescommon.SecretNameSuffix
	basicAuth := datasource.Spec.Client.BasicAuth
	oauth := datasource.Spec.Client.OAuth
	tls := datasource.Spec.Client.TLS

	secretWithName := &persesv1.GlobalSecret{
		Kind: persesv1.KindGlobalSecret,
		Metadata: persesv1.Metadata{
			Name: secretName,
		},
		Spec: persesv1.SecretSpec{},
	}

	if basicAuth != nil {
		basicAuthConfig := &secret.BasicAuth{}
		basicAuthConfig.Username = basicAuth.Username

		switch basicAuth.Type {
		case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
			namespace := ""
			if basicAuth.Namespace != nil {
				namespace = *basicAuth.Namespace
			}
			passwordData, err := persescommon.GetBasicAuthData(ctx, r.Client, namespace, datasourceName, basicAuth)

			if err != nil {
				gdlog.WithFields(logger.Fields{
					"globaldatasource": datasourceName,
					"namespace":        basicAuth.Namespace,
					"error":            err,
				}).Error("Failed to get user basic auth password data for globaldatasource")
				return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonInvalidConfiguration)
			}

			basicAuthConfig.Password = passwordData
		case persesv1alpha2.SecretSourceTypeFile:
			basicAuthConfig.PasswordFile = basicAuth.PasswordPath
		}

		secretWithName.Spec.BasicAuth = basicAuthConfig
	}

	if oauth != nil {
		oAuthConfig := &secret.OAuth{
			TokenURL:       oauth.TokenURL,
			Scopes:         oauth.Scopes,
			EndpointParams: oauth.EndpointParams,
		}

		if oauth.AuthStyle != nil && *oauth.AuthStyle != 0 {
			oAuthConfig.AuthStyle = int(*oauth.AuthStyle)
		}

		oAuthConfig.TokenURL = oauth.TokenURL
		switch oauth.Type {
		case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
			namespace := ""
			if oauth.Namespace != nil {
				namespace = *oauth.Namespace
			}
			clientIDData, clientSecretData, err := persescommon.GetOAuthData(ctx, r.Client, namespace, datasourceName, oauth)
			if err != nil {
				gdlog.WithFields(logger.Fields{
					"globaldatasource": datasourceName,
					"namespace":        oauth.Namespace,
					"error":            err,
				}).Error("Failed to get user oauth data for globaldatasource")
				return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonInvalidConfiguration)
			}

			oAuthConfig.ClientID = clientIDData
			oAuthConfig.ClientSecret = clientSecretData
		case persesv1alpha2.SecretSourceTypeFile:
			// the clientID is a Hidden field in perses API,
			// but doesn't expose it as a file field for it, so we need to read it and use the value
			clientIDPath := ""
			if oauth.ClientIDPath != nil {
				clientIDPath = *oauth.ClientIDPath
			}
			clientID, err := os.ReadFile(clientIDPath)
			if err != nil {
				err = fmt.Errorf("failed to read the OAuth client ID file: %s", clientIDPath)
				return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonInvalidConfiguration)
			}
			oAuthConfig.ClientID = string(clientID)
			if oauth.ClientSecretPath != nil {
				oAuthConfig.ClientSecretFile = *oauth.ClientSecretPath
			}
		}

		secretWithName.Spec.OAuth = oAuthConfig
	}

	if tls != nil {
		insecureSkipVerify := false
		if tls.InsecureSkipVerify != nil {
			insecureSkipVerify = *tls.InsecureSkipVerify
		}
		tlsConfig := &secret.TLSConfig{
			InsecureSkipVerify: insecureSkipVerify,
		}

		if tls.CaCert != nil {
			switch tls.CaCert.Type {
			case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
				caCertNamespace := ""
				if tls.CaCert.Namespace != nil {
					caCertNamespace = *tls.CaCert.Namespace
				}
				caData, _, err := persescommon.GetTLSCertData(ctx, r.Client, caCertNamespace, datasourceName, tls.CaCert)

				if err != nil {
					gdlog.WithFields(logger.Fields{
						"globaldatasource": datasourceName,
						"namespace":        tls.CaCert.Namespace,
						"error":            err,
					}).Error("Failed to get CA data for globaldatasource")
					return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonInvalidConfiguration)
				}

				tlsConfig.CA = caData
			case persesv1alpha2.SecretSourceTypeFile:
				tlsConfig.CAFile = tls.CaCert.CertPath
			}
		}

		if tls.UserCert != nil {
			switch tls.UserCert.Type {
			case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
				userCertNamespace := ""
				if tls.UserCert.Namespace != nil {
					userCertNamespace = *tls.UserCert.Namespace
				}
				certData, keyData, err := persescommon.GetTLSCertData(ctx, r.Client, userCertNamespace, datasourceName, tls.UserCert)

				if err != nil {
					gdlog.WithFields(logger.Fields{
						"globaldatasource": datasourceName,
						"namespace":        tls.UserCert.Namespace,
						"error":            err,
					}).Error("Failed to get user certificate data for globaldatasource")
					return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonInvalidConfiguration)
				}

				tlsConfig.Cert = certData
				tlsConfig.Key = keyData
			case persesv1alpha2.SecretSourceTypeFile:
				tlsConfig.CertFile = tls.UserCert.CertPath

				if tls.UserCert.PrivateKeyPath != nil && len(*tls.UserCert.PrivateKeyPath) > 0 {
					tlsConfig.KeyFile = *tls.UserCert.PrivateKeyPath
				}
			}
		}

		secretWithName.Spec.TLSConfig = tlsConfig
	}

	_, err := persesClient.GlobalSecret().Get(secretName)

	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err = persesClient.GlobalSecret().Create(secretWithName)

			if err != nil {
				gdlog.WithError(err).Errorf("Failed to create globalsecret: %s", secretName)
				return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonBackendError)
			}

			gdlog.Infof("GlobalSecret created: %s", secretName)

			res, err := subreconciler.ContinueReconciling()
			return res, "", err
		}

		return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonBackendError)
	} else {
		_, err = persesClient.GlobalSecret().Update(secretWithName)

		if err != nil {
			gdlog.WithError(err).Errorf("Failed to update globalsecret: %s", secretName)
			return subreconciler.RequeueWithErrorAndReason(err, persescommon.ReasonBackendError)
		}

		gdlog.Infof("GlobalSecret updated: %s", secretName)
	}

	res, err := subreconciler.ContinueReconciling()
	return res, "", err
}

func (r *PersesGlobalDatasourceReconciler) deleteGlobalDatasourceInAllInstances(ctx context.Context, datasourceName string) (*ctrl.Result, error) {
	persesInstances := &persesv1alpha2.PersesList{}
	var opts []client.ListOption
	err := r.List(ctx, persesInstances, opts...)
	if err != nil {
		gdlog.WithError(err).Error("Failed to get perses instances")
		return subreconciler.RequeueWithError(err)
	}

	if len(persesInstances.Items) == 0 {
		gdlog.Info("No Perses instances found")
		return subreconciler.DoNotRequeue()
	}

	for _, persesInstance := range persesInstances.Items {
		gdlog.Infof("Deleting perses instance: %s", persesInstance.Name)
		if r, err := r.deleteGlobalDatasource(ctx, persesInstance, datasourceName); subreconciler.ShouldHaltOrRequeue(r, err) {
			return r, err
		}
	}

	return subreconciler.DoNotRequeue()
}

func (r *PersesGlobalDatasourceReconciler) deleteGlobalDatasource(ctx context.Context, perses persesv1alpha2.Perses, datasourceName string) (*ctrl.Result, error) {
	persesClient, err := r.ClientFactory.CreateClient(ctx, r.Client, perses)

	if err != nil {
		gdlog.WithError(err).Error("Failed to create perses rest client")
		return subreconciler.RequeueWithError(err)
	}

	err = persesClient.GlobalDatasource().Delete(datasourceName)

	if err != nil && errors.Is(err, perseshttp.RequestNotFoundError) {
		gdlog.Infof("GlobalDatasource not found: %s", datasourceName)
	}

	gdlog.Infof("GlobalDatasource deleted: %s", datasourceName)

	secretName := datasourceName + persescommon.SecretNameSuffix

	err = persesClient.GlobalSecret().Delete(secretName)

	if err != nil && errors.Is(err, perseshttp.RequestNotFoundError) {
		gdlog.Infof("GlobalSecret not found: %s", secretName)
	}

	gdlog.Infof("GlobalSecret deleted: %s", secretName)

	return subreconciler.ContinueReconciling()
}
