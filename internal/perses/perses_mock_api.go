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
