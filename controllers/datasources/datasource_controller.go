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

	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/perses/perses-operator/internal/subreconciler"
	"github.com/perses/perses/pkg/client/perseshttp"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	logger "github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var dlog = logger.WithField("module", "datasource_controller")

func (r *PersesDatasourceReconciler) reconcileDatasourcesInAllInstances(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
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

	datasource := &persesv1alpha1.PersesDatasource{}

	if r, err := r.getLatestPersesDatasource(ctx, req, datasource); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	for _, persesInstance := range persesInstances.Items {
		if r, err := r.syncPersesDatasource(ctx, persesInstance, datasource); subreconciler.ShouldHaltOrRequeue(r, err) {
			return r, err
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDatasourceReconciler) syncPersesDatasource(ctx context.Context, perses persesv1alpha1.Perses, datasource *persesv1alpha1.PersesDatasource) (*ctrl.Result, error) {
	persesClient, err := r.ClientFactory.CreateClient(perses)

	if err != nil {
		dlog.WithError(err).Error("Failed to create perses rest client")
		return subreconciler.RequeueWithError(err)
	}

	_, err = persesClient.Project().Get(datasource.Namespace)

	if err != nil {
		dlog.WithError(err).Errorf("project error: %s", datasource.Namespace)
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err := persesClient.Project().Create(&persesv1.Project{
				Kind: "Project",
				Metadata: persesv1.Metadata{
					Name: datasource.Namespace,
				},
			})

			if err != nil {
				dlog.WithError(err).Errorf("Failed to create perses project: %s", datasource.Namespace)
				return subreconciler.RequeueWithError(err)
			}

			dlog.Infof("Project created: %s", datasource.Namespace)
		}

		return subreconciler.RequeueWithError(err)
	}

	_, err = persesClient.Datasource(datasource.Namespace).Get(datasource.Name)

	datasourceWithName := &persesv1.Datasource{
		Kind: persesv1.KindDatasource,
		Metadata: persesv1.ProjectMetadata{
			Metadata: persesv1.Metadata{
				Name: datasource.Name,
			},
		},
		Spec: datasource.Spec.DatasourceSpec,
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

func (r *PersesDatasourceReconciler) deleteDatasourceInAllInstances(ctx context.Context, req ctrl.Request, datasourceNamespace string, datasourceName string) (*ctrl.Result, error) {
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
	persesClient, err := r.ClientFactory.CreateClient(perses)

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

	return subreconciler.ContinueReconciling()
}
