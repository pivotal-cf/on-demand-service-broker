package service

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"log"

	"github.com/pivotal-cf/on-demand-service-broker/authorizationheader"
)

//go:generate counterfeiter -o fakes/fake_doer.go . Doer
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type ServiceInstanceLister struct {
	authHeaderBuilder authorizationheader.AuthHeaderBuilder
	baseURL           string
	client            Doer
	configured        bool
	logger            *log.Logger
}

var (
	InstanceNotFound = errors.New("Service instance not found")
)

func NewInstanceLister(client Doer, authHeaderBuilder authorizationheader.AuthHeaderBuilder, baseURL string, configured bool, logger *log.Logger) *ServiceInstanceLister {
	return &ServiceInstanceLister{
		authHeaderBuilder: authHeaderBuilder,
		baseURL:           baseURL,
		client:            client,
		configured:        configured,
		logger:            logger,
	}
}

func (s *ServiceInstanceLister) FilteredInstances(params map[string]string) ([]Instance, error) {
	request, err := http.NewRequest(http.MethodGet, s.baseURL, nil)
	if err != nil {
		return nil, err
	}

	values := request.URL.Query()
	for k, v := range params {
		values.Add(k, v)
	}
	request.URL.RawQuery = values.Encode()

	err = s.authHeaderBuilder.AddAuthHeader(request, s.logger)
	if err != nil {
		return nil, err
	}

	response, err := s.client.Do(request)
	if err != nil {
		return s.instanceListerError(response, err)
	}

	if response.StatusCode != http.StatusOK {
		return s.instanceListerError(response, fmt.Errorf("HTTP response status: %s", response.Status))
	}

	defer response.Body.Close()

	var instances []Instance
	err = json.NewDecoder(response.Body).Decode(&instances)
	if err != nil {
		return instances, err
	}
	return instances, nil
}

func (s *ServiceInstanceLister) Instances() ([]Instance, error) {
	return s.FilteredInstances(map[string]string{})
}

func (s *ServiceInstanceLister) LatestInstanceInfo(instance Instance) (Instance, error) {
	instances, err := s.Instances()
	if err != nil {
		return Instance{}, err
	}
	for _, inst := range instances {
		if inst.GUID == instance.GUID {
			return inst, nil
		}
	}
	return Instance{}, InstanceNotFound
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
