package service

import (
	"encoding/json"
	"fmt"
	"net/http"
)

//go:generate counterfeiter -o fakes/fake_http_client.go . HTTPClient
type HTTPClient interface {
	Get(path string, query map[string]string) (*http.Response, error)
	Patch(path string) (*http.Response, error)
}

type ServiceInstanceLister struct {
	client HTTPClient
}

func NewInstanceLister(client HTTPClient) *ServiceInstanceLister {
	return &ServiceInstanceLister{
		client: client,
	}
}

func (s *ServiceInstanceLister) Instances() ([]Instance, error) {
	var instances []Instance

	response, err := s.client.Get("", nil)
	if err != nil {
		return instances, err
	}

	if response.StatusCode != http.StatusOK {
		return instances, fmt.Errorf("HTTP response status: %s", response.Status)
	}

	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&instances)
	if err != nil {
		return instances, err
	}

	return instances, nil
}
