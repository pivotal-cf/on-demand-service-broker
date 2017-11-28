package service

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

//go:generate counterfeiter -o fakes/fake_http_client.go . HTTPClient
type HTTPClient interface {
	Get(path string, query map[string]string) (*http.Response, error)
}

type ServiceInstanceLister struct {
	client     HTTPClient
	configured bool
}

func NewInstanceLister(client HTTPClient, configured bool) *ServiceInstanceLister {
	return &ServiceInstanceLister{
		client:     client,
		configured: configured,
	}
}

func (s *ServiceInstanceLister) Instances() ([]Instance, error) {
	var instances []Instance

	response, err := s.client.Get("", nil)
	if err != nil {
		return s.instanceListerError(response, err)
	}

	if response.StatusCode != http.StatusOK {
		return s.instanceListerError(response, fmt.Errorf("HTTP response status: %s", response.Status))
	}

	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&instances)
	if err != nil {
		return instances, err
	}

	return instances, nil
}

func (s *ServiceInstanceLister) instanceListerError(response *http.Response, err error) ([]Instance, error) {
	if s.configured {
		if urlError, ok := err.(*url.Error); ok {
			if urlError.Err != nil && urlError.URL != "" {
				switch urlError.Err.(type) {
				case x509.UnknownAuthorityError:
					return []Instance{}, fmt.Errorf(
						"SSL validation error for `service_instances_api.url`: %s. Please configure a `service_instances_api.root_ca_cert` and use a valid SSL certificate",
						urlError.URL,
					)
				default:
					return []Instance{}, fmt.Errorf("error communicating with service_instances_api (%s): %s", urlError.URL, err.Error())
				}
			}
		}

		if response != nil {
			return []Instance{}, fmt.Errorf("error communicating with service_instances_api (%s): %s", response.Request.URL, err.Error())
		}
	}
	return []Instance{}, err
}
