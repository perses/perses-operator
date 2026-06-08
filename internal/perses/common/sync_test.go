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
	"testing"
	"time"

	"github.com/perses/common/set"
	dashboardSpec "github.com/perses/spec/go/dashboard"
	datasourceSpec "github.com/perses/spec/go/datasource"

	commonSpec "github.com/perses/spec/go/common"
	persesv1 "github.com/rhobs/perses/pkg/model/api/v1"
	"github.com/stretchr/testify/assert"
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
			Spec: dashboardSpec.Spec{
				Duration: "1h",
				Display:  &commonSpec.Display{Name: "Test"},
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
			Spec: datasourceSpec.Spec{
				Default: true,
				Plugin: commonSpec.Plugin{
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
			Spec: datasourceSpec.Spec{
				Default: true,
				Plugin: commonSpec.Plugin{
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
