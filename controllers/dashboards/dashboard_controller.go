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

package dashboards

import (
	"context"
	"errors"
	"time"

	"github.com/perses/perses/pkg/client/perseshttp"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
	logger "github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	persesv1alpha1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/perses/perses-operator/internal/subreconciler"
)

var dlog = logger.WithField("module", "dashboard_controller")

func (r *PersesDashboardReconciler) reconcileDashboardInAllInstances(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	persesInstances := &persesv1alpha1.PersesList{}
	var opts []client.ListOption
	err := r.List(ctx, persesInstances, opts...)
	if err != nil {
		dlog.WithError(err).Error("Failed to get perses instances")
		return subreconciler.RequeueWithError(err)
	}

	if len(persesInstances.Items) == 0 {
		dlog.Info("No Perses instances found, retrying in 1 minute")
		return subreconciler.RequeueWithDelay(time.Minute)
	}

	dashboard := &persesv1alpha1.PersesDashboard{}

	if r, err := r.getLatestPersesDashboard(ctx, req, dashboard); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	for _, persesInstance := range persesInstances.Items {
		if r, err := r.syncPersesDashboard(ctx, persesInstance, dashboard); subreconciler.ShouldHaltOrRequeue(r, err) {
			return r, err
		}
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDashboardReconciler) syncPersesDashboard(ctx context.Context, perses persesv1alpha1.Perses, dashboard *persesv1alpha1.PersesDashboard) (*ctrl.Result, error) {
	persesClient, err := r.ClientFactory.CreateClient(ctx, r.Client, perses)

	if err != nil {
		dlog.WithError(err).Error("Failed to create perses rest client")
		return subreconciler.RequeueWithError(err)
	}

	_, err = persesClient.Project().Get(dashboard.Namespace)

	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err := persesClient.Project().Create(&persesv1.Project{
				Kind: "Project",
				Metadata: persesv1.Metadata{
					Name: dashboard.Namespace,
				},
				Spec: persesv1.ProjectSpec{
					Display: &common.Display{
						Name: dashboard.Namespace,
					},
				},
			})

			if err != nil {
				dlog.WithError(err).Errorf("Failed to create perses project: %s", dashboard.Namespace)
				return subreconciler.RequeueWithError(err)
			}

			dlog.Infof("Project created: %s", dashboard.Namespace)
		} else {
			dlog.WithError(err).Errorf("project error: %s", dashboard.Namespace)
			return subreconciler.RequeueWithError(err)
		}
	}

	_, err = persesClient.Dashboard(dashboard.Namespace).Get(dashboard.Name)

	persesDashboard := &persesv1.Dashboard{
		Kind: persesv1.KindDashboard,
		Metadata: persesv1.ProjectMetadata{
			Metadata: persesv1.Metadata{
				Name: dashboard.Name,
			},
		},
		Spec: dashboard.Spec.DashboardSpec,
	}

	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			_, err = persesClient.Dashboard(dashboard.Namespace).Create(persesDashboard)

			if err != nil {
				dlog.WithError(err).Errorf("Failed to create dashboard: %s", dashboard.Name)
				return subreconciler.RequeueWithError(err)
			}

			dlog.Infof("Dashboard created: %s", dashboard.Name)

			return subreconciler.ContinueReconciling()
		}

		return subreconciler.RequeueWithError(err)
	} else {
		_, err = persesClient.Dashboard(dashboard.Namespace).Update(persesDashboard)

		if err != nil {
			dlog.WithError(err).Errorf("Failed to update dashboard: %s", dashboard.Name)
			return subreconciler.RequeueWithError(err)
		}

		dlog.Infof("Dashboard updated: %s", dashboard.Name)
	}

	return subreconciler.ContinueReconciling()
}

func (r *PersesDashboardReconciler) deleteDashboardInAllInstances(ctx context.Context, _ ctrl.Request, dashbboardNamespace string, dashboardName string) (*ctrl.Result, error) {
	persesInstances := &persesv1alpha1.PersesList{}
	var opts []client.ListOption
	err := r.List(ctx, persesInstances, opts...)
	if err != nil {
		dlog.WithError(err).Error("Failed to get perses instances")
		return subreconciler.RequeueWithError(err)
	}

	if len(persesInstances.Items) == 0 {
		dlog.Info("No Perses instances found")
		return subreconciler.DoNotRequeue()
	}

	for _, persesInstance := range persesInstances.Items {
		if r, err := r.deleteDashboard(ctx, persesInstance, dashbboardNamespace, dashboardName); subreconciler.ShouldHaltOrRequeue(r, err) {
			return r, err
		}
	}

	return subreconciler.DoNotRequeue()
}

func (r *PersesDashboardReconciler) deleteDashboard(ctx context.Context, perses persesv1alpha1.Perses, dashboardNamespace string, dashboardName string) (*ctrl.Result, error) {
	persesClient, err := r.ClientFactory.CreateClient(ctx, r.Client, perses)

	if err != nil {
		dlog.WithError(err).Error("Failed to create perses rest client")
		return subreconciler.RequeueWithError(err)
	}

	_, err = persesClient.Project().Get(dashboardNamespace)

	if err != nil {
		dlog.WithError(err).Errorf("project error: %s", dashboardNamespace)

		return subreconciler.RequeueWithError(err)
	}

	err = persesClient.Dashboard(dashboardNamespace).Delete(dashboardName)

	if err != nil && errors.Is(err, perseshttp.RequestNotFoundError) {
		dlog.Infof("Dashboard not found: %s", dashboardName)
	}

	dlog.Infof("Dashboard deleted: %s", dashboardName)

	return subreconciler.ContinueReconciling()
}
