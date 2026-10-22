// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/perses/common/set"
	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	speccommon "github.com/perses/spec/go/common"
	specDashboard "github.com/perses/spec/go/dashboard"
	specdatasource "github.com/perses/spec/go/datasource"
	specplugin "github.com/perses/spec/go/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardInSync(t *testing.T) {
	baseDashboard := func() *persesv1.Dashboard {
		return &persesv1.Dashboard{
			Kind: persesv1.KindDashboard,
			Metadata: persesv1.ProjectMetadata{
				Metadata: persesv1.Metadata{
					Name: "test-dashboard",
					Tags: set.New("tag1", "tag2"),
				},
			},
			Spec: specDashboard.Spec{
				Duration: "1h",
				Display:  &speccommon.Display{Name: "Test"},
			},
		}
	}

	t.Run("identical dashboards", func(t *testing.T) {
		assert.True(t, DashboardInSync(baseDashboard(), baseDashboard()))
	})

	t.Run("different tags", func(t *testing.T) {
		existing := baseDashboard()
		desired := baseDashboard()
		desired.Metadata.Tags = set.New("tag3")
		assert.False(t, DashboardInSync(existing, desired))
	})

	t.Run("different spec", func(t *testing.T) {
		existing := baseDashboard()
		desired := baseDashboard()
		desired.Spec.Duration = "2h"
		assert.False(t, DashboardInSync(existing, desired))
	})

	t.Run("extra metadata on existing does not affect comparison", func(t *testing.T) {
		existing := baseDashboard()
		existing.Metadata.Version = 5
		existing.Metadata.CreatedAt = time.Now()
		existing.Metadata.UpdatedAt = time.Now()
		assert.True(t, DashboardInSync(existing, baseDashboard()))
	})

	t.Run("nil tags on both", func(t *testing.T) {
		existing := baseDashboard()
		desired := baseDashboard()
		existing.Metadata.Tags = nil
		desired.Metadata.Tags = nil
		assert.True(t, DashboardInSync(existing, desired))
	})

	t.Run("empty tags on both", func(t *testing.T) {
		existing := baseDashboard()
		desired := baseDashboard()
		existing.Metadata.Tags = set.Set[string]{}
		desired.Metadata.Tags = set.Set[string]{}
		assert.True(t, DashboardInSync(existing, desired))
	})
}

func TestDashboardInSyncWithLayoutReferences(t *testing.T) {
	const specJSON = `{
		"duration": "5m",
		"panels": {
			"firstPanel": {
				"kind": "Panel",
				"spec": {
					"display": {"name": "Test Panel"},
					"plugin": {"kind": "TimeSeriesChart", "spec": {}},
					"queries": [{"kind": "TimeSeriesQuery", "spec": {
						"plugin": {"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}
					}}]
				}
			},
			"secondPanel": {
				"kind": "Panel",
				"spec": {
					"display": {"name": "Test Panel"},
					"plugin": {"kind": "TimeSeriesChart", "spec": {}},
					"queries": [{"kind": "TimeSeriesQuery", "spec": {
						"plugin": {"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}
					}}]
				}
			}
		},
		"layouts": [{"kind": "Grid", "spec": {"items": [{
			"x": 0, "y": 0, "width": 24, "height": 6,
			"content": {"$ref": "#/spec/panels/firstPanel"}
		}]}}]
	}`

	for _, tc := range []struct {
		name   string
		mutate func(*persesv1.Dashboard)
		want   bool
	}{
		{
			name: "API-resolved references match the Kubernetes spec",
			want: true,
		},
		{
			name: "panel edits are detected",
			mutate: func(dashboard *persesv1.Dashboard) {
				dashboard.Spec.Panels["firstPanel"].Spec.Display.Name = "Changed Panel"
			},
		},
		{
			name: "query edits are detected",
			mutate: func(dashboard *persesv1.Dashboard) {
				dashboard.Spec.Panels["firstPanel"].Spec.Queries[0].Spec.Plugin.Spec.(map[string]any)["query"] = "rate(up[5m])"
			},
		},
		{
			name: "reference changes between identical panels are detected",
			mutate: func(dashboard *persesv1.Dashboard) {
				ref := dashboard.Spec.Layouts[0].Spec.(*specDashboard.GridLayoutSpec).Items[0].Content
				ref.Ref = "#/spec/panels/secondPanel"
				ref.Path = []string{"spec", "panels", "secondPanel"}
				ref.Object = dashboard.Spec.Panels["secondPanel"]
			},
		},
		{
			name: "layout edits are detected",
			mutate: func(dashboard *persesv1.Dashboard) {
				dashboard.Spec.Layouts[0].Spec.(*specDashboard.GridLayoutSpec).Items[0].Width = 12
			},
		},
		{
			name: "tag edits are detected",
			mutate: func(dashboard *persesv1.Dashboard) {
				dashboard.Metadata.Tags = set.New("changed")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var existing persesv1.Dashboard
			require.NoError(t, json.Unmarshal([]byte(`{"kind":"Dashboard","metadata":{"name":"test-dashboard"},"spec":`+specJSON+`}`), &existing))
			var cr persesv1alpha2.PersesDashboard
			require.NoError(t, json.Unmarshal([]byte(`{"spec":{"config":`+specJSON+`}}`), &cr))
			desired := &persesv1.Dashboard{Spec: cr.Spec.Config.Spec}

			existingRef := existing.Spec.Layouts[0].Spec.(*specDashboard.GridLayoutSpec).Items[0].Content
			desiredRef := desired.Spec.Layouts[0].Spec.(*specDashboard.GridLayoutSpec).Items[0].Content
			require.NotNil(t, existingRef.Object)
			require.Nil(t, desiredRef.Object)

			if tc.mutate != nil {
				tc.mutate(&existing)
			}
			assert.Equal(t, tc.want, DashboardInSync(&existing, desired))
			assert.NotNil(t, existingRef.Object, "comparison must not mutate the API dashboard")
			assert.Nil(t, desiredRef.Object, "comparison must not mutate the desired dashboard")
		})
	}
}

func TestDatasourceInSync(t *testing.T) {
	baseDatasource := func() *persesv1.Datasource {
		return &persesv1.Datasource{
			Kind: persesv1.KindDatasource,
			Metadata: persesv1.ProjectMetadata{
				Metadata: persesv1.Metadata{
					Name: "test-ds",
					Tags: set.New("ds-tag"),
				},
			},
			Spec: specdatasource.Spec{
				Default: true,
				Plugin: specplugin.Plugin{
					Kind: "PrometheusDatasource",
				},
			},
		}
	}

	t.Run("identical datasources", func(t *testing.T) {
		assert.True(t, DatasourceInSync(baseDatasource(), baseDatasource()))
	})

	t.Run("different tags", func(t *testing.T) {
		existing := baseDatasource()
		desired := baseDatasource()
		desired.Metadata.Tags = set.New("other-tag")
		assert.False(t, DatasourceInSync(existing, desired))
	})

	t.Run("different spec", func(t *testing.T) {
		existing := baseDatasource()
		desired := baseDatasource()
		desired.Spec.Default = false
		assert.False(t, DatasourceInSync(existing, desired))
	})

	t.Run("extra metadata on existing", func(t *testing.T) {
		existing := baseDatasource()
		existing.Metadata.Version = 3
		existing.Metadata.CreatedAt = time.Now()
		assert.True(t, DatasourceInSync(existing, baseDatasource()))
	})
}

func TestGlobalDatasourceInSync(t *testing.T) {
	baseGlobalDS := func() *persesv1.GlobalDatasource {
		return &persesv1.GlobalDatasource{
			Kind: persesv1.KindGlobalDatasource,
			Metadata: persesv1.Metadata{
				Name: "test-gds",
				Tags: set.New("gds-tag"),
			},
			Spec: specdatasource.Spec{
				Default: true,
				Plugin: specplugin.Plugin{
					Kind: "PrometheusDatasource",
				},
			},
		}
	}

	t.Run("identical global datasources", func(t *testing.T) {
		assert.True(t, GlobalDatasourceInSync(baseGlobalDS(), baseGlobalDS()))
	})

	t.Run("different tags", func(t *testing.T) {
		existing := baseGlobalDS()
		desired := baseGlobalDS()
		desired.Metadata.Tags = set.New("other-tag")
		assert.False(t, GlobalDatasourceInSync(existing, desired))
	})

	t.Run("different spec", func(t *testing.T) {
		existing := baseGlobalDS()
		desired := baseGlobalDS()
		desired.Spec.Default = false
		assert.False(t, GlobalDatasourceInSync(existing, desired))
	})

	t.Run("extra metadata on existing", func(t *testing.T) {
		existing := baseGlobalDS()
		existing.Metadata.Version = 10
		existing.Metadata.CreatedAt = time.Now()
		assert.True(t, GlobalDatasourceInSync(existing, baseGlobalDS()))
	})
}
