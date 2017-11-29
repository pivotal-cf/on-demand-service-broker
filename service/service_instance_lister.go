package service

import (
	"crypto/x509"
	"encoding/json"
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

func NewInstanceLister(client Doer, authHeaderBuilder authorizationheader.AuthHeaderBuilder, baseURL string, configured bool, logger *log.Logger) *ServiceInstanceLister {
	return &ServiceInstanceLister{
		authHeaderBuilder: authHeaderBuilder,
		baseURL:           baseURL,
		client:            client,
		configured:        configured,
		logger:            logger,
	}
}

func (s *ServiceInstanceLister) Instances() ([]Instance, error) {
	var instances []Instance
	request, err := http.NewRequest(
		http.MethodGet,
		s.baseURL,
		nil,
	)

	if err != nil {
		return nil, err
	}
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
