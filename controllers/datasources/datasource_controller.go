/*
Copyright 2023 The Perses Authors.

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

package datasources

import (
	"context"
	"errors"
	"time"

	v1 "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/perseshttp"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/perses/pkg/model/api/v1/secret"
	logger "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/perses/perses-operator/api/v1alpha1"
	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	persescommon "github.com/perses/perses-operator/internal/perses/common"
	"github.com/perses/perses-operator/internal/subreconciler"
)

const secretNameSuffix = "-secret"

var dlog = logger.WithField("module", "datasource_controller")

func (r *PersesDatasourceReconciler) reconcileDatasourcesInAllInstances(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	datasource := &persesv1alpha1.PersesDatasource{}

	if r, err := r.getLatestPersesDatasource(ctx, req, datasource); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(datasource.Spec.InstanceSelector)
	if err != nil {
		return subreconciler.RequeueWithError(err)
	}

	persesInstances := &persesv1alpha1.PersesList{}
	opts := &client.ListOptions{
		LabelSelector: labelSelector,
	}

	err = r.Client.List(ctx, persesInstances, opts)
	if err != nil {
		dlog.WithError(err).Error("Failed to get perses instances")
		return subreconciler.RequeueWithDelayAndError(time.Minute, err)
	}

	if len(persesInstances.Items) == 0 {
		dlog.Info("No Perses instances found, requeue in 1 minute")
		return subreconciler.RequeueWithDelay(time.Minute)
	}

	for _, persesInstance := range persesInstances.Items {
		if r, err := r.syncPersesDatasource(ctx, persesInstance, datasource); subreconciler.ShouldHaltOrRequeue(r, err) {
			return r, err
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDatasourceReconciler) syncPersesDatasource(ctx context.Context, perses persesv1alpha1.Perses, datasource *persesv1alpha1.PersesDatasource) (*ctrl.Result, error) {
	persesClient, err := r.ClientFactory.CreateClient(ctx, r.Client, perses)

	if err != nil {
		dlog.WithError(err).Error("Failed to create perses rest client")
		return subreconciler.RequeueWithError(err)
	}

	_, err = persesClient.Project().Get(datasource.Namespace)

	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err := persesClient.Project().Create(&persesv1.Project{
				Kind: "Project",
				Metadata: persesv1.Metadata{
					Name: datasource.Namespace,
				},
				Spec: persesv1.ProjectSpec{
					Display: &common.Display{
						Name: datasource.Namespace,
					},
				},
			})

			if err != nil {
				dlog.WithError(err).Errorf("Failed to create perses project: %s", datasource.Namespace)
				return subreconciler.RequeueWithError(err)
			}

			dlog.Infof("Project created: %s", datasource.Namespace)
		} else {
			dlog.WithError(err).Errorf("project error: %s", datasource.Namespace)
			return subreconciler.RequeueWithError(err)
		}
	}

	// create a secret holding the TLS configuration so the datasource can reference it
	if datasource.Spec.Client != nil && datasource.Spec.Client.TLS != nil && datasource.Spec.Client.TLS.Enable {
		_, err := r.syncPersesSecretForTLS(ctx, persesClient, datasource)

		if err != nil {
			dlog.WithError(err).Errorf("Failed to create datasource secret: %s", datasource.Name)
			return subreconciler.RequeueWithDelayAndError(time.Minute, err)
		}
	}

	_, err = persesClient.Datasource(datasource.Namespace).Get(datasource.Name)

	datasourceWithName := &persesv1.Datasource{
		Kind: persesv1.KindDatasource,
		Metadata: persesv1.ProjectMetadata{
			Metadata: persesv1.Metadata{
				Name: datasource.Name,
			},
		},
		Spec: datasource.Spec.Config.DatasourceSpec,
	}

	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err = persesClient.Datasource(datasource.Namespace).Create(datasourceWithName)

			if err != nil {
				dlog.WithError(err).Errorf("Failed to create datasource: %s", datasource.Name)
				return subreconciler.RequeueWithDelayAndError(time.Minute, err)
			}

			dlog.Infof("Datasource created: %s", datasource.Name)

			return subreconciler.ContinueReconciling()
		}

		return subreconciler.RequeueWithError(err)
	} else {
		_, err = persesClient.Datasource(datasource.Namespace).Update(datasourceWithName)

		if err != nil {
			dlog.WithError(err).Errorf("Failed to update datasource: %s", datasource.Name)
			return subreconciler.RequeueWithDelayAndError(time.Minute, err)
		}

		dlog.Infof("Datasource updated: %s", datasource.Name)
	}

	return subreconciler.ContinueReconciling()
}

// creates/updates a Perses Secret with TLS configuration,
// retrieving cert/key data from Secrets, ConfigMaps, or files specified in the PersesDatasource.
func (r *PersesDatasourceReconciler) syncPersesSecretForTLS(ctx context.Context, persesClient v1.ClientInterface, datasource *persesv1alpha1.PersesDatasource) (*ctrl.Result, error) {
	namespace := datasource.Namespace
	datasourceName := datasource.Name
	secretName := datasourceName + secretNameSuffix
	tls := datasource.Spec.Client.TLS

	tlsConfig := secret.TLSConfig{
		InsecureSkipVerify: tls.InsecureSkipVerify,
	}

	if tls.CaCert != nil {
		if tls.CaCert.Type == v1alpha1.CertificateTypeSecret || tls.CaCert.Type == v1alpha1.CertificateTypeConfigMap {
			caData, _, err := persescommon.GetTLSCertData(ctx, r.Client, namespace, datasourceName, tls.CaCert)

			if err != nil {
				dlog.WithFields(logger.Fields{
					"datasource": datasourceName,
					"namespace":  namespace,
					"error":      err,
				}).Error("Failed to get CA data for datasource")
				return subreconciler.RequeueWithError(err)
			}

			tlsConfig.CA = caData
		} else if tls.CaCert.Type == v1alpha1.CertificateTypeFile {
			tlsConfig.CAFile = tls.CaCert.CertPath
		}
	}

	if tls.UserCert != nil {
		if tls.UserCert.Type == v1alpha1.CertificateTypeSecret || tls.UserCert.Type == v1alpha1.CertificateTypeConfigMap {
			certData, keyData, err := persescommon.GetTLSCertData(ctx, r.Client, namespace, datasourceName, tls.UserCert)

			if err != nil {
				dlog.WithFields(logger.Fields{
					"datasource": datasourceName,
					"namespace":  namespace,
					"error":      err,
				}).Error("Failed to get user certifitate data for datasource")
				return subreconciler.RequeueWithError(err)
			}

			tlsConfig.Cert = certData
			tlsConfig.Key = keyData
		} else if tls.UserCert.Type == v1alpha1.CertificateTypeFile {
			tlsConfig.CertFile = tls.UserCert.CertPath

			if len(tls.UserCert.PrivateKeyPath) > 0 {
				tlsConfig.KeyFile = tls.UserCert.PrivateKeyPath
			}
		}
	}

	secretWithName := &persesv1.Secret{
		Kind: persesv1.KindSecret,
		Metadata: persesv1.ProjectMetadata{
			Metadata: persesv1.Metadata{
				Name: secretName,
			},
		},
		Spec: persesv1.SecretSpec{
			TLSConfig: &tlsConfig,
		},
	}

	_, err := persesClient.Secret(namespace).Get(secretName)

	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err = persesClient.Secret(namespace).Create(secretWithName)

			if err != nil {
				dlog.WithError(err).Errorf("Failed to create secret: %s", secretName)
				return subreconciler.RequeueWithDelayAndError(time.Minute, err)
			}

			dlog.Infof("Secret created: %s", secretName)

			return subreconciler.ContinueReconciling()
		}

		return subreconciler.RequeueWithError(err)
	} else {
		_, err = persesClient.Secret(namespace).Update(secretWithName)

		if err != nil {
			dlog.WithError(err).Errorf("Failed to update secret: %s", secretName)
			return subreconciler.RequeueWithDelayAndError(time.Minute, err)
		}

		dlog.Infof("Secret updated: %s", secretName)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDatasourceReconciler) deleteDatasourceInAllInstances(ctx context.Context, datasourceNamespace string, datasourceName string) (*ctrl.Result, error) {
	persesInstances := &persesv1alpha1.PersesList{}
	var opts []client.ListOption
	err := r.Client.List(ctx, persesInstances, opts...)
	if err != nil {
		dlog.WithError(err).Error("Failed to get perses instances")
		return subreconciler.RequeueWithError(err)
	}

	if len(persesInstances.Items) == 0 {
		dlog.Info("No Perses instances found")
		return subreconciler.DoNotRequeue()
	}

	for _, persesInstance := range persesInstances.Items {
		if r, err := r.deleteDatasource(ctx, persesInstance, datasourceNamespace, datasourceName); subreconciler.ShouldHaltOrRequeue(r, err) {
			return r, err
		}
	}

	return subreconciler.DoNotRequeue()
}

func (r *PersesDatasourceReconciler) deleteDatasource(ctx context.Context, perses persesv1alpha1.Perses, datasourceNamespace string, datasourceName string) (*ctrl.Result, error) {
	persesClient, err := r.ClientFactory.CreateClient(ctx, r.Client, perses)

	if err != nil {
		dlog.WithError(err).Error("Failed to create perses rest client")
		return subreconciler.RequeueWithError(err)
	}

	_, err = persesClient.Project().Get(datasourceNamespace)

	if err != nil {
		dlog.WithError(err).Errorf("project error: %s", datasourceNamespace)

		return subreconciler.RequeueWithError(err)
	}

	err = persesClient.Datasource(datasourceNamespace).Delete(datasourceName)

	if err != nil && errors.Is(err, perseshttp.RequestNotFoundError) {
		dlog.Infof("Datasource not found: %s", datasourceName)
	}

	dlog.Infof("Datasource deleted: %s", datasourceName)

	secretName := datasourceName + secretNameSuffix

	err = persesClient.Secret(datasourceNamespace).Delete(secretName)

	if err != nil && errors.Is(err, perseshttp.RequestNotFoundError) {
		dlog.Infof("Secret not found: %s", secretName)
	}

	dlog.Infof("Secret deleted: %s", secretName)

	return subreconciler.ContinueReconciling()
}
