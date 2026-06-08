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
	"maps"

	persesv1 "github.com/rhobs/perses/pkg/model/api/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

// DashboardInSync returns true if the existing dashboard in Perses
// matches the desired state (tags and spec).
func DashboardInSync(existing, desired *persesv1.Dashboard) bool {
	return maps.Equal(existing.Metadata.Tags, desired.Metadata.Tags) &&
		equality.Semantic.DeepEqual(existing.Spec, desired.Spec)
}

// DatasourceInSync returns true if the existing datasource in Perses
// matches the desired state (tags and spec).
func DatasourceInSync(existing, desired *persesv1.Datasource) bool {
	return maps.Equal(existing.Metadata.Tags, desired.Metadata.Tags) &&
		equality.Semantic.DeepEqual(existing.Spec, desired.Spec)
}

// GlobalDatasourceInSync returns true if the existing global datasource
// in Perses matches the desired state (tags and spec).
func GlobalDatasourceInSync(existing, desired *persesv1.GlobalDatasource) bool {
	return maps.Equal(existing.Metadata.Tags, desired.Metadata.Tags) &&
		equality.Semantic.DeepEqual(existing.Spec, desired.Spec)
}
