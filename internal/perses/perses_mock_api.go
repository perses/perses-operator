package perses

import (
	v1 "github.com/perses/perses/pkg/client/api/v1"
	modelv1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/stretchr/testify/mock"
)

type MockClient struct {
	v1.ClientInterface
	mock.Mock
}

type project struct {
	v1.ProjectInterface
}

func (p *project) Get(name string) (*modelv1.Project, error) {
	return nil, nil
}

func (c *MockClient) Project() v1.ProjectInterface {
	return &project{}
}

type MockDashboard struct {
	v1.DashboardInterface
	mock.Mock
}

func (c *MockClient) Dashboard(project string) v1.DashboardInterface {
	args := c.Called(project)
	return args.Get(0).(v1.DashboardInterface)
}

func (d *MockDashboard) Get(name string) (*modelv1.Dashboard, error) {
	args := d.Called(name)
	return args.Get(0).(*modelv1.Dashboard), args.Error(1)
}

func (d *MockDashboard) Update(dashboard *modelv1.Dashboard) (*modelv1.Dashboard, error) {
	args := d.Called(dashboard)
	return args.Get(0).(*modelv1.Dashboard), args.Error(1)
}

func (d *MockDashboard) Delete(name string) error {
	args := d.Called(name)
	return args.Error(0)
}

func (d *MockDashboard) Create(dashboard *modelv1.Dashboard) (*modelv1.Dashboard, error) {
	args := d.Called(dashboard)
	return args.Get(0).(*modelv1.Dashboard), args.Error(1)
}

type MockDatasource struct {
	v1.DatasourceInterface
	mock.Mock
}

func (c *MockClient) Datasource(project string) v1.DatasourceInterface {
	args := c.Called(project)
	return args.Get(0).(v1.DatasourceInterface)
}

func (d *MockDatasource) Get(name string) (*modelv1.Datasource, error) {
	args := d.Called(name)
	return args.Get(0).(*modelv1.Datasource), args.Error(1)
}

func (d *MockDatasource) Update(dashboard *modelv1.Datasource) (*modelv1.Datasource, error) {
	args := d.Called(dashboard)
	return args.Get(0).(*modelv1.Datasource), args.Error(1)
}

func (d *MockDatasource) Delete(name string) error {
	args := d.Called(name)
	return args.Error(0)
}

func (d *MockDatasource) Create(dashboard *modelv1.Datasource) (*modelv1.Datasource, error) {
	args := d.Called(dashboard)
	return args.Get(0).(*modelv1.Datasource), args.Error(1)
}

type MockSecret struct {
	v1.SecretInterface
	mock.Mock
}

func (c *MockClient) Secret(secretName string) v1.SecretInterface {
	args := c.Called(secretName)
	return args.Get(0).(v1.SecretInterface)
}

func (c *MockSecret) Create(secret *modelv1.Secret) (*modelv1.Secret, error) {
	args := c.Called(secret)
	return args.Get(0).(*modelv1.Secret), args.Error(1)
}

func (c *MockSecret) Get(name string) (*modelv1.Secret, error) {
	args := c.Called(name)
	return args.Get(0).(*modelv1.Secret), args.Error(1)
}

func (c *MockSecret) Delete(name string) error {
	args := c.Called(name)
	return args.Error(0)
}
