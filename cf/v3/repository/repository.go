package repository

import (
	"encoding/json"
	"net/url"

	"github.com/cloudfoundry/cli/cf/v3/models"
	"github.com/cloudfoundry/go-ccapi/v3/client"
)

//go:generate counterfeiter -o fakes/fake_repository.go . Repository
type Repository interface {
	GetApplications() ([]models.V3Application, error)
	GetProcesses(path string) ([]models.V3Process, error)
	GetRoutes(path string) ([]models.V3Route, error)
}

type repository struct {
	ccClient     client.Client
	tokenHandler TokenHandler
}

func NewRepository(
	ccClient client.Client,
	tokenHandler TokenHandler,
) Repository {
	return &repository{
		ccClient:     ccClient,
		tokenHandler: tokenHandler,
	}
}

func (r *repository) GetApplications() ([]models.V3Application, error) {
	jsonResponse, err := r.tokenHandler.Do(func() ([]byte, error) {
		return r.ccClient.GetApplications(url.Values{})
	})
	if err != nil {
		return []models.V3Application{}, err
	}

	applications := []models.V3Application{}
	err = json.Unmarshal(jsonResponse, &applications)
	if err != nil {
		return []models.V3Application{}, err
	}

	return applications, nil
}

func (r *repository) GetProcesses(path string) ([]models.V3Process, error) {
	jsonResponse, err := r.tokenHandler.Do(func() ([]byte, error) {
		return r.ccClient.GetResources(path, 0)
	})
	if err != nil {
		return []models.V3Process{}, err
	}

	processes := []models.V3Process{}
	err = json.Unmarshal(jsonResponse, &processes)
	if err != nil {
		return []models.V3Process{}, err
	}

	return processes, nil
}

func (r *repository) GetRoutes(path string) ([]models.V3Route, error) {
	jsonResponse, err := r.tokenHandler.Do(func() ([]byte, error) {
		return r.ccClient.GetResources(path, 0)
	})
	if err != nil {
		return []models.V3Route{}, err
	}

	routes := []models.V3Route{}
	err = json.Unmarshal(jsonResponse, &routes)
	if err != nil {
		return []models.V3Route{}, err
	}

	return routes, nil
}
