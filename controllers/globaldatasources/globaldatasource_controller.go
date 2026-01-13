/*
Copyright 2025 The Perses Authors.

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persescommon "github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

var gdlog = logger.WithField("module", "globaldatasource_controller")

func (r *PersesGlobalDatasourceReconciler) reconcileGlobalDatasourcesInAllInstances(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	persesInstances := &persesv1alpha2.PersesList{}
	var opts []client.ListOption
	err := r.List(ctx, persesInstances, opts...)
	if err != nil {
		gdlog.WithError(err).Error("Failed to get perses instances")
		return subreconciler.RequeueWithError(err)
	}

	if len(persesInstances.Items) == 0 {
		gdlog.Info("No Perses instances found, requeue in 1 minute")
		return subreconciler.RequeueWithDelay(time.Minute)
	}

	globaldatasource := &persesv1alpha2.PersesGlobalDatasource{}

	if r, err := r.getLatestPersesGlobalDatasource(ctx, req, globaldatasource); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	for _, persesInstance := range persesInstances.Items {
		if r, err := r.syncPersesGlobalDatasource(ctx, persesInstance, globaldatasource); subreconciler.ShouldHaltOrRequeue(r, err) {
			return r, err
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesGlobalDatasourceReconciler) syncPersesGlobalDatasource(ctx context.Context, perses persesv1alpha2.Perses, globaldatasource *persesv1alpha2.PersesGlobalDatasource) (*ctrl.Result, error) {
	persesClient, err := r.ClientFactory.CreateClient(ctx, r.Client, perses)

	if err != nil {
		gdlog.WithError(err).Error("Failed to create perses rest client")
		return subreconciler.RequeueWithError(err)
	}

	// create a secret holding the secret configuration so the globaldatasource can reference it
	if persescommon.HasSecretConfig(globaldatasource.Spec.Client) {
		_, err = r.syncPersesGlobalSecret(ctx, persesClient, globaldatasource)
		if err != nil {
			gdlog.WithError(err).Errorf("Failed to create globaldatasource secret: %s", globaldatasource.Name)
			return subreconciler.RequeueWithError(err)
		}
	}

	_, err = persesClient.GlobalDatasource().Get(globaldatasource.Name)

	globalDatasourceWithName := &persesv1.GlobalDatasource{
		Kind: persesv1.KindGlobalDatasource,
		Metadata: persesv1.Metadata{
			Name: globaldatasource.Name,
		},
		Spec: globaldatasource.Spec.Config.DatasourceSpec,
	}

	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err = persesClient.GlobalDatasource().Create(globalDatasourceWithName)

			if err != nil {
				gdlog.WithError(err).Errorf("Failed to create globaldatasource: %s", globaldatasource.Name)
				return subreconciler.RequeueWithError(err)
			}

			gdlog.Infof("GlobalDatasource created: %s", globaldatasource.Name)

			return subreconciler.ContinueReconciling()
		}

		return subreconciler.RequeueWithError(err)
	} else {
		_, err = persesClient.GlobalDatasource().Update(globalDatasourceWithName)

		if err != nil {
			gdlog.WithError(err).Errorf("Failed to update globaldatasource: %s", globaldatasource.Name)
			return subreconciler.RequeueWithError(err)
		}

		gdlog.Infof("GlobalDatasource updated: %s", globaldatasource.Name)
	}

	return subreconciler.ContinueReconciling()
}

// creates/updates a Perses Global Secret with configuration,
// retrieving cert/key data from Secrets, ConfigMaps, or files specified in the PersesGlobalDatasource.
func (r *PersesGlobalDatasourceReconciler) syncPersesGlobalSecret(ctx context.Context, persesClient v1.ClientInterface, datasource *persesv1alpha2.PersesGlobalDatasource) (*ctrl.Result, error) {
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
			passwordData, err := persescommon.GetBasicAuthData(ctx, r.Client, basicAuth.Namespace, datasourceName, basicAuth)

			if err != nil {
				gdlog.WithFields(logger.Fields{
					"globaldatasource": datasourceName,
					"namespace":        basicAuth.Namespace,
					"error":            err,
				}).Error("Failed to get user basic auth password data for globaldatasource")
				return subreconciler.RequeueWithError(err)
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

		if oauth.AuthStyle != 0 {
			oAuthConfig.AuthStyle = oauth.AuthStyle
		}

		oAuthConfig.TokenURL = oauth.TokenURL
		switch oauth.Type {
		case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
			clientIDData, clientSecretData, err := persescommon.GetOAuthData(ctx, r.Client, oauth.Namespace, datasourceName, oauth)
			if err != nil {
				gdlog.WithFields(logger.Fields{
					"globaldatasource": datasourceName,
					"namespace":        oauth.Namespace,
					"error":            err,
				}).Error("Failed to get user oauth data for globaldatasource")
				return subreconciler.RequeueWithError(err)
			}

			oAuthConfig.ClientID = clientIDData
			oAuthConfig.ClientSecret = clientSecretData
		case persesv1alpha2.SecretSourceTypeFile:
			// the clientID is a Hidden field in perses API,
			// but doesn't expose it as a file field for it, so we need to read it and use the value
			clientID, err := os.ReadFile(oauth.ClientIDPath)
			if err != nil {
				return subreconciler.RequeueWithError(fmt.Errorf("failed to read the OAuth client ID file: %s", oauth.ClientIDPath))
			}
			oAuthConfig.ClientID = string(clientID)
			oAuthConfig.ClientSecretFile = oauth.ClientSecretPath
		}

		secretWithName.Spec.OAuth = oAuthConfig
	}

	if tls != nil {
		tlsConfig := &secret.TLSConfig{
			InsecureSkipVerify: tls.InsecureSkipVerify,
		}

		if tls.CaCert != nil {
			switch tls.CaCert.Type {
			case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
				caData, _, err := persescommon.GetTLSCertData(ctx, r.Client, tls.CaCert.Namespace, datasourceName, tls.CaCert)

				if err != nil {
					gdlog.WithFields(logger.Fields{
						"globaldatasource": datasourceName,
						"namespace":        tls.CaCert.Namespace,
						"error":            err,
					}).Error("Failed to get CA data for globaldatasource")
					return subreconciler.RequeueWithError(err)
				}

				tlsConfig.CA = caData
			case persesv1alpha2.SecretSourceTypeFile:
				tlsConfig.CAFile = tls.CaCert.CertPath
			}
		}

		if tls.UserCert != nil {
			switch tls.UserCert.Type {
			case persesv1alpha2.SecretSourceTypeSecret, persesv1alpha2.SecretSourceTypeConfigMap:
				certData, keyData, err := persescommon.GetTLSCertData(ctx, r.Client, tls.UserCert.Namespace, datasourceName, tls.UserCert)

				if err != nil {
					gdlog.WithFields(logger.Fields{
						"globaldatasource": datasourceName,
						"namespace":        tls.UserCert.Namespace,
						"error":            err,
					}).Error("Failed to get user certificate data for globaldatasource")
					return subreconciler.RequeueWithError(err)
				}

				tlsConfig.Cert = certData
				tlsConfig.Key = keyData
			case persesv1alpha2.SecretSourceTypeFile:
				tlsConfig.CertFile = tls.UserCert.CertPath

				if len(tls.UserCert.PrivateKeyPath) > 0 {
					tlsConfig.KeyFile = tls.UserCert.PrivateKeyPath
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
				return subreconciler.RequeueWithError(err)
			}

			gdlog.Infof("GlobalSecret created: %s", secretName)

			return subreconciler.ContinueReconciling()
		}

		return subreconciler.RequeueWithError(err)
	} else {
		_, err = persesClient.GlobalSecret().Update(secretWithName)

		if err != nil {
			gdlog.WithError(err).Errorf("Failed to update globalsecret: %s", secretName)
			return subreconciler.RequeueWithError(err)
		}

		gdlog.Infof("GlobalSecret updated: %s", secretName)
	}

	return subreconciler.ContinueReconciling()
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
